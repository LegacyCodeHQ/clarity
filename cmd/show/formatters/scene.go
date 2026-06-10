package formatters

import (
	"fmt"
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
}

// SceneNode is a graph node resolved for rendering. LabelLines is the node's
// label content as ordered visual rows with no separator, so each formatter
// joins them with its own line break (DOT "\n", Mermaid "<br/>") and applies its
// own escaping. Deriving the lines here is what keeps the two label outputs in
// step — including the trailing "(deleted)"/"(renamed)" markers.
type SceneNode struct {
	Key        string
	LabelLines []string
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
func BuildScene(g depgraph.FileDependencyGraph, opts RenderOptions) Scene {
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
	nodes := make(map[string]SceneNode, len(filePaths))
	for _, source := range filePaths {
		nodes[source] = SceneNode{
			Key:        source,
			LabelLines: nodeLabelLines(g, nodeNames[source], source),
		}
	}

	return Scene{
		Header: GraphHeader{
			Orientation:     orientation,
			Title:           opts.Label,
			TrailingNewline: opts.Direction != "",
		},
		FilePaths:  filePaths,
		NodeNames:  nodeNames,
		CycleNodes: buildCycleNodes(g),
		Nodes:      nodes,
	}
}

// nodeLabelLines resolves a node's label into ordered visual rows. It mirrors
// the label both formatters built independently, including module file counts,
// churn stats with the new-file sprout, and the deleted/renamed markers.
func nodeLabelLines(g depgraph.FileDependencyGraph, name, source string) []string {
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

	return lines
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
