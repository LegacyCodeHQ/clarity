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
	if !strings.Contains(out, "Module") || !strings.Contains(out, "Non-test") || !strings.Contains(out, "Total") {
		t.Fatalf("expected the table header, got:\n%s", out)
	}
	if !strings.Contains(out, "─") {
		t.Fatalf("expected a rule separating the header from the rows, got:\n%s", out)
	}
}

func TestModules_SplitsTestFromNonTest(t *testing.T) {
	repoDir := t.TempDir()
	for _, f := range []string{"core.go", "util.go", "core_test.go"} {
		if err := os.WriteFile(filepath.Join(repoDir, f), []byte("package core\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%s) error = %v", f, err)
		}
	}
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "core", "files": ["*.go"] }
  ]
}`)

	out := runModulesCmd(t, "--repo", repoDir)

	// core.go + util.go are non-test; core_test.go is a test → 2 / 1 / 3.
	if !strings.Contains(out, "1 modules · 2 non-test · 1 test · 3 files") {
		t.Fatalf("expected the test/non-test split in the summary, got:\n%s", out)
	}
}

func TestModules_SortBySizeOrdersLargestFirst(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "alpha.go"), []byte("package alpha\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	zetaDir := filepath.Join(repoDir, "zeta")
	if err := os.MkdirAll(zetaDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	for _, f := range []string{"z1.go", "z2.go", "z3.go"} {
		if err := os.WriteFile(filepath.Join(zetaDir, f), []byte("package zeta\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
	}
	writeModulesConfig(t, repoDir, `{
  "modules": [
    { "name": "alpha", "files": ["alpha.go"] },
    { "name": "zeta", "files": ["zeta/*.go"] }
  ]
}`)

	// alpha (1 file) sorts before zeta by name, so --sort-by size must reverse it.
	out := runModulesCmd(t, "--repo", repoDir, "--sort-by", "size")
	zIdx := strings.Index(out, "zeta")
	aIdx := strings.Index(out, "alpha")
	if zIdx == -1 || aIdx == -1 {
		t.Fatalf("expected both modules listed, got:\n%s", out)
	}
	if zIdx > aIdx {
		t.Fatalf("expected zeta (larger) before alpha with --sort-by size, got:\n%s", out)
	}
}

func TestModules_SortByRejectsUnknownValue(t *testing.T) {
	repoDir := t.TempDir()
	writeModulesConfig(t, repoDir, `{ "modules": [ { "name": "a", "files": ["a.go"] } ] }`)

	cmd := NewCommand()
	cmd.SetArgs([]string{"--repo", repoDir, "--sort-by", "bogus"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected an error for unknown --sort-by value, got:\n%s", out.String())
	}
}

func TestModules_EmptyWhenConfigAbsent(t *testing.T) {
	repoDir := t.TempDir()
	out := runModulesCmd(t, "--repo", repoDir)
	if !strings.Contains(out, "No modules declared") {
		t.Fatalf("expected an empty-state message, got:\n%s", out)
	}
}
