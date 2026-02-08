package why

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if members[0].Callee != "ParseSwiftImports" {
		t.Fatalf("expected callee ParseSwiftImports, got %#v", members[0])
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
	if !strings.Contains(output, "TestParseSwiftImports") {
		t.Fatalf("expected caller test function in output, got:\n%s", output)
	}
}
