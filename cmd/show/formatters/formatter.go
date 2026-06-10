package formatters

import "github.com/LegacyCodeHQ/clarity/depgraph"

// Formatter is the interface that all graph formatters must implement.
type Formatter interface {
	// Format converts a dependency graph to a formatted string representation.
	Format(g depgraph.FileDependencyGraph, opts RenderOptions) (string, error)
	// GenerateURL creates a shareable URL for the formatted output.
	// Returns the URL and true if supported, or ("", false) if not.
	GenerateURL(output string) (string, bool)
}

// RenderOptions contains output-specific rendering options.
type RenderOptions struct {
	// Label is an optional title or label for the graph output.
	Label string
	// Direction is the layout direction for the graph.
	Direction GraphDirection
	// BasePath is an optional filesystem base used to derive stable relative node IDs.
	BasePath string
	// EdgeLabels enables deterministic short labels on edges.
	EdgeLabels bool
}
