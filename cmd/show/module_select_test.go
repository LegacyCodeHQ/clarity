package show

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runShowErr(t *testing.T, args ...string) error {
	t.Helper()
	cmd := NewCommand()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	return cmd.Execute()
}

func TestShowCommand_ModuleSelectDrawsBoundary(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// App.java imports Helper.java; "support" contains Helper.java, so App is an
	// incoming dependent crossing the module boundary.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "support")

	if !strings.Contains(out, "subgraph cluster") {
		t.Fatalf("expected a boundary box (subgraph cluster) for the module:\n%s", out)
	}
	if !strings.Contains(out, `label="support"`) {
		t.Fatalf("expected the boundary labeled with the module name:\n%s", out)
	}
	// The member stays expanded inside the box (not collapsed into one node).
	if !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected the member Helper.java rendered inside the boundary:\n%s", out)
	}
	// The incoming dependent is shown, dashed (pruned/incomplete styling).
	if !strings.Contains(out, "App.java") || !strings.Contains(out, "dashed") {
		t.Fatalf("expected App.java shown as a dashed incoming dependent:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectDirectionIn(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "support", "-d", "in")
	if !strings.Contains(out, "App.java") {
		t.Fatalf("expected the incoming dependent App.java with -d in:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectDirectionOutExcludesIncoming(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	// Helper has no outgoing dependencies, and App is incoming only — so -d out
	// must drop App entirely while keeping the member.
	out := runShow(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "support", "-d", "out")
	if strings.Contains(out, "App.java") {
		t.Fatalf("expected -d out to exclude the incoming dependent App.java:\n%s", out)
	}
	if !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected the member Helper.java still rendered with -d out:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectIsolatedDrawsNoBoundary(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// Both files are members, so App->Helper is internal and nothing crosses the
	// boundary. With no dependents or dependencies, no box is drawn.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "bundle", "files": ["src/main/java/com/example/App.java", "src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "bundle")
	if strings.Contains(out, "subgraph cluster") {
		t.Fatalf("expected no boundary box for an isolated module:\n%s", out)
	}
	if !strings.Contains(out, "App.java") || !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected the module's files still rendered without a box:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectThroughSymlinkedRepo(t *testing.T) {
	// Reproduces the /tmp -> /private/tmp aliasing: the repo is reached through a
	// symlink. Module member paths (derived from the symlink-resolved repo root)
	// must still match graph node keys (derived from the -i input path).
	realRepo := resolveSymlinks(t.TempDir())
	dir := filepath.Join(realRepo, "src", "main", "java", "com", "example")
	if err := os.MkdirAll(filepath.Join(dir, "util"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	appContent := "package com.example;\n\nimport com.example.util.Helper;\n\npublic class App {}\n"
	if err := os.WriteFile(filepath.Join(dir, "App.java"), []byte(appContent), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "util", "Helper.java"), []byte("package com.example.util;\n\npublic class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	writeModulesConfig(t, realRepo, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	linkRepo := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(realRepo, linkRepo); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	out := runShow(t, "-i", filepath.Join(linkRepo, "src"), "-r", linkRepo, "-f", "dot", "--module", "support")
	if !strings.Contains(out, "subgraph cluster") {
		t.Fatalf("expected the boundary to draw through a symlinked repo path, got:\n%s", out)
	}
	if !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected the member resolved through a symlinked path:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectUnknownName(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	err := runShowErr(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "nope")
	if err == nil {
		t.Fatal("expected an error for an unknown module name, got nil")
	}
	if !strings.Contains(err.Error(), "support") {
		t.Fatalf("expected the error to list available modules, got: %v", err)
	}
}

func TestShowCommand_ModuleSelectNoMembersInScope(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// "ghost" is declared but its file is not in the rendered scope.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "ghost", "files": ["src/main/java/com/example/Missing.java"] }
  ]
}`)

	err := runShowErr(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "--module", "ghost")
	if err == nil {
		t.Fatal("expected an error when the module has no files in scope, got nil")
	}
	if !strings.Contains(err.Error(), "no files in the current scope") {
		t.Fatalf("expected a scope error, got: %v", err)
	}
}

func TestShowCommand_ModuleDirectionRequiresModule(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	err := runShowErr(t, "-i", srcDir, "-r", repoDir, "-f", "dot", "-d", "in")
	if err == nil {
		t.Fatal("expected an error when --direction is used without --module, got nil")
	}
	if !strings.Contains(err.Error(), "--module") {
		t.Fatalf("expected the error to mention --module, got: %v", err)
	}
}
