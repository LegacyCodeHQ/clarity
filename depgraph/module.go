package depgraph

import (
	"sort"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// Module is a user-named set of files projected onto the dependency graph.
// Files holds graph node keys (the same absolute paths used as graph vertices).
type Module struct {
	Name  string
	Files []string
}

// Collapse describes the result of collapsing modules into a graph.
type Collapse struct {
	// Members maps each created module node to the sorted member files it
	// collapsed, for annotating the node with its file count and churn.
	Members map[string][]string
	// EdgeOrigins maps each collapsed edge that touches a module node to the
	// original file→file edges it represents, so renderers can label it by the
	// underlying dependencies instead of the collapsed endpoints.
	EdgeOrigins map[FileEdge][]FileEdge
}

// CollapseModules contracts each module's member files into a single synthetic
// node named after the module. It is a config-driven post-processing transform
// applied to the built graph: edges touching a member are redirected to the
// module node, edges internal to a module (including self-loops) are dropped,
// and duplicate edges are deduplicated. Members absent from the graph are
// ignored; a module with no members in the graph contributes nothing.
//
// Running the transform on the dependency graph (before metadata is computed)
// lets cycles, edges, and stats recompute naturally against the collapsed
// shape, so a module reads as a single opaque node in the rendered graph.
//
// It returns the collapsed graph and a Collapse describing the created module
// nodes (their members) and the provenance of every collapsed edge, so the
// caller can annotate module nodes and preserve edge labels.
func CollapseModules(g DependencyGraph, modules []Module) (DependencyGraph, Collapse, error) {
	adjacency, err := AdjacencyList(g)
	if err != nil {
		return nil, Collapse{}, err
	}

	memberToModule := make(map[string]string)
	moduleMembers := make(map[string][]string)
	for _, module := range modules {
		for _, file := range module.Files {
			if _, ok := adjacency[file]; ok {
				memberToModule[file] = module.Name
				moduleMembers[module.Name] = append(moduleMembers[module.Name], file)
			}
		}
	}
	if len(memberToModule) == 0 {
		return g, Collapse{}, nil
	}

	resolve := func(node string) string {
		if name, ok := memberToModule[node]; ok {
			return name
		}
		return node
	}

	edges := make(map[string]map[string]bool)
	ensure := func(node string) {
		if edges[node] == nil {
			edges[node] = make(map[string]bool)
		}
	}
	edgeOrigins := make(map[FileEdge][]FileEdge)

	for source, deps := range adjacency {
		from := resolve(source)
		ensure(from)
		for _, dep := range deps {
			to := resolve(dep)
			if from == to {
				continue // drop edges internal to a module (and self-loops)
			}
			ensure(to)
			edges[from][to] = true
			// Record provenance for edges that were rerouted to a module node,
			// so renderers can label them by their original dependencies.
			if from != source || to != dep {
				collapsedEdge := FileEdge{From: from, To: to}
				edgeOrigins[collapsedEdge] = append(edgeOrigins[collapsedEdge], FileEdge{From: source, To: dep})
			}
		}
	}

	collapsed := make(map[string][]string, len(edges))
	for node, targets := range edges {
		deps := make([]string, 0, len(targets))
		for target := range targets {
			deps = append(deps, target)
		}
		sort.Strings(deps)
		collapsed[node] = deps
	}

	for name := range moduleMembers {
		sort.Strings(moduleMembers[name])
	}
	for edge := range edgeOrigins {
		sortFileEdges(edgeOrigins[edge])
	}

	graph, err := NewDependencyGraphFromAdjacency(collapsed)
	if err != nil {
		return nil, Collapse{}, err
	}
	return graph, Collapse{Members: moduleMembers, EdgeOrigins: edgeOrigins}, nil
}

// AnnotateModule marks a collapsed module node and records its file count and
// aggregated churn, summed from the per-file stats of its members. Members
// without a stats entry (unchanged files) still count toward the file total.
//
// It is a free function rather than a method so the core FileDependencyGraph
// type does not gain module-specific API from this file (which would couple
// the core graph back to the module feature).
func AnnotateModule(fg *FileDependencyGraph, moduleNode string, members []string, fileStats map[string]vcs.FileStats) {
	md, ok := fg.Meta.Files[moduleNode]
	if !ok {
		return
	}

	md.IsModule = true
	md.ModuleFileCount = len(members)

	var additions, deletions int
	for _, member := range members {
		if stats, ok := fileStats[member]; ok {
			additions += stats.Additions
			deletions += stats.Deletions
		}
	}
	md.Stats = &vcs.FileStats{Additions: additions, Deletions: deletions}

	fg.Meta.Files[moduleNode] = md
}
