package show

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeModulesConfig(t *testing.T, repoDir, body string) {
	t.Helper()
	configDir := filepath.Join(repoDir, ".clarity")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "modules.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func TestLoadConfigModules_AbsentReturnsNil(t *testing.T) {
	modules, err := loadConfigModules(t.TempDir())
	if err != nil {
		t.Fatalf("loadConfigModules() error = %v", err)
	}
	if modules != nil {
		t.Fatalf("expected nil modules when config absent, got %v", modules)
	}
}

func TestLoadConfigModules_ReadsLiteralFiles(t *testing.T) {
	repoDir := t.TempDir()
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "core", "files": ["src/main.go", "src/utils.go"] }
  ]
}`)

	modules, err := loadConfigModules(repoDir)
	if err != nil {
		t.Fatalf("loadConfigModules() error = %v", err)
	}
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}
	if modules[0].Name != "core" {
		t.Fatalf("expected name %q, got %q", "core", modules[0].Name)
	}
	want := []string{
		filepath.Join(repoDir, "src", "main.go"),
		filepath.Join(repoDir, "src", "utils.go"),
	}
	if len(modules[0].Files) != len(want) {
		t.Fatalf("expected %d files, got %d (%v)", len(want), len(modules[0].Files), modules[0].Files)
	}
	for i, f := range modules[0].Files {
		if f != want[i] {
			t.Fatalf("file[%d]: expected %q, got %q", i, want[i], f)
		}
	}
}

func TestLoadConfigModules_ExpandsGlobs(t *testing.T) {
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(pkgDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
	}
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "pkg", "files": ["pkg/*.go"] }
  ]
}`)

	modules, err := loadConfigModules(repoDir)
	if err != nil {
		t.Fatalf("loadConfigModules() error = %v", err)
	}
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}
	want := []string{
		filepath.Join(pkgDir, "a.go"),
		filepath.Join(pkgDir, "b.go"),
	}
	if len(modules[0].Files) != len(want) {
		t.Fatalf("expected %d files from glob, got %d (%v)", len(want), len(modules[0].Files), modules[0].Files)
	}
	for i, f := range modules[0].Files {
		if f != want[i] {
			t.Fatalf("file[%d]: expected %q, got %q", i, want[i], f)
		}
	}
}

func TestLoadConfigModules_RejectsInvalidJSON(t *testing.T) {
	repoDir := t.TempDir()
	writeModulesConfig(t, repoDir, `{ this is not json `)
	if _, err := loadConfigModules(repoDir); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadConfigModules_RejectsMissingName(t *testing.T) {
	repoDir := t.TempDir()
	writeModulesConfig(t, repoDir, `{ "modules": [ { "files": ["a.go"] } ] }`)
	if _, err := loadConfigModules(repoDir); err == nil {
		t.Fatal("expected error for missing module name, got nil")
	}
}

func TestLoadConfigModules_RejectsEmptyFileList(t *testing.T) {
	repoDir := t.TempDir()
	writeModulesConfig(t, repoDir, `{ "modules": [ { "name": "core", "files": [] } ] }`)
	if _, err := loadConfigModules(repoDir); err == nil {
		t.Fatal("expected error for empty file list, got nil")
	}
}

func TestShowCommand_ModulesFlagAppliesConfig(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	out := runShow(t, srcDir, "-r", repoDir, "-f", "dot", "--collapse")
	if !strings.Contains(out, `"support"`) {
		t.Fatalf("expected --modules to collapse into module node \"support\", got:\n%s", out)
	}
	if strings.Contains(out, "Helper.java") {
		t.Fatalf("expected Helper.java collapsed into module, but it is still a node:\n%s", out)
	}
}

func TestShowCommand_ModulesOffByDefault(t *testing.T) {
	repoDir, srcDir := writeJavaPair(t)
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["src/main/java/com/example/util/Helper.java"] }
  ]
}`)

	// Without --modules the config is ignored, even though modules.json exists.
	out := runShow(t, srcDir, "-r", repoDir, "-f", "dot")
	if strings.Contains(out, `"support"`) {
		t.Fatalf("expected modules off by default, but module node present:\n%s", out)
	}
	if !strings.Contains(out, "Helper.java") {
		t.Fatalf("expected Helper.java as a node when modules are off, got:\n%s", out)
	}
}

func writeJavaPair(t *testing.T) (repoDir, srcDir string) {
	t.Helper()
	// Resolve symlinks so an absolute path argument matches the symlink-normalized repo
	// base (macOS /var -> /private/var), keeping graph node keys and config paths
	// consistent the way they are when clarity runs against a real repo.
	repoDir = resolveSymlinks(t.TempDir())
	dir := filepath.Join(repoDir, "src", "main", "java", "com", "example")
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
	return repoDir, filepath.Join(repoDir, "src")
}

func runShow(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewCommand()
	cmd.SetArgs(args)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute(%v) error = %v", args, err)
	}
	return stdout.String()
}
