package languages

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
)

func TestLanguagesCommand_PrintsSupportedLanguagesAndExtensions(t *testing.T) {
	cmd := NewCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), out.Bytes())
}

func TestLanguagesCommand_JSONFormat_GroupsExtensionsByLanguage(t *testing.T) {
	cmd := NewCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var payload struct {
		Languages []struct {
			Language   string   `json:"language"`
			Maturity   string   `json:"maturity"`
			Extensions []string `json:"extensions"`
		} `json:"languages"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(payload.Languages) == 0 {
		t.Fatal("expected at least one language in JSON output")
	}

	byName := make(map[string]struct {
		maturity   string
		extensions []string
	})
	for _, l := range payload.Languages {
		byName[l.Language] = struct {
			maturity   string
			extensions []string
		}{l.Maturity, l.Extensions}
		// Maturity must be the machine slug, not the human display name.
		if strings.Contains(l.Maturity, " ") || l.Maturity != strings.ToLower(l.Maturity) {
			t.Fatalf("maturity %q for %q is not a machine slug", l.Maturity, l.Language)
		}
	}

	go_, ok := byName["Go"]
	if !ok {
		t.Fatalf("expected Go in output, got %v", byName)
	}
	if go_.maturity != "actively_tested" {
		t.Fatalf("Go maturity = %q, want actively_tested", go_.maturity)
	}
	if len(go_.extensions) != 1 || go_.extensions[0] != ".go" {
		t.Fatalf("Go extensions = %v, want [.go]", go_.extensions)
	}
}

func TestLanguagesCommand_UnknownFormat_ReturnsError(t *testing.T) {
	cmd := NewCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--format", "yaml"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for an unknown --format, got nil")
	}
}
