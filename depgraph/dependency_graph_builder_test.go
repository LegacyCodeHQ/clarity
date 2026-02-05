package depgraph

import (
	"path/filepath"
	"testing"
)

type stubDependencyResolver struct {
	supportedByExt map[string]bool
	resolvedFiles  []string
}

func (s *stubDependencyResolver) SupportsFileExtension(ext string) bool {
	return s.supportedByExt[ext]
}

func (s *stubDependencyResolver) ResolveProjectImports(absPath, _, _ string) ([]string, error) {
	s.resolvedFiles = append(s.resolvedFiles, absPath)
	return []string{}, nil
}

func (s *stubDependencyResolver) FinalizeGraph(_ DependencyGraph) error {
	return nil
}

func TestBuildDependencyGraphWithResolver_UsesResolverExtensionSupport(t *testing.T) {
	t.Helper()

	resolver := &stubDependencyResolver{
		supportedByExt: map[string]bool{
			".go": true,
		},
	}

	filePaths := []string{"main.go", "README.md"}
	graph, err := BuildDependencyGraphWithResolver(filePaths, resolver)
	if err != nil {
		t.Fatalf("BuildDependencyGraphWithResolver() error = %v", err)
	}

	goPath, err := filepath.Abs("main.go")
	if err != nil {
		t.Fatalf("filepath.Abs(main.go) error = %v", err)
	}
	readmePath, err := filepath.Abs("README.md")
	if err != nil {
		t.Fatalf("filepath.Abs(README.md) error = %v", err)
	}

	if !ContainsNode(graph, goPath) {
		t.Fatalf("expected graph to contain %s", goPath)
	}
	if !ContainsNode(graph, readmePath) {
		t.Fatalf("expected graph to contain %s", readmePath)
	}
	if len(resolver.resolvedFiles) != 1 || resolver.resolvedFiles[0] != goPath {
		t.Fatalf("expected resolver to process only supported file, got %v", resolver.resolvedFiles)
	}
}
