package formatters

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

// Scene is the renderer-agnostic plan for one graph render. It is built once by
// BuildScene from the dependency graph and render options, and holds the
// resolved primitives that every formatter renders.
//
// Keeping the derivation here — rather than in each formatter — is what stops
// the DOT and Mermaid outputs from drifting apart: a formatter translates Scene
// primitives into its own syntax and contains no graph logic of its own. New
// primitives are added to the Scene and to the Renderer contract together, so a
// capability cannot exist in one formatter but not the other.
type Scene struct {
	Header GraphHeader
	// FilePaths are the graph's node keys, sorted for deterministic output.
	FilePaths []string
	// NodeNames maps each node key to its display name.
	NodeNames map[string]string
	// CycleNodes is the set of nodes that participate in a dependency cycle.
	CycleNodes map[string]bool
	// Nodes holds the resolved presentation for each graph node, keyed by node
	// key. Built once here so both formatters render identical content.
	Nodes map[string]SceneNode
	// Edges are the dependency edges in deterministic render order.
	Edges []SceneEdge
	// Cluster, when set, is the labeled module boundary to draw around its
	// member nodes; nil when no boundary is rendered.
	Cluster *SceneCluster
	// MajorityType is the most common file type; nodes of this type render
	// neutral rather than type-colored.
	MajorityType string
	// HasMultipleTypes is true when more than one file type is present.
	HasMultipleTypes bool
}

// SceneCluster is a labeled boundary drawn around a set of member node keys. It
// is populated only for the single-module boundary view.
type SceneCluster struct {
	Name    string
	Members []string
}

// SceneNode is the domain view of a graph node for the formatters: a file with
// its type, lifecycle, role, and cycle membership. Formatters render from these
// fields and do not reach back into the dependency graph.
//
// LabelLines is the node's label as ordered rows with no separator, so each
// formatter joins them with its own line break (DOT "\n", Mermaid "<br/>") and
// applies its own escaping — keeping the two label outputs in step.
type SceneNode struct {
	Key        string
	Name       string
	LabelLines []string
	// Type is the file type: its extension, or its base name when extensionless.
	Type string
	// Kind is what the node fundamentally is: a regular file, a test file, or a
	// collapsed module. These are mutually exclusive.
	Kind NodeKind
	// State is the node's lifecycle: present, deleted, or renamed.
	State depgraph.FileState
	// IsPruned marks a node whose subtree was elided — orthogonal to Kind.
	IsPruned bool
	// InCycle marks a node in a dependency cycle — orthogonal to Kind.
	InCycle bool
	// Phantom describes the node's test-sibling, or nil when it has none.
	Phantom *depgraph.PhantomMetadata
}

// NodeKind is what a graph node fundamentally is. The kinds are mutually
// exclusive; cycle membership, prune state, and lifecycle apply independently
// and are modeled separately.
type NodeKind int

const (
	// NodeKindFile is a regular (non-test) source file.
	NodeKindFile NodeKind = iota
	// NodeKindTest is a test file.
	NodeKindTest
	// NodeKindModule is a collapsed module node.
	NodeKindModule
)

// SceneEdge is the domain view of a dependency edge for the formatters: a
// directed dependency with its lifecycle and cycle membership.
type SceneEdge struct {
	From string
	To   string
	// State is the edge's lifecycle: present, deleted, or renamed.
	State depgraph.EdgeState
	// InCycle marks an edge that participates in a dependency cycle.
	InCycle bool
	// Labels are the underlying dependency labels, used when edge labels are on
	// (a collapsed module edge keeps one labeled arrow per original dependency).
	Labels []string
}

// GraphHeader carries the graph-level render attributes.
type GraphHeader struct {
	// Orientation is the resolved layout direction (already defaulted).
	Orientation GraphDirection
	// Title is an optional graph label/title; empty when unset.
	Title string
	// TrailingNewline records whether the layout direction was set explicitly,
	// which both formatters use to decide a trailing newline.
	TrailingNewline bool
}

