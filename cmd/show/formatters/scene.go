package formatters

import (
	"sort"

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

	return Scene{
		Header: GraphHeader{
			Orientation:     orientation,
			Title:           opts.Label,
			TrailingNewline: opts.Direction != "",
		},
		FilePaths:  filePaths,
		NodeNames:  BuildNodeNames(filePaths),
		CycleNodes: buildCycleNodes(g),
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
