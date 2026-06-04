package show

import (
	"path/filepath"
	"testing"
)

func TestBuildModules_ResolvesFilesRelativeToRepo(t *testing.T) {
	repoDir := t.TempDir()
	resolver, err := NewPathResolver(repoDir, false)
	if err != nil {
		t.Fatalf("NewPathResolver() error = %v", err)
	}

	modules, err := buildModules([]string{"core=src/main.go,src/utils.go"}, resolver)
	if err != nil {
		t.Fatalf("buildModules() error = %v", err)
	}

	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}
	if modules[0].Name != "core" {
		t.Fatalf("expected name %q, got %q", "core", modules[0].Name)
	}

	want := []string{
		filepath.Join(resolveSymlinks(repoDir), "src", "main.go"),
		filepath.Join(resolveSymlinks(repoDir), "src", "utils.go"),
	}
	if len(modules[0].Files) != len(want) {
		t.Fatalf("expected %d files, got %d", len(want), len(modules[0].Files))
	}
	for i, file := range modules[0].Files {
		if file != want[i] {
			t.Fatalf("file[%d]: expected %q, got %q", i, want[i], file)
		}
	}
}

func TestBuildModules_RejectsMissingName(t *testing.T) {
	resolver, err := NewPathResolver(t.TempDir(), false)
	if err != nil {
		t.Fatalf("NewPathResolver() error = %v", err)
	}

	if _, err := buildModules([]string{"=src/main.go"}, resolver); err == nil {
		t.Fatal("expected error for missing module name, got nil")
	}
	if _, err := buildModules([]string{"src/main.go"}, resolver); err == nil {
		t.Fatal("expected error for missing '=' separator, got nil")
	}
}

func TestBuildModules_RejectsEmptyFileList(t *testing.T) {
	resolver, err := NewPathResolver(t.TempDir(), false)
	if err != nil {
		t.Fatalf("NewPathResolver() error = %v", err)
	}

	if _, err := buildModules([]string{"core="}, resolver); err == nil {
		t.Fatal("expected error for empty file list, got nil")
	}
}