// BuildScene resolves a dependency graph and render options into a
// renderer-agnostic Scene. Formatter-specific syntax is applied later by a
// Renderer; this function owns all of the shared derivation.
func BuildScene(g depgraph.FileDependencyGraph, opts RenderOptions) (Scene, error) {
	orientation := opts.Direction
	if orientation == "" {
		orientation = DefaultDirection
	}

	filePaths := make([]string, 0, len(g.Meta.Files))
	for node := range g.Meta.Files {
		filePaths = append(filePaths, node)
	}
	sort.Strings(filePaths)

	nodeNames := BuildNodeNames(filePaths)

	fileType := make(map[string]string, len(filePaths))
	typeCounts := make(map[string]int)
	for _, source := range filePaths {
		key := fileTypeKey(source)
		fileType[source] = key
		typeCounts[key]++
	}

	cycleNodes := buildCycleNodes(g)

	nodes := make(map[string]SceneNode, len(filePaths))
	for _, source := range filePaths {
		md := g.Meta.Files[source]
		nodes[source] = SceneNode{
			Key:        source,
			Name:       nodeNames[source],
			LabelLines: nodeLabelLines(g, nodeNames[source], source, opts.BasePath),
			Type:       fileType[source],
			Kind:       nodeKind(md),
			State:      md.State,
			IsPruned:   md.IsPruned,
			InCycle:    cycleNodes[source],
			Phantom:    md.Phantom,
		}
	}

	var cluster *SceneCluster
	if g.Meta.ModuleCluster != nil {
		cluster = &SceneCluster{
			Name:    g.Meta.ModuleCluster.Name,
			Members: g.Meta.ModuleCluster.Members,
		}
	}

	adjacency, err := depgraph.AdjacencyList(g.Graph)
	if err != nil {
		return Scene{}, err
	}
	var edges []SceneEdge
	for _, source := range filePaths {
		deps := append([]string(nil), adjacency[source]...)
		sort.Strings(deps)
		for _, dep := range deps {
			md := g.Meta.Edges[depgraph.FileEdge{From: source, To: dep}]
			edges = append(edges, SceneEdge{
				From:    source,
				To:      dep,
				State:   md.State,
				InCycle: md.InCycle,
				Labels:  edgeLabels(g, source, dep, opts.BasePath),
			})
		}
	}

	return Scene{
		Header: GraphHeader{
			Orientation:     orientation,
			Title:           opts.Label,
			TrailingNewline: opts.Direction != "",
		},
		FilePaths:        filePaths,
		NodeNames:        nodeNames,
		CycleNodes:       cycleNodes,
		Nodes:            nodes,
		Edges:            edges,
		Cluster:          cluster,
		MajorityType:     majorityType(typeCounts),
		HasMultipleTypes: len(typeCounts) > 1,
	}, nil
}

// nodeKind classifies a node as a module, a test file, or a regular file. A
// module takes precedence: its synthetic node is never itself a test file.
func nodeKind(md depgraph.FileMetadata) NodeKind {
	switch {
	case md.IsModule:
		return NodeKindModule
	case md.IsTest:
		return NodeKindTest
	default:
		return NodeKindFile
	}
}

// majorityType returns the most common type key, breaking ties by sort order so
// the choice is deterministic.
func majorityType(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	majority := ""
	maxCount := 0
	for _, k := range keys {
		if counts[k] > maxCount {
			maxCount = counts[k]
			majority = k
		}
	}
	return majority
}

// nodeLabelLines resolves a node's label into ordered visual rows. It mirrors
// the label both formatters built independently, including module file counts,
// churn stats with the new-file sprout, and the deleted/renamed markers.
func nodeLabelLines(g depgraph.FileDependencyGraph, name, source, basePath string) []string {
	md, ok := g.Meta.Files[source]
	if !ok {
		return []string{name}
	}

	if md.IsModule {
		return strings.Split(moduleNodeLabel(name, md, "\n"), "\n")
	}

	lines := []string{name}
	if md.Stats != nil {
		stats := *md.Stats
		prefix := name
		if stats.IsNew {
			prefix = fmt.Sprintf("🪴 %s", name)
		}

		var parts []string
		if stats.Additions > 0 {
			parts = append(parts, fmt.Sprintf("+%d", stats.Additions))
		}
		if stats.Deletions > 0 {
			parts = append(parts, fmt.Sprintf("-%d", stats.Deletions))
		}

		switch {
		case len(parts) > 0:
			lines = []string{prefix, strings.Join(parts, " ")}
		case stats.IsNew:
			lines = []string{prefix}
		}
	}

	switch md.State {
	case depgraph.FileStateDeleted:
		lines = append(lines, "(deleted)")
	case depgraph.FileStateRenamed:
		lines = append(lines, "(renamed)")
	case depgraph.FileStatePresent:
		// no marker
	}

	// A collapsed rename renders as the single new-path node, annotated with the
	// path it came from (the old node is removed from the graph).
	if md.RenamedFrom != "" {
		lines = append(lines, renameAnnotation(md.RenamedFrom, source, basePath))
	}

	return lines
}

// renameAnnotation describes a path change the way a developer thinks of it,
// from the old and new paths git reported: changing only the basename is a
// rename (✏️), changing only the directory is a move (🚚), and changing both is
// a move + rename. Paths are shown relative to basePath, matching the node names.
func renameAnnotation(oldPath, newPath, basePath string) string {
	oldRel := nodeKey(oldPath, basePath)
	newRel := nodeKey(newPath, basePath)
	switch {
	case filepath.Dir(oldRel) == filepath.Dir(newRel):
		return fmt.Sprintf("✏️ renamed from %s", filepath.Base(oldRel))
	case filepath.Base(oldRel) == filepath.Base(newRel):
		return fmt.Sprintf("🚚 moved from %s/", filepath.Dir(oldRel))
	default:
		return fmt.Sprintf("🚚✏️ moved & renamed from %s", oldRel)
	}
}

// buildCycleNodes returns the set of nodes that participate in a dependency
// cycle, derived from the representative cycle paths and any edge flagged in a
// cycle. Both formatters render cycle membership from this single set.
func buildCycleNodes(g depgraph.FileDependencyGraph) map[string]bool {
	cycleNodes := make(map[string]bool)
	for _, cycle := range g.Meta.Cycles {
		for _, node := range cycle.Path {
			cycleNodes[node] = true
		}
	}
	for edge, md := range g.Meta.Edges {
		if md.InCycle {
			cycleNodes[edge.From] = true
			cycleNodes[edge.To] = true
		}
	}
	return cycleNodes
}
