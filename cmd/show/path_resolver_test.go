package show

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathResolverResolve_WithRepoBase_ResolvesRelativePathFromRepo(t *testing.T) {
	repoDir := t.TempDir()
	resolver, err := NewPathResolver(repoDir, false)
	if err != nil {
		t.Fatalf("NewPathResolver() error = %v", err)
	}

	resolved, err := resolver.Resolve(RawPath(filepath.Join("src", "main.go")))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expected := filepath.Join(resolveSymlinks(repoDir), "src", "main.go")
	if resolved.String() != expected {
		t.Fatalf("expected %q, got %q", expected, resolved.String())
	}
}

func TestPathResolverResolve_WithoutRepoBase_UsesCurrentWorkingDirectory(t *testing.T) {
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalDir); chdirErr != nil {
			t.Fatalf("os.Chdir() cleanup error = %v", chdirErr)
		}
	})
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	resolver, err := NewPathResolver("", false)
	if err != nil {
		t.Fatalf("NewPathResolver() error = %v", err)
	}

	resolved, err := resolver.Resolve(RawPath("main.go"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	expected, err := filepath.Abs("main.go")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	expected = resolveSymlinks(expected)
	if resolved.String() != expected {
		t.Fatalf("expected %q, got %q", expected, resolved.String())
	}
}

func TestPathResolverResolve_AbsolutePath_Unchanged(t *testing.T) {
	resolver, err := NewPathResolver(t.TempDir(), true)
	if err != nil {
		t.Fatalf("NewPathResolver() error = %v", err)
	}

	absolutePath := filepath.Join(t.TempDir(), "main.go")
	resolved, err := resolver.Resolve(RawPath(absolutePath))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resolved.String() != absolutePath {
		t.Fatalf("expected absolute path to be unchanged: %q, got %q", absolutePath, resolved.String())
	}
}

func TestPathResolverResolve_AbsolutePathOutsideRepo_Disallowed(t *testing.T) {
	repoDir := t.TempDir()
	resolver, err := NewPathResolver(repoDir, false)
	if err != nil {
		t.Fatalf("NewPathResolver() error = %v", err)
	}

	outsidePath := filepath.Join(t.TempDir(), "main.go")
	_, err = resolver.Resolve(RawPath(outsidePath))
	if err == nil {
		t.Fatalf("expected error for path outside repo, got nil")
	}
}
