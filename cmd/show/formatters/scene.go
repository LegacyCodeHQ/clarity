package formatters

import "github.com/LegacyCodeHQ/clarity/depgraph"

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
	_ = g // graph-derived primitives migrate into the Scene incrementally.

	orientation := opts.Direction
	if orientation == "" {
		orientation = DefaultDirection
	}

	return Scene{
		Header: GraphHeader{
			Orientation:     orientation,
			Title:           opts.Label,
			TrailingNewline: opts.Direction != "",
		},
	}
}
