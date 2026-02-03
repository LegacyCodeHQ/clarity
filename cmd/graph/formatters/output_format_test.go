package formatters

import "testing"

func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		format   OutputFormat
		expected string
	}{
		{OutputFormatDOT, "dot"},
		{OutputFormatJSON, "json"},
		{OutputFormatMermaid, "mermaid"},
		{endOfSupportedFormatsMarker, "unknown"},
		{OutputFormat(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.format.String(); got != tt.expected {
				t.Errorf("OutputFormat.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected OutputFormat
		ok       bool
	}{
		{"dot", OutputFormatDOT, true},
		{"json", OutputFormatJSON, true},
		{"mermaid", OutputFormatMermaid, true},
		{"invalid", OutputFormatDOT, false},
		{"", OutputFormatDOT, false},
		{"DOT", OutputFormatDOT, false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseOutputFormat(tt.input)
			if got != tt.expected || ok != tt.ok {
				t.Errorf("ParseOutputFormat(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.expected, tt.ok)
			}
		})
	}
}

func TestSupportedFormats(t *testing.T) {
	got := SupportedFormats()
	expected := "dot, json, mermaid"

	if got != expected {
		t.Errorf("SupportedFormats() = %q, want %q", got, expected)
	}
}

func TestSupportedFormatsCount(t *testing.T) {
	// Verify the count matches the number of formats
	expectedCount := 3
	if int(endOfSupportedFormatsMarker) != expectedCount {
		t.Errorf("endOfSupportedFormatsMarker = %d, want %d", endOfSupportedFormatsMarker, expectedCount)
	}
}
