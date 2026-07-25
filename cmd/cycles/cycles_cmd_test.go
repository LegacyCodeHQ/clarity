package cycles

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

func TestCycles_DetectsCircularDependency(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir)

	if !strings.Contains(out, "a.mjs") || !strings.Contains(out, "b.mjs") {
		t.Fatalf("expected cycle output to mention a.mjs and b.mjs, got:\n%s", out)
	}
	if !strings.Contains(out, "→") {
		t.Fatalf("expected a cycle arrow in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Found 1 cyclic component") {
		t.Fatalf("expected a one-component summary, got:\n%s", out)
	}
	if !strings.Contains(out, "Smallest break set (exact, 1 edge)") {
		t.Fatalf("expected an exact minimum break set, got:\n%s", out)
	}
}

func TestCycles_NoCycles_ReportsNone(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `export function b() {}
`)

	out := executeCycles(t, dir)

	if !strings.Contains(strings.ToLower(out), "no cyclic components") {
		t.Fatalf("expected a 'no cyclic components' message, got:\n%s", out)
	}
}

func TestCycles_ReportsCompleteComponentWhenRepresentativeOmitsNode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `
import "./b.mjs";
import "./c.mjs";
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import "./a.mjs";`)
	writeFile(t, filepath.Join(dir, "c.mjs"), `import "./a.mjs";`)

	out := executeCycles(t, dir)

	if !strings.Contains(out, "3 files, 4 internal dependencies") {
		t.Fatalf("expected complete SCC membership and edges, got:\n%s", out)
	}
	if !strings.Contains(out, "Representative loop:") {
		t.Fatalf("expected representative-loop wording, got:\n%s", out)
	}
	if !strings.Contains(out, "Smallest break set (exact, 2 edges)") {
		t.Fatalf("expected verified two-edge minimum, got:\n%s", out)
	}

	urlOut := executeCycles(t, dir, "--url")
	if !strings.Contains(urlOut, "c.mjs") {
		t.Fatalf("expected URL graph to contain the complete component, got:\n%s", urlOut)
	}
}

