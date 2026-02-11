package formatters

import "strings"

// OutputFormat represents an output format type
type OutputFormat int

const (
	OutputFormatDOT OutputFormat = iota
	OutputFormatMermaid
	endOfSupportedFormatsMarker // endOfSupportedFormatsMarker for iteration
)

// String returns the string representation of the format
func (f OutputFormat) String() string {
	switch f {
	case OutputFormatDOT:
		return "dot"
	case OutputFormatMermaid:
		return "mermaid"
	case endOfSupportedFormatsMarker:
		return "unknown"
	default:
		return "unknown"
	}
}

// ParseOutputFormat converts a string to OutputFormat
func ParseOutputFormat(s string) (OutputFormat, bool) {
	switch strings.ToLower(s) {
	case "dot":
		return OutputFormatDOT, true
	case "mermaid":
		return OutputFormatMermaid, true
	default:
		return OutputFormatDOT, false
	}
}

// SupportedFormats returns a list of all supported output format names.
func SupportedFormats() string {
	formats := make([]string, 0, endOfSupportedFormatsMarker)
	for i := OutputFormat(0); i < endOfSupportedFormatsMarker; i++ {
		formats = append(formats, i.String())
	}
	return strings.Join(formats, ", ")
}
