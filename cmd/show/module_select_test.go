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

func TestShowCommand_ModuleSelectBoxesMembers(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// "support" contains Helper.java; App.java (also in scope) is not a member.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, srcDir, "-r", repoDir, "-f", "dot", "--module", "support")

	if !strings.Contains(out, "subgraph cluster") {
		t.Fatalf("expected a boundary box (subgraph cluster) for the module:\n%s", out)
	}
	if !strings.Contains(out, `label="support"`) {
		t.Fatalf("expected the box labeled with the module name:\n%s", out)
	}
	if !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected the member Helper.java boxed:\n%s", out)
	}
	// Everything else in scope renders too — the box frames, it does not filter.
	if !strings.Contains(out, "App.java") {
		t.Fatalf("expected non-member App.java still rendered (union scope):\n%s", out)
	}
}

func TestShowCommand_ModuleSelectMermaidSubgraph(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, srcDir, "-r", repoDir, "-f", "mermaid", "--module", "support")
	if !strings.Contains(out, `subgraph moduleCluster["support"]`) {
		t.Fatalf("expected a mermaid subgraph boundary labeled with the module name:\n%s", out)
	}
	// The subgraph sets its own direction so it lays out like DOT (rankdir=LR),
	// not Mermaid's default top-to-bottom.
	if !strings.Contains(out, "direction LR") {
		t.Fatalf("expected the subgraph to set direction LR for layout consistency:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectBoxesEvenWhenIsolated(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// Both files are members, so nothing crosses a boundary — the box still draws.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "bundle", "files": ["src/main/java/com/example/App.java", "src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, srcDir, "-r", repoDir, "-f", "dot", "--module", "bundle")
	if !strings.Contains(out, "subgraph cluster") {
		t.Fatalf("expected the module box to draw even when isolated:\n%s", out)
	}
	if !strings.Contains(out, "App.java") || !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected both members rendered inside the box:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectSelfScopesOnCleanTree(t *testing.T) {
	repoDir, _ := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)
	gitInitRepo(t, repoDir)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "seed")

	// No paths and a clean tree: the module supplies its own scope rather than
	// printing the "working directory is clean" hint.
	out := runShow(t, "-r", repoDir, "-f", "dot", "--module", "support")
	if strings.Contains(out, "Working directory is clean") {
		t.Fatalf("expected --module to self-scope on a clean tree, got:\n%s", out)
	}
	if !strings.Contains(out, "subgraph cluster") || !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected the module rendered from its own files:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectThroughSymlinkedRepo(t *testing.T) {
	// Reproduces the /tmp -> /private/tmp aliasing: the repo is reached through a
	// symlink. Module member paths (derived from the symlink-resolved repo root)
	// must still match graph node keys (derived from the positional path argument).
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

	out := runShow(t, filepath.Join(linkRepo, "src"), "-r", linkRepo, "-f", "dot", "--module", "support")
	if !strings.Contains(out, "subgraph cluster") {
		t.Fatalf("expected the box to draw through a symlinked repo path, got:\n%s", out)
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

	err := runShowErr(t, srcDir, "-r", repoDir, "-f", "dot", "--module", "nope")
	if err == nil {
		t.Fatal("expected an error for an unknown module name, got nil")
	}
	if !strings.Contains(err.Error(), "support") {
		t.Fatalf("expected the error to list available modules, got: %v", err)
	}
}

func TestShowCommand_CollapseCannotBeUsedWithModule(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	err := runShowErr(t, srcDir, "-r", repoDir, "-f", "dot", "--collapse", "--module", "support")
	if err == nil {
		t.Fatal("expected an error when --collapse is used with --module, got nil")
	}
	if !strings.Contains(err.Error(), "--collapse cannot be used with --module") {
		t.Fatalf("expected collapse/module conflict error, got: %v", err)
	}
}

func TestShowCommand_ModuleSelectResolvesToNoFiles(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	// The declared file does not exist, so the module resolves to nothing.
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "ghost", "files": ["src/main/java/com/example/Missing.java"] }
  ]
}`)

	err := runShowErr(t, srcDir, "-r", repoDir, "-f", "dot", "--module", "ghost")
	if err == nil {
		t.Fatal("expected an error when the module resolves to no files, got nil")
	}
	if !strings.Contains(err.Error(), "resolves to no files") {
		t.Fatalf("expected a no-files error, got: %v", err)
	}
}

func seedCommittedJavaRepo(t *testing.T, modulesConfig string) string {
	t.Helper()
	repoDir, _ := writeJavaPair(t)
	writeModulesConfig(t, repoDir, modulesConfig)
	gitInitRepo(t, repoDir)
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "seed")
	return repoDir
}

func TestShowCommand_ModuleSelectDirectionInShowsDependentsPruned(t *testing.T) {
	repoDir := seedCommittedJavaRepo(t, `{
  "modules": [ { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] } ]
}`)

	// App.java imports Helper (the module), so -d in surfaces it as a pruned
	// dependent on a clean tree (it is context, not a change).
	out := runShow(t, "-r", repoDir, "-f", "dot", "--module", "support", "--reach", "up")
	if !strings.Contains(out, "subgraph cluster") || !strings.Contains(out, "App.java") {
		t.Fatalf("expected the box and the incoming dependent App.java:\n%s", out)
	}
	if !strings.Contains(out, "dashed") {
		t.Fatalf("expected the dependent styled as pruned (dashed):\n%s", out)
	}
}

func TestShowCommand_ModuleSelectDirectionOutShowsDependenciesPruned(t *testing.T) {
	repoDir := seedCommittedJavaRepo(t, `{
  "modules": [ { "name": "app", "files": ["src/main/java/com/example/App.java"] } ]
}`)

	// App imports Helper, so -d out surfaces Helper as a pruned dependency.
	out := runShow(t, "-r", repoDir, "-f", "dot", "--module", "app", "--reach", "down")
	if !strings.Contains(out, "Helper.java") || !strings.Contains(out, "dashed") {
		t.Fatalf("expected Helper.java as a pruned out-dependency:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectDefaultShowsNoNeighbors(t *testing.T) {
	repoDir := seedCommittedJavaRepo(t, `{
  "modules": [ { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] } ]
}`)

	// Default (none): just the module box, no dependents/dependencies, nothing pruned.
	out := runShow(t, "-r", repoDir, "-f", "dot", "--module", "support")
	if strings.Contains(out, "App.java") || strings.Contains(out, "dashed") {
		t.Fatalf("expected no neighbors and nothing pruned by default:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectCommitUsesCommitConfig(t *testing.T) {
	repoDir := seedCommittedJavaRepo(t, `{
  "modules": [ { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] } ]
}`)

	writeModulesConfig(t, repoDir, `{
  "modules": [ { "name": "working", "files": ["src/main/java/com/example/App.java"] } ]
}`)

	out := runShow(t, "-r", repoDir, "-c", "HEAD", "-f", "dot", "--module", "support")
	if !strings.Contains(out, "subgraph cluster") || !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected --commit --module to use module config from HEAD, got:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectCommitIgnoresWorkingTreeDeletion(t *testing.T) {
	repoDir := seedCommittedJavaRepo(t, `{
  "modules": [ { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] } ]
}`)

	helperPath := filepath.Join(repoDir, "src", "main", "java", "com", "example", "util", "Helper.java")
	if err := os.Remove(helperPath); err != nil {
		t.Fatalf("os.Remove() error = %v", err)
	}

	out := runShow(t, "-r", repoDir, "-c", "HEAD", "-f", "dot", "--module", "support")
	if !strings.Contains(out, "subgraph cluster") || !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected --commit --module to resolve members from HEAD despite working-tree deletion, got:\n%s", out)
	}
}

func TestShowCommand_ModuleSelectChangedNeighborKeepsChangeStyling(t *testing.T) {
	repoDir := seedCommittedJavaRepo(t, `{
  "modules": [ { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] } ]
}`)
	// Edit the dependent so it is both an incoming neighbor and a working-set change.
	appPath := filepath.Join(repoDir, "src", "main", "java", "com", "example", "App.java")
	edited := "package com.example;\n\nimport com.example.util.Helper;\n\npublic class App { /* edit */ }\n"
	if err := os.WriteFile(appPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	out := runShow(t, "-r", repoDir, "-f", "dot", "--module", "support", "--reach", "up")
	if !strings.Contains(out, "App.java") {
		t.Fatalf("expected the changed dependent App.java rendered:\n%s", out)
	}
	// Changed beats pruned: App is a neighbor but also changed, so nothing is pruned.
	if strings.Contains(out, "dashed") {
		t.Fatalf("expected a changed neighbor to keep change styling, not pruned:\n%s", out)
	}
}

// (removed TestShowCommand_ModuleDirectionRequiresModule: --reach has no
// "requires --module" rule; it applies to file and path anchors too.)
