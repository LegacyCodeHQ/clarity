package golang_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mapContentReader(contents map[string]string) vcs.ContentReader {
	return func(filePath string) ([]byte, error) {
		content, ok := contents[filePath]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(content), nil
	}
}

func mustAdjacency(t *testing.T, g depgraph.DependencyGraph) map[string][]string {
	t.Helper()
	adj, err := depgraph.AdjacencyList(g)
	require.NoError(t, err)
	return adj
}

func TestBuildDependencyGraph_GoEmbedGlobIncludesAllMatches(t *testing.T) {
	tmpDir := t.TempDir()

	goModPath := filepath.Join(tmpDir, "go.mod")
	require.NoError(t, os.WriteFile(goModPath, []byte("module embedglob\n\ngo 1.25\n"), 0644))

	templatesDir := filepath.Join(tmpDir, "templates")
	require.NoError(t, os.Mkdir(templatesDir, 0755))

	mainPath := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

import _ "embed"

//go:embed templates/*.html
var templates string
`
	require.NoError(t, os.WriteFile(mainPath, []byte(mainContent), 0644))

	indexPath := filepath.Join(templatesDir, "index.html")
	require.NoError(t, os.WriteFile(indexPath, []byte("<h1>Index</h1>"), 0644))

	aboutPath := filepath.Join(templatesDir, "about.html")
	require.NoError(t, os.WriteFile(aboutPath, []byte("<h1>About</h1>"), 0644))

	files := []string{mainPath, indexPath, aboutPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 2, "glob embed should include all matching files")
	assert.Contains(t, mainDeps, indexPath)
	assert.Contains(t, mainDeps, aboutPath)
}

func TestBuildDependencyGraph_GoVirtualReaderResolvesModuleRoot(t *testing.T) {
	mainPath := filepath.Clean("/virtual/main.go")
	libPath := filepath.Clean("/virtual/pkg/lib.go")
	goModPath := filepath.Clean("/virtual/go.mod")

	reader := mapContentReader(map[string]string{
		goModPath: "module virtualmod\n\ngo 1.25\n",
		mainPath: `package main

import "virtualmod/pkg"

func main() {
	_ = pkg.Helper()
}
`,
		libPath: `package pkg

func Helper() string {
	return "ok"
}
`,
	})

	files := []string{mainPath, libPath}
	graph, err := depgraph.BuildDependencyGraph(files, reader)
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 1, "virtual content reader should resolve go.mod without filesystem stat")
	assert.Contains(t, mainDeps, libPath)
}

func TestBuildDependencyGraph_GoModReplaceResolvesLocalDependency(t *testing.T) {
	tmpDir := t.TempDir()

	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := `module app

go 1.25

replace example.com/shared => ./third_party/shared
`
	require.NoError(t, os.WriteFile(goModPath, []byte(goModContent), 0644))

	mainPath := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

import "example.com/shared"

func main() {
	_ = shared.Version()
}
`
	require.NoError(t, os.WriteFile(mainPath, []byte(mainContent), 0644))

	sharedDir := filepath.Join(tmpDir, "third_party", "shared")
	require.NoError(t, os.MkdirAll(sharedDir, 0755))

	sharedPath := filepath.Join(sharedDir, "shared.go")
	sharedContent := `package shared

func Version() string {
	return "v1"
}
`
	require.NoError(t, os.WriteFile(sharedPath, []byte(sharedContent), 0644))

	files := []string{mainPath, sharedPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 1, "replace directive should map external import to local package")
	assert.Contains(t, mainDeps, sharedPath)
}

func TestBuildDependencyGraph_GoDotImportResolvesUsedSymbolsOnly(t *testing.T) {
	tmpDir := t.TempDir()

	goModPath := filepath.Join(tmpDir, "go.mod")
	require.NoError(t, os.WriteFile(goModPath, []byte("module dotmod\n\ngo 1.25\n"), 0644))

	pkgDir := filepath.Join(tmpDir, "pkg")
	require.NoError(t, os.Mkdir(pkgDir, 0755))

	fooPath := filepath.Join(pkgDir, "foo.go")
	require.NoError(t, os.WriteFile(fooPath, []byte(`package pkg

func Foo() string {
	return "foo"
}
`), 0644))

	barPath := filepath.Join(pkgDir, "bar.go")
	require.NoError(t, os.WriteFile(barPath, []byte(`package pkg

func Bar() string {
	return "bar"
}
`), 0644))

	mainPath := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

import . "dotmod/pkg"

func main() {
	_ = Foo()
}
`
	require.NoError(t, os.WriteFile(mainPath, []byte(mainContent), 0644))

	files := []string{mainPath, fooPath, barPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 1, "dot import should link only symbols actually used")
	assert.Contains(t, mainDeps, fooPath)
	assert.NotContains(t, mainDeps, barPath)
}
