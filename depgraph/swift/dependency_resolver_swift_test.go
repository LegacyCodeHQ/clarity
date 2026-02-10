package swift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSwiftProjectImports_SwiftPMModule(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "App")
	fooDir := filepath.Join(tmpDir, "Sources", "Foo")
	require.NoError(t, os.MkdirAll(appDir, 0o755))
	require.NoError(t, os.MkdirAll(fooDir, 0o755))

	appPath := filepath.Join(appDir, "App.swift")
	require.NoError(t, os.WriteFile(appPath, []byte("import Foo\n\nstruct App {\n    let value: Foo\n}\n"), 0o644))

	fooPath := filepath.Join(fooDir, "Foo.swift")
	require.NoError(t, os.WriteFile(fooPath, []byte("struct Foo {}\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		appPath: true,
		fooPath: true,
	}

	imports, err := ResolveSwiftProjectImports(appPath, appPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{fooPath}, imports)
}

func TestResolveSwiftProjectImports_TestsModuleImportsMain(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "Sources", "Widget")
	testsDir := filepath.Join(tmpDir, "Tests", "WidgetTests")
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.MkdirAll(testsDir, 0o755))

	mainPath := filepath.Join(mainDir, "Widget.swift")
	require.NoError(t, os.WriteFile(mainPath, []byte("struct Widget {}\n"), 0o644))

	testPath := filepath.Join(testsDir, "WidgetTests.swift")
	require.NoError(t, os.WriteFile(testPath, []byte("import Widget\n\nfinal class WidgetTests {\n    let subject: Widget\n}\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		mainPath: true,
		testPath: true,
	}

	imports, err := ResolveSwiftProjectImports(testPath, testPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{mainPath}, imports)
}

func TestResolveSwiftProjectImports_FlatLayoutResolvesTypeReference(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "clarity-desktop")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	viewPath := filepath.Join(appDir, "DependencyGraphView.swift")
	require.NoError(t, os.WriteFile(viewPath, []byte("import SwiftUI\nimport ComposableArchitecture\n\nstruct DependencyGraphView: View {\n    let store: StoreOf<DependencyGraphFeature>\n}\n"), 0o644))

	featurePath := filepath.Join(appDir, "DependencyGraphFeature.swift")
	require.NoError(t, os.WriteFile(featurePath, []byte("import ComposableArchitecture\n\nstruct DependencyGraphFeature: Reducer {}\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		viewPath:    true,
		featurePath: true,
	}

	imports, err := ResolveSwiftProjectImports(viewPath, viewPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{featurePath}, imports)
}

func TestResolveSwiftProjectImports_FlatLayoutContentViewDependsOnModels(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "clarity-desktop")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	contentViewPath := filepath.Join(appDir, "ContentView.swift")
	require.NoError(t, os.WriteFile(contentViewPath, []byte(`
import SwiftUI

struct ContentView: View {
    var body: some View {
        DependencyGraphView(graph: .sample)
    }
}

private extension DependencyGraph {
    static let sample = DependencyGraph(
        title: "demo",
        commit: "abc123",
        files: [
            GraphFileNode(id: "n0", path: "README.md", additions: 1, deletions: 0),
        ],
        edges: [
            GraphEdge(from: "n0", to: "n0"),
        ]
    )
}
`), 0o644))

	modelsPath := filepath.Join(appDir, "DependencyGraphModels.swift")
	require.NoError(t, os.WriteFile(modelsPath, []byte(`
import Foundation

struct GraphFileNode: Identifiable, Hashable {
    let id: String
    let path: String
    let additions: Int
    let deletions: Int
}

struct GraphEdge: Hashable {
    let from: String
    let to: String
}

struct DependencyGraph: Hashable {
    let title: String
    let commit: String
    let files: [GraphFileNode]
    let edges: [GraphEdge]
}
`), 0o644))

	viewPath := filepath.Join(appDir, "DependencyGraphView.swift")
	require.NoError(t, os.WriteFile(viewPath, []byte(`
import SwiftUI

struct DependencyGraphView: View {
    let graph: DependencyGraph

    var body: some View {
        Text(graph.title)
    }
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		contentViewPath: true,
		modelsPath:      true,
		viewPath:        true,
	}

	imports, err := ResolveSwiftProjectImports(contentViewPath, contentViewPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{modelsPath, viewPath}, imports)
}
