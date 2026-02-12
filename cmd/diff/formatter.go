package diff

import (
	"fmt"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
)

// Formatter renders a dependency graph delta into a concrete output format.
type Formatter interface {
	Format(delta graphDelta) (string, error)
}

type dotDiffFormatter struct{}

type mermaidDiffFormatter struct{}

// NewDiffFormatter constructs a formatter for the requested output format.
func NewDiffFormatter(format string) (Formatter, error) {
	parsed, ok := formatters.ParseOutputFormat(format)
	if !ok {
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, formatters.SupportedFormats())
	}

	switch parsed {
	case formatters.OutputFormatDOT:
		return dotDiffFormatter{}, nil
	case formatters.OutputFormatMermaid:
		return mermaidDiffFormatter{}, nil
	case formatters.OutputFormatJSON:
		return nil, fmt.Errorf("json format is not supported for diff (valid options: dot, mermaid)")
	default:
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, formatters.SupportedFormats())
	}
}
