package formatters

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/sanity/git"
	"github.com/LegacyCodeHQ/sanity/parsers"
)

// Format name constants
const (
	FormatDOT     = "dot"
	FormatJSON    = "json"
	FormatMermaid = "mermaid"
)

// FormatOptions contains optional parameters for formatting dependency graphs.
type FormatOptions struct {
	// Label is an optional title or label for the graph
	Label string
	// FileStats contains file statistics (additions/deletions) for display in nodes
	FileStats map[string]git.FileStats
}

// Formatter is the interface that all graph formatters must implement.
type Formatter interface {
	// Format converts a dependency graph to a formatted string representation.
	Format(g parsers.DependencyGraph, opts FormatOptions) (string, error)
}

// registry holds all registered formatters
var registry = make(map[string]func() Formatter)

// Register adds a formatter constructor to the registry.
// Each formatter should call this in its init() function.
func Register(name string, constructor func() Formatter) {
	registry[name] = constructor
}

// NewFormatter creates a Formatter for the specified format type.
func NewFormatter(format string) (Formatter, error) {
	constructor, ok := registry[format]
	if !ok {
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, availableFormats())
	}
	return constructor(), nil
}

// availableFormats returns a comma-separated list of registered format names.
func availableFormats() string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
