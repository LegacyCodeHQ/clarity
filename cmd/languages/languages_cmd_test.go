package languages

import (
	"bytes"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/LegacyCodeHQ/sanity/depgraph"
)

func TestLanguagesCommand_PrintsSupportedLanguagesAndExtensions(t *testing.T) {
	cmd := NewCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var expected strings.Builder
	expected.WriteString("\n")
	writer := tabwriter.NewWriter(&expected, 0, 0, 2, ' ', 0)
	for _, language := range depgraph.SupportedLanguages() {
		_, _ = writer.Write([]byte(language.Maturity.Symbol()))
		_, _ = writer.Write([]byte(" "))
		_, _ = writer.Write([]byte(language.Name))
		_, _ = writer.Write([]byte("\t"))
		_, _ = writer.Write([]byte(strings.Join(language.Extensions, ", ")))
		_, _ = writer.Write([]byte("\n"))
	}
	_ = writer.Flush()
	expected.WriteString("\n")
	expected.WriteString("----------------------------------------------------\n")
	expected.WriteString("○ Vibed  ◐ Basic Testing  ● Active Testing  ✓ Stable\n")
	expected.WriteString("\n")

	if out.String() != expected.String() {
		t.Fatalf("output = %q, want %q", out.String(), expected.String())
	}
}
