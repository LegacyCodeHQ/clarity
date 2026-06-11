package depgraph

import (
	"path/filepath"
	"sort"

	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

// FileDependencyGraph wraps a dependency graph with file-level metadata.
type FileDependencyGraph struct {
	Graph DependencyGraph
	Meta  FileGraphMetadata
}

// FileGraphMetadata contains metadata keyed by file and edge.
type FileGraphMetadata struct {
	Files  map[string]FileMetadata
	Edges  map[FileEdge]EdgeMetadata
	Cycles []FileCycle
	// EdgeOrigins maps a collapsed edge (an edge in the rendered graph that
	// touches a module node) to the original file→file edges it represents.
	// It lets renderers label a collapsed edge by its underlying dependencies
	// so labels stay stable whether or not modules are applied.
	EdgeOrigins map[FileEdge][]FileEdge
	// ModuleCluster, when set, names a module whose Members render inside a drawn
	// boundary box. It is populated only for the single-module boundary view
	// (show --module <name>) and only when the module has external dependents or
	// dependencies worth framing; an isolated module leaves this nil so its files
	// render exactly as they would without a box.
	ModuleCluster *ModuleCluster
}

// ModuleCluster groups a module's member file nodes so a renderer can draw a
// labeled boundary around them.
type ModuleCluster struct {
	Name    string
	Members []string
}

// FileState describes whether a rendered file node exists in the current tree
// or is historical context for a deleted file.
type FileState string

const (
	FileStatePresent FileState = "present"
	FileStateDeleted FileState = "deleted"
	FileStateRenamed FileState = "renamed"
)

// FileMetadata holds metadata for a single file node.
type FileMetadata struct {
	Stats *vcs.FileStats
	State FileState
	// RenamedFrom is the old path when this node is a rename target (the rename
	// is collapsed onto this single node; the old path is removed from the graph).
	RenamedFrom     string
	IsTest          bool
	IsPruned        bool
	IsModule        bool
	ModuleFileCount int
	Extension       string
	Phantom         *PhantomMetadata
}

// PhantomMetadata describes a sibling "phantom" node attached to a file to
// represent in-file test code as a distinct visual node. Kind identifies the
// language-specific source of the phantom (e.g. "rust-test"). Stats carries
// any per-line attribution for watch mode; nil in show mode. ProdChanged is
// true when the prod side of the file also has changes (watch mode only).
type PhantomMetadata struct {
	Kind        string
	Stats       *vcs.FileStats
	ProdChanged bool
}

// FileEdge identifies a directed edge between two files.
type FileEdge struct {
	From string
	To   string
}

// EdgeState describes whether a rendered dependency exists in the current tree
// or is shown as historical context for a deleted node.
type EdgeState string

const (
	EdgeStatePresent EdgeState = "present"
	EdgeStateDeleted EdgeState = "deleted"
	EdgeStateRenamed EdgeState = "renamed"
)

// EdgeMetadata holds metadata for a graph edge.
type EdgeMetadata struct {
	InCycle bool
	State   EdgeState
}

// FileCycle describes a representative cycle path for a cyclic SCC.
type FileCycle struct {
	Path []string
}

// NewFileDependencyGraph creates a file-annotated graph from a dependency graph, optional file stats,
// and an optional content reader used for richer test detection.
func NewFileDependencyGraph(g DependencyGraph, fileStats map[string]vcs.FileStats, contentReader vcs.ContentReader) (FileDependencyGraph, error) {
	adjacency, err := AdjacencyList(g)
	if err != nil {
		return FileDependencyGraph{}, err
	}

	files := make(map[string]FileMetadata, len(adjacency))
	edges := make(map[FileEdge]EdgeMetadata)

	nodes := make([]string, 0, len(adjacency))
	for node := range adjacency {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	for _, node := range nodes {
		md := FileMetadata{
			State:     FileStatePresent,
			IsTest:    registry.IsTestFile(node, contentReader),
			Extension: filepath.Ext(filepath.Base(node)),
		}

		if fileStats != nil {
			if stats, ok := fileStats[node]; ok {
				statsCopy := stats
				md.Stats = &statsCopy
			}
		}

		files[node] = md

		for _, dep := range adjacency[node] {
			edges[FileEdge{From: node, To: dep}] = EdgeMetadata{State: EdgeStatePresent}
		}
	}

	cycles, cycleEdges := findCyclesAndCycleEdges(adjacency)
	for edge := range cycleEdges {
		edgeMetadata := edges[edge]
		edgeMetadata.InCycle = true
		edges[edge] = edgeMetadata
	}

	return FileDependencyGraph{
		Graph: g,
		Meta: FileGraphMetadata{
			Files:  files,
			Edges:  edges,
			Cycles: cycles,
		},
	}, nil
}

// MarkDeletedFiles marks file nodes and any incident edges as deleted context.
func MarkDeletedFiles(fg *FileDependencyGraph, deletedFiles []string) {
	deleted := make(map[string]bool, len(deletedFiles))
	for _, file := range deletedFiles {
		deleted[file] = true
		md := fg.Meta.Files[file]
		md.State = FileStateDeleted
		if md.Extension == "" {
			md.Extension = filepath.Ext(filepath.Base(file))
		}
		fg.Meta.Files[file] = md
	}

	for edge, md := range fg.Meta.Edges {
		if deleted[edge.From] || deleted[edge.To] {
			md.State = EdgeStateDeleted
			fg.Meta.Edges[edge] = md
		}
	}
}

// MarkRenamedFiles collapses each rename onto its new-path node: the new node
// records the old path it came from and takes churn computed from the old vs
// new content (a pure move reads as +0 -0). The old path is expected to have
// been removed from the graph by CollapseRenames, so a rename renders as a
// single node rather than an old->new pair. oldContent maps each old path to
// its pre-rename bytes; reader yields the new node's current content.
func MarkRenamedFiles(fg *FileDependencyGraph, renames map[string]string, oldContent map[string][]byte, reader vcs.ContentReader) {
	for oldPath, newPath := range renames {
		md, ok := fg.Meta.Files[newPath]
		if !ok {
			continue
		}
		md.RenamedFrom = oldPath
		if md.Extension == "" {
			md.Extension = filepath.Ext(filepath.Base(newPath))
		}
		var newBytes []byte
		if reader != nil {
			newBytes, _ = reader(newPath)
		}
		stats := renameChurn(oldContent[oldPath], newBytes)
		md.Stats = &stats
		fg.Meta.Files[newPath] = md
	}
}

func findCyclesAndCycleEdges(adjacency map[string][]string) ([]FileCycle, map[FileEdge]bool) {
	sccs := stronglyConnectedComponents(adjacency)
	cycleEdges := make(map[FileEdge]bool)

	var cycles []FileCycle
	for _, scc := range sccs {
		if !isCyclicSCC(adjacency, scc) {
			continue
		}

		allowed := make(map[string]bool, len(scc))
		for _, node := range scc {
			allowed[node] = true
		}
		for _, from := range scc {
			for _, to := range adjacency[from] {
				if allowed[to] {
					cycleEdges[FileEdge{From: from, To: to}] = true
				}
			}
		}

		pathWithClosure := canonicalCyclePath(adjacency, scc)
		if len(pathWithClosure) < 2 {
			continue
		}

		cyclePath := append([]string(nil), pathWithClosure[:len(pathWithClosure)-1]...)
		cycles = append(cycles, FileCycle{Path: cyclePath})

	}

	return cycles, cycleEdges
}

func stronglyConnectedComponents(adjacency map[string][]string) [][]string {
	nodes := make([]string, 0, len(adjacency))
	for node := range adjacency {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	index := 0
	indices := make(map[string]int, len(nodes))
	lowLink := make(map[string]int, len(nodes))
	onStack := make(map[string]bool, len(nodes))
	stack := make([]string, 0, len(nodes))
	var sccs [][]string

	var strongConnect func(v string)
	strongConnect = func(v string) {
		indices[v] = index
		lowLink[v] = index
		index++

		stack = append(stack, v)
		onStack[v] = true

		neighbors := append([]string(nil), adjacency[v]...)
		sort.Strings(neighbors)
		for _, w := range neighbors {
			if _, seen := indices[w]; !seen {
				strongConnect(w)
				if lowLink[w] < lowLink[v] {
					lowLink[v] = lowLink[w]
				}
				continue
			}
			if onStack[w] && indices[w] < lowLink[v] {
				lowLink[v] = indices[w]
			}
		}

		if lowLink[v] == indices[v] {
			var component []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				component = append(component, w)
				if w == v {
					break
				}
			}
			sort.Strings(component)
			sccs = append(sccs, component)
		}
	}

	for _, node := range nodes {
		if _, seen := indices[node]; !seen {
			strongConnect(node)
		}
	}

	sort.Slice(sccs, func(i, j int) bool {
		return sccs[i][0] < sccs[j][0]
	})

	return sccs
}

func isCyclicSCC(adjacency map[string][]string, scc []string) bool {
	if len(scc) > 1 {
		return true
	}
	if len(scc) == 0 {
		return false
	}

	node := scc[0]
	for _, dep := range adjacency[node] {
		if dep == node {
			return true
		}
	}
	return false
}

func canonicalCyclePath(adjacency map[string][]string, scc []string) []string {
	if len(scc) == 0 {
		return nil
	}

	allowed := make(map[string]bool, len(scc))
	for _, node := range scc {
		allowed[node] = true
	}

	start := scc[0]
	if len(scc) == 1 {
		return []string{start, start}
	}

	path := []string{start}
	inPath := map[string]bool{start: true}
	var found []string

	var dfs func(curr string) bool
	dfs = func(curr string) bool {
		neighbors := append([]string(nil), adjacency[curr]...)
		sort.Strings(neighbors)

		for _, next := range neighbors {
			if !allowed[next] {
				continue
			}

			if next == start && len(path) > 1 {
				found = append(append([]string(nil), path...), start)
				return true
			}

			if inPath[next] {
				continue
			}

			inPath[next] = true
			path = append(path, next)
			if dfs(next) {
				return true
			}
			path = path[:len(path)-1]
			delete(inPath, next)
		}

		return false
	}

	if dfs(start) {
		return found
	}

	// Fallback to all SCC edges if no canonical path was found.
	// This should not happen for cyclic SCCs, but keeps metadata populated.
	result := []string{start}
	result = append(result, scc[1:]...)
	result = append(result, start)
	return result
}
