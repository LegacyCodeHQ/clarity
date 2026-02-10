package why

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
)

func TestWhyCommand_TextDirectDependency(t *testing.T) {
	repoDir := t.TempDir()
	fromPath := filepath.Join(repoDir, "from.js")
	toPath := filepath.Join(repoDir, "to.js")

	if err := os.WriteFile(fromPath, []byte("import { x } from './to.js'\nexport const y = x\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(toPath, []byte("export const x = 1\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "from.js", "to.js"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "from.js depends on to.js") {
		t.Fatalf("expected direct dependency in output, got:\n%s", output)
	}
}

func TestWhyCommand_TextNoDirectDependency(t *testing.T) {
	repoDir := t.TempDir()
	aPath := filepath.Join(repoDir, "a.js")
	bPath := filepath.Join(repoDir, "b.js")

	if err := os.WriteFile(aPath, []byte("export const a = 1\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bPath, []byte("export const b = 2\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "a.js", "b.js"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No immediate dependency") {
		t.Fatalf("expected no-direct-dependency message, got:\n%s", output)
	}
}

func TestWhyCommand_DOTFormat(t *testing.T) {
	repoDir := t.TempDir()
	fromPath := filepath.Join(repoDir, "from.js")
	toPath := filepath.Join(repoDir, "to.js")

	if err := os.WriteFile(fromPath, []byte("import { x } from './to.js'\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(toPath, []byte("export const x = 1\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-f", "dot", "from.js", "to.js"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "digraph G") {
		t.Fatalf("expected DOT output, got:\n%s", output)
	}
	if !strings.Contains(output, `" -> "`) && !strings.Contains(output, `"from.js"`) {
		t.Fatalf("expected edge and node labels in DOT output, got:\n%s", output)
	}
}

func TestWhyCommand_MermaidFormat(t *testing.T) {
	repoDir := t.TempDir()
	fromPath := filepath.Join(repoDir, "from.js")
	toPath := filepath.Join(repoDir, "to.js")

	if err := os.WriteFile(fromPath, []byte("import { x } from './to.js'\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(toPath, []byte("export const x = 1\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-f", "mermaid", "from.js", "to.js"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "flowchart LR") {
		t.Fatalf("expected mermaid output, got:\n%s", output)
	}
	if !strings.Contains(output, "-->") {
		t.Fatalf("expected mermaid edge, got:\n%s", output)
	}
}

func TestFindReferencedMembers_GoFiles_ReturnsUsageDetails(t *testing.T) {
	dir := t.TempDir()
	fromPath := filepath.Join(dir, "source_test.go")
	toPath := filepath.Join(dir, "target.go")

	target := `package why

func ParseSwiftImports() {}
func SwiftImports() {}
`
	source := `package why

import "testing"

func TestX(t *testing.T) {
	ParseSwiftImports()
}
`

	if err := os.WriteFile(toPath, []byte(target), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(fromPath, []byte(source), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	members, err := findReferencedMembers(fromPath, toPath)
	if err != nil {
		t.Fatalf("findReferencedMembers() error = %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 usage, got %#v", members)
	}
	if members[0].Callee.Name != "ParseSwiftImports" {
		t.Fatalf("expected callee ParseSwiftImports, got %#v", members[0])
	}
	if members[0].Callee.Meta.Kind != SymbolKindFunc {
		t.Fatalf("expected callee kind func, got %#v", members[0])
	}
	if members[0].Caller != "TestX" {
		t.Fatalf("expected caller TestX, got %#v", members[0])
	}
	if members[0].Line <= 0 {
		t.Fatalf("expected a valid line number, got %#v", members[0])
	}
}

func TestWhyCommand_TextShowsMembersForParserAndTest(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", "../..", "depgraph/swift/parser_swift.go", "depgraph/swift/parser_swift_test.go"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "members:") {
		t.Fatalf("expected members section in output, got:\n%s", output)
	}
	if !strings.Contains(output, "calls:") {
		t.Fatalf("expected calls section in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ParseSwiftImports") {
		t.Fatalf("expected ParseSwiftImports in members, got:\n%s", output)
	}
	if !strings.Contains(output, "(func)") {
		t.Fatalf("expected function kind labels in output, got:\n%s", output)
	}
	if !strings.Contains(output, "TestParseSwiftImports") {
		t.Fatalf("expected caller test function in output, got:\n%s", output)
	}
}

func TestParseGoTopLevelMembers_CapturesSymbolKinds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.go")
	source := `package why

const Pi = 3.14
var globalFlag = true
type Graph struct{}
func ParseSwiftImports() {}
func (g *Graph) Resolve() {}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	members, err := parseGoTopLevelMembers(path)
	if err != nil {
		t.Fatalf("parseGoTopLevelMembers() error = %v", err)
	}

	if members["Pi"].Kind != SymbolKindConst {
		t.Fatalf("expected const kind for Pi, got %#v", members["Pi"])
	}
	if members["globalFlag"].Kind != SymbolKindVar {
		t.Fatalf("expected var kind for globalFlag, got %#v", members["globalFlag"])
	}
	if members["Graph"].Kind != SymbolKindType {
		t.Fatalf("expected type kind for Graph, got %#v", members["Graph"])
	}
	if members["ParseSwiftImports"].Kind != SymbolKindFunc {
		t.Fatalf("expected func kind for ParseSwiftImports, got %#v", members["ParseSwiftImports"])
	}
	if members["Resolve"].Kind != SymbolKindMethod {
		t.Fatalf("expected method kind for Resolve, got %#v", members["Resolve"])
	}
	if members["Resolve"].Receiver == "" {
		t.Fatalf("expected method receiver for Resolve, got %#v", members["Resolve"])
	}
}

func TestWhyCommand_MermaidShowsKindAwareLabels(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.go")
	sourcePath := filepath.Join(dir, "source_test.go")

	target := `package why
const Pi = 3.14
type Graph struct{}
func ParseSwiftImports() {}
`
	source := `package why
func TestX() {
	ParseSwiftImports()
}
`
	if err := os.WriteFile(targetPath, []byte(target), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", dir, "-f", "mermaid", "target.go", "source_test.go"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "ParseSwiftImports()") {
		t.Fatalf("expected function-style label in mermaid, got:\n%s", output)
	}
}

func TestWhyCommand_MermaidShowsMethodReceiverLabels(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.go")
	sourcePath := filepath.Join(dir, "source_test.go")

	target := `package why
type Graph struct{}
func (g *Graph) Resolve() {}
`
	source := `package why
func TestX() {
	var g Graph
	g.Resolve()
}
`
	if err := os.WriteFile(targetPath, []byte(target), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", dir, "-f", "mermaid", "target.go", "source_test.go"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "(*Graph).Resolve()") {
		t.Fatalf("expected method receiver label in mermaid, got:\n%s", output)
	}
	if !strings.Contains(output, "L") || !strings.Contains(output, "(calls method *Graph)") {
		t.Fatalf("expected method+line relationship label in mermaid, got:\n%s", output)
	}
}

func TestWhyCommand_DOTFormat_Golden(t *testing.T) {
	repoDir := t.TempDir()
	targetPath := filepath.Join(repoDir, "target.go")
	sourcePath := filepath.Join(repoDir, "source_test.go")

	target := `package why
const Pi = 3.14
type Graph struct{}
func ParseSwiftImports() {}
func (g *Graph) Resolve() {}
`
	source := `package why
func TestX() {
	ParseSwiftImports()
	var g Graph
	g.Resolve()
	_ = Pi
}
`
	if err := os.WriteFile(targetPath, []byte(target), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-f", "dot", "target.go", "source_test.go"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), stdout.Bytes())
}

func TestWhyCommand_MermaidFormat_Golden(t *testing.T) {
	repoDir := t.TempDir()
	targetPath := filepath.Join(repoDir, "target.go")
	sourcePath := filepath.Join(repoDir, "source_test.go")

	target := `package why
const Pi = 3.14
type Graph struct{}
func ParseSwiftImports() {}
func (g *Graph) Resolve() {}
`
	source := `package why
func TestX() {
	ParseSwiftImports()
	var g Graph
	g.Resolve()
	_ = Pi
}
`
	if err := os.WriteFile(targetPath, []byte(target), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-f", "mermaid", "target.go", "source_test.go"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), stdout.Bytes())
}
