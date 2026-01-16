package formatters

import (
	"encoding/json"

	"github.com/LegacyCodeHQ/sanity/parsers"
)

func init() {
	Register("json", func() Formatter { return &JSONFormatter{} })
}

// JSONFormatter formats dependency graphs as JSON.
type JSONFormatter struct{}

// Format converts the dependency graph to JSON format.
// The opts parameter is accepted for interface compatibility but not used.
func (f *JSONFormatter) Format(g parsers.DependencyGraph, opts FormatOptions) (string, error) {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
