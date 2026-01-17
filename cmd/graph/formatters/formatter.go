package formatters

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/sanity/parsers"
	"github.com/LegacyCodeHQ/sanity/vcs"
)

// OutputFormat represents an output format type
type OutputFormat string

const (
	OutputFormatDOT     OutputFormat = "dot"
	OutputFormatJSON    OutputFormat = "json"
	OutputFormatMermaid OutputFormat = "mermaid"
)

// String returns the string representation of the format
func (f OutputFormat) String() string {
	return string(f)
}

// FormatOptions contains optional parameters for formatting dependency graphs.
type FormatOptions struct {
	// Label is an optional title or label for the graph
	Label string
	// FileStats contains file statistics (additions/deletions) for display in nodes
	FileStats map[string]vcs.FileStats
}

// Formatter is the interface that all graph formatters must implement.
type Formatter interface {
	// Format converts a dependency graph to a formatted string representation.
	Format(g parsers.DependencyGraph, opts FormatOptions) (string, error)
	// GenerateURL creates a shareable URL for the formatted output.
	// Returns the URL and true if supported, or ("", false) if not.
	GenerateURL(output string) (string, bool)
}

// registry holds all registered formatters
var registry = make(map[OutputFormat]func() Formatter)

// Register adds a formatter constructor to the registry.
// Each formatter should call this in its init() function.
func Register(name OutputFormat, constructor func() Formatter) {
	registry[name] = constructor
}

// NewFormatter creates a Formatter for the specified format type.
func NewFormatter(format string) (Formatter, error) {
	constructor, ok := registry[OutputFormat(format)]
	if !ok {
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, availableFormats())
	}
	return constructor(), nil
}

// availableFormats returns a comma-separated list of registered format names.
func availableFormats() string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name.String())
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
