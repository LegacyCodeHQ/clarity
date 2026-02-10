package graph

import (
	"fmt"

	"github.com/LegacyCodeHQ/clarity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/clarity/cmd/graph/formatters/dot"
	"github.com/LegacyCodeHQ/clarity/cmd/graph/formatters/mermaid"
)

// NewFormatter creates a Formatter for the specified format type.
func NewFormatter(format string) (formatters.Formatter, error) {
	f, ok := formatters.ParseOutputFormat(format)
	if !ok {
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, formatters.SupportedFormats())
	}

	switch f {
	case formatters.OutputFormatDOT:
		return &dot.Formatter{}, nil
	case formatters.OutputFormatMermaid:
		return &mermaid.Formatter{}, nil
	default:
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, formatters.SupportedFormats())
	}
}