func TestCycles_JSONIncludesCompleteAnalysis(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import "./b.mjs";`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import "./a.mjs";`)

	out := executeCycles(t, dir, "--format", "json")
	var payload struct {
		Components []struct {
			Nodes []string `json:"nodes"`
		} `json:"components"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("expected valid JSON, got %v:\n%s", err, out)
	}
	if len(payload.Components) != 1 ||
		!strings.HasSuffix(payload.Components[0].Nodes[0], ".mjs") {
		t.Fatalf("expected one component with relative paths, got:\n%s", out)
	}

	for _, field := range []string{
		`"components"`,
		`"representative_loop"`,
		`"break_analysis": "exact"`,
		`"break_sets"`,
		`"evidence"`,
	} {
		if !strings.Contains(out, field) {
			t.Fatalf("expected JSON field %s, got:\n%s", field, out)
		}
	}
}

func TestCycles_CodeOnlyExcludesMarkdownNavigationLoops(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), `[guide](guide.md)`)
	writeFile(t, filepath.Join(dir, "guide.md"), `[readme](README.md)`)

	out := executeCycles(t, dir, "--code-only")

	if !strings.Contains(out, "No cyclic components") {
		t.Fatalf("expected Markdown-only loop to be excluded, got:\n%s", out)
	}
}

func TestCycles_ExcludeKindRecomputesHTMLNavigationCycles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"),
		`<a href="about.html">About</a>`)
	writeFile(t, filepath.Join(dir, "about.html"),
		`<a href="index.html">Home</a>`)

	included := executeCycles(t, dir, "--include-kind", "navigation")
	if !strings.Contains(included, "Found 1 cyclic component") ||
		!strings.Contains(included, "Relationship filters: include=navigation") {
		t.Fatalf("expected navigation-only cycle, got:\n%s", included)
	}

	excluded := executeCycles(t, dir, "--exclude-kind", "navigation")
	if !strings.Contains(excluded, "No cyclic components") ||
		!strings.Contains(excluded, "Relationship filters: exclude=navigation") {
		t.Fatalf("expected navigation cycle to be removed, got:\n%s", excluded)
	}
}

func TestCycles_JSONIncludesRelationshipFilters(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import "./b.mjs";`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import "./a.mjs";`)

	out := executeCycles(t, dir, "--include-kind", "import", "--format", "json")
	if !strings.Contains(out, `"include_kinds": [`) ||
		!strings.Contains(out, `"import"`) {
		t.Fatalf("expected active relationship filters in JSON, got:\n%s", out)
	}
}

func TestCycles_RejectsUnknownRelationshipKind(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"--include-kind", "magic"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown dependency relationship") {
		t.Fatalf("expected an unknown-kind error, got %v", err)
	}
}

func TestCycles_HelpMarksEntireAPISurfaceExperimental(t *testing.T) {
	cmd := NewCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help to render, got %v", err)
	}
	for _, phrase := range []string{
		"EXPERIMENTAL API",
		"command name, flags, relationship taxonomy, human output",
		"JSON schema may change without compatibility notice",
	} {
		if !strings.Contains(output.String(), phrase) {
			t.Fatalf("expected help to contain %q, got:\n%s", phrase, output.String())
		}
	}
}

func TestRenderEvidence_CapsHumanReferencesWithoutAffectingJSON(t *testing.T) {
	edge := depgraph.FileEdge{From: "a.go", To: "b.go"}
	evidence := make([]depgraph.DependencyEvidence, 25)
	for index := range evidence {
		evidence[index] = depgraph.DependencyEvidence{
			Symbol:          "B",
			Kind:            "go-same-package-type-reference",
			Relationship:    depgraph.RelationshipTypeReference,
			ReferenceFile:   "a.go",
			ReferenceLine:   index + 1,
			DeclarationFile: "b.go",
			DeclarationLine: 1,
			Confidence:      depgraph.EvidenceConfidenceHigh,
		}
	}
	item := renderedCycle{
		component: depgraph.FileCycle{Edges: []depgraph.FileEdge{edge}},
		metadata: map[depgraph.FileEdge]depgraph.EdgeMetadata{
			edge: {Evidence: evidence},
		},
	}
	var output bytes.Buffer
	renderEvidence(&output, item, ".")

	if !strings.Contains(output.String(), "… 5 more references") {
		t.Fatalf("expected explicit human evidence cap, got:\n%s", output.String())
	}
}

func TestCycles_ExplainShowsDependencyEvidence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import "./b.mjs";`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import "./a.mjs";`)

	out := executeCycles(t, dir, "--explain")

	if !strings.Contains(out, "Internal dependencies:") ||
		!strings.Contains(out, "javascript-import/import") {
		t.Fatalf("expected source-level evidence, got:\n%s", out)
	}
}

func TestCycles_RelativizesToScope(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	writeFile(t, filepath.Join(sub, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(sub, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir)

	if !strings.Contains(out, "pkg/a.mjs"+cycleArrow) {
		t.Fatalf("expected cycle paths relative to scope (pkg/a.mjs), got:\n%s", out)
	}
	if strings.Contains(out, filepath.Join(dir, "pkg", "a.mjs")) {
		t.Fatalf("expected cycle file paths to be relative, not absolute, got:\n%s", out)
	}
}

func TestCycles_URLFlag_EmitsDiagramLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir, "-u")

	if !strings.Contains(out, "https://") || !strings.Contains(out, "dreampuf.github.io") {
		t.Fatalf("expected a visualization URL with -u, got:\n%s", out)
	}
	if !strings.Contains(out, "a.mjs"+cycleArrow) {
		t.Fatalf("expected -u output to still include the cycle path, got:\n%s", out)
	}
}

func TestCycles_NoURLByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir)

	if strings.Contains(out, "https://") {
		t.Fatalf("expected no URL without -u, got:\n%s", out)
	}
}

func executeCycles(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewCommand()
	cmd.SetArgs(args)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}
	return stdout.String()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
}
