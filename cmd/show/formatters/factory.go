package formatters

import "fmt"

// NewFormatter creates a Formatter for the provided output format string.
func NewFormatter(format string) (Formatter, error) {
	f, ok := ParseOutputFormat(format)
	if !ok {
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, SupportedFormats())
	}

	switch f {
	case OutputFormatDOT:
		return &dotFormatter{}, nil
	case OutputFormatMermaid:
		return mermaidFormatter{}, nil
	case endOfSupportedFormatsMarker:
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, SupportedFormats())
	default:
		return nil, fmt.Errorf("unknown format: %s (valid options: %s)", format, SupportedFormats())
	}
}
