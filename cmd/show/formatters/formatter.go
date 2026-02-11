package formatters

import (
	"fmt"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

type dotFormatter struct{}

type mermaidFormatter struct{}

// Formatter is the interface that all graph formatters must implement.
type Formatter interface {
	// Format converts a dependency graph to a formatted string representation.
	Format(g depgraph.FileDependencyGraph, opts RenderOptions) (string, error)
	// GenerateURL creates a shareable URL for the formatted output.
	// Returns the URL and true if supported, or ("", false) if not.
	GenerateURL(output string) (string, bool)
}

// NewFormatter creates a Formatter for the provided output format string.
func NewFormatter(format string) (Formatter, error) {
	f, ok := ParseOutputFormat(format)
	if !ok {
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, SupportedFormats())
	}

	switch f {
	case OutputFormatDOT:
		return dotFormatter{}, nil
	case OutputFormatMermaid:
		return mermaidFormatter{}, nil
	case endOfSupportedFormatsMarker:
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, SupportedFormats())
	default:
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, SupportedFormats())
	}
}

// RenderOptions contains output-specific rendering options.
type RenderOptions struct {
	// Label is an optional title or label for the graph output.
	Label string
}
