package modules

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

func runModulesCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewCommand()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute(%v) error = %v", args, err)
	}
	return out.String()
}

func TestModules_ListsDeclaredModulesText(t *testing.T) {
	repoDir := t.TempDir()
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "support", "files": ["a.go"] },
    { "name": "core", "files": ["main.go", "util.go"] }
  ]
}`)

	out := runModulesCmd(t, "--repo", repoDir)

	// Alphabetical: core before support, with resolved file counts.
	coreIdx := strings.Index(out, "core")
	supportIdx := strings.Index(out, "support")
	if coreIdx == -1 || supportIdx == -1 {
		t.Fatalf("expected both modules listed, got:\n%s", out)
	}
	if coreIdx > supportIdx {
		t.Fatalf("expected modules sorted alphabetically (core before support), got:\n%s", out)
	}
	if !strings.Contains(out, "MODULE") || !strings.Contains(out, "FILES") {
		t.Fatalf("expected a table header, got:\n%s", out)
	}
}

func TestModules_JSON(t *testing.T) {
	repoDir := t.TempDir()
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "core", "files": ["main.go", "util.go"] }
  ]
}`)

	out := runModulesCmd(t, "--repo", repoDir, "--format", "json")
	if !strings.Contains(out, `"name": "core"`) {
		t.Fatalf("expected JSON with module name, got:\n%s", out)
	}
	if !strings.Contains(out, `"file_count": 2`) {
		t.Fatalf("expected JSON with file_count 2, got:\n%s", out)
	}
}

func TestModules_EmptyWhenConfigAbsent(t *testing.T) {
	repoDir := t.TempDir()
	out := runModulesCmd(t, "--repo", repoDir)
	if !strings.Contains(out, "No modules declared") {
		t.Fatalf("expected an empty-state message, got:\n%s", out)
	}
}
