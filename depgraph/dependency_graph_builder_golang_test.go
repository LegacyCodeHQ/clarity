package depgraph_test

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

func TestBuildDependencyGraph_GoTypeReachesItsMethodProviders(t *testing.T) {
	// Go splits a type and (some of) its methods across files. The factory
	// in formatter.go constructs &dotFormatter{}; the dotFormatter behaviour
	// lives in formatter_dot.go as methods on that type. A method file defines
	// no package-level symbol anyone references, so nothing edges *into* it —
	// the only edge runs method -> type (the receiver reference). Without a
	// type -> method edge, reaching the type (e.g. via the constructor) never
	// reaches its behaviour, and formatter_dot.go silently drops out of the
	// graph. Assert the type's file reaches the files that add methods to it.
	goModPath := filepath.Clean("/virtual/go.mod")
	typePath := filepath.Clean("/virtual/formatters/formatter.go")
	methodPath := filepath.Clean("/virtual/formatters/formatter_dot.go")

	reader := mapContentReader(map[string]string{
		goModPath: "module fmtmod\n\ngo 1.25\n",
		typePath: `package formatters

type dotFormatter struct{}

func NewFormatter() *dotFormatter {
	return &dotFormatter{}
}
`,
		methodPath: `package formatters

func (f *dotFormatter) Format() string {
	return "dot"
}
`,
	})

	files := []string{typePath, methodPath}
	graph, err := depgraph.BuildDependencyGraph(files, reader)
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	assert.Contains(t, adj[typePath], methodPath,
		"the file defining a type should reach the files that add methods to that type")
}

func TestBuildDependencyGraph_GoMethodOwnershipNeverEdgesToTestFile(t *testing.T) {
	// A _test.go in the production package may add methods to a production
	// type. Method-ownership must not turn that into a production -> test edge:
	// production code never depends on tests.
	goModPath := filepath.Clean("/virtual/go.mod")
	typePath := filepath.Clean("/virtual/widget/widget.go")
	testMethodPath := filepath.Clean("/virtual/widget/widget_test.go")

	reader := mapContentReader(map[string]string{
		goModPath: "module widgetmod\n\ngo 1.25\n",
		typePath: `package widget

type Widget struct{}
`,
		testMethodPath: `package widget

func (w *Widget) testOnlyHelper() string {
	return "test"
}
`,
	})

	files := []string{typePath, testMethodPath}
	graph, err := depgraph.BuildDependencyGraph(files, reader)
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	assert.NotContains(t, adj[typePath], testMethodPath,
		"a production type file must never gain an edge to a test method file")
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
