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

func TestResolveSwiftProjectImports_TopLevelFunctionReference(t *testing.T) {
	// CLR-14: a file that calls a top-level (global) function declared in
	// another file must produce a file->file edge. Mirrors the Strata repro
	// where DiffView.swift calls numberedRows/sideBySideRows declared as
	// global funcs in Diff.swift, yet Diff.swift showed zero incoming edges.
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "StrataKit")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	diffPath := filepath.Join(appDir, "Diff.swift")
	require.NoError(t, os.WriteFile(diffPath, []byte(`
import Foundation

func sideBySideRows(_ text: String) -> [String] {
    return [text]
}

func numberedRows(_ rows: [String]) -> [String] {
    return rows
}
`), 0o644))

	viewPath := filepath.Join(appDir, "DiffView.swift")
	require.NoError(t, os.WriteFile(viewPath, []byte(`
import SwiftUI

struct DiffView: View {
    var body: some View {
        let rows = numberedRows(sideBySideRows("hello"))
        return Text(rows.joined())
    }
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		diffPath: true,
		viewPath: true,
	}

	imports, err := ResolveSwiftProjectImports(viewPath, viewPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{diffPath}, imports)
}

func TestResolveSwiftProjectImports_TopLevelConstantReference(t *testing.T) {
	// CLR-14: a file that references a top-level (global) let/var declared in
	// another file must produce a file->file edge.
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "StrataKit")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	constantsPath := filepath.Join(appDir, "Constants.swift")
	require.NoError(t, os.WriteFile(constantsPath, []byte(`
import Foundation

let defaultTabWidth = 4
`), 0o644))

	consumerPath := filepath.Join(appDir, "Formatter.swift")
	require.NoError(t, os.WriteFile(consumerPath, []byte(`
import Foundation

struct Formatter {
    func indent() -> Int {
        return defaultTabWidth * 2
    }
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		constantsPath: true,
		consumerPath:  true,
	}

	imports, err := ResolveSwiftProjectImports(consumerPath, consumerPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{constantsPath}, imports)
}

func TestResolveSwiftProjectImports_CustomOperatorReference(t *testing.T) {
	// CLR-14: a file that uses a custom operator declared/implemented in
	// another file must produce a file->file edge.
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "StrataKit")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	operatorPath := filepath.Join(appDir, "Operators.swift")
	require.NoError(t, os.WriteFile(operatorPath, []byte(`
import Foundation

infix operator |>: AdditionPrecedence

func |> (value: Int, transform: (Int) -> Int) -> Int {
    return transform(value)
}
`), 0o644))

	consumerPath := filepath.Join(appDir, "Pipeline.swift")
	require.NoError(t, os.WriteFile(consumerPath, []byte(`
import Foundation

struct Pipeline {
    func run() -> Int {
        return 3 |> { $0 + 1 }
    }
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		operatorPath: true,
		consumerPath: true,
	}

	imports, err := ResolveSwiftProjectImports(consumerPath, consumerPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{operatorPath}, imports)
}

func TestResolveSwiftProjectImports_NoEdgeForFunctionLocalDeclaration(t *testing.T) {
	// CLR-15 variant 1: a type declared inside a function body must not be
	// treated as an external reference to a same-named top-level type in
	// another file.
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "App")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	consumerPath := filepath.Join(appDir, "Consumer.swift")
	require.NoError(t, os.WriteFile(consumerPath, []byte(`
import Foundation

struct Consumer {
    func make() -> Int {
        struct Alpha {
            let value: Int
        }
        return Alpha(value: 1).value
    }
}
`), 0o644))

	alphaPath := filepath.Join(appDir, "Alpha.swift")
	require.NoError(t, os.WriteFile(alphaPath, []byte("import Foundation\n\nstruct Alpha {}\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		consumerPath: true,
		alphaPath:    true,
	}

	imports, err := ResolveSwiftProjectImports(consumerPath, consumerPath, supplied, reader)
	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestResolveSwiftProjectImports_NoEdgeForNestedTypeUsedUnqualified(t *testing.T) {
	// CLR-15 variant 2: a nested type referenced unqualified within its
	// enclosing type must resolve to the nested declaration, not to a
	// same-named top-level type in another file.
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "App")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	betaPath := filepath.Join(appDir, "Beta.swift")
	require.NoError(t, os.WriteFile(betaPath, []byte(`
import Foundation

struct Beta {
    struct Alpha {
        let value: Int
    }

    func make() -> Alpha {
        return Alpha(value: 1)
    }
}
`), 0o644))

	alphaPath := filepath.Join(appDir, "Alpha.swift")
	require.NoError(t, os.WriteFile(alphaPath, []byte("import Foundation\n\nstruct Alpha {}\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		betaPath:  true,
		alphaPath: true,
	}

	imports, err := ResolveSwiftProjectImports(betaPath, betaPath, supplied, reader)
	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestResolveSwiftProjectImports_NoEdgeForNestedDeclarationCollision(t *testing.T) {
	// CLR-15 variant 3: a file that only declares a nested type (and references
	// nothing external) must not get an edge to another file declaring a
	// same-named top-level type.
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "App")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	scopePath := filepath.Join(appDir, "Scope.swift")
	require.NoError(t, os.WriteFile(scopePath, []byte(`
import Foundation

struct Scope {
    struct Duplicated {
        let value: Int
    }
}
`), 0o644))

	duplicatedPath := filepath.Join(appDir, "Duplicated.swift")
	require.NoError(t, os.WriteFile(duplicatedPath, []byte("import Foundation\n\nstruct Duplicated {}\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		scopePath:      true,
		duplicatedPath: true,
	}

	imports, err := ResolveSwiftProjectImports(scopePath, scopePath, supplied, reader)
	require.NoError(t, err)
	assert.Empty(t, imports)
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

func TestResolveSwiftProjectImports_ExtensionMethodReference(t *testing.T) {
	// CLR-31: a file that calls a method defined only in an extension body in
	// another file must produce a file->file edge. Mirrors the Strata repro
	// where UserFlowParser.swift calls parseStatusBlockLine/openStatusBlock/
	// isStatusOpener defined in UserFlowParser+Resolve.swift (an extension-only
	// file), yet that file showed zero incoming edges.
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Sources", "StrataKit")
	require.NoError(t, os.MkdirAll(appDir, 0o755))

	extensionPath := filepath.Join(appDir, "Parser+Resolve.swift")
	require.NoError(t, os.WriteFile(extensionPath, []byte(`
import Foundation

extension Parser {
    mutating func parseStatusBlockLine(_ raw: String, line: Int) {}

    static func isStatusOpener(_ raw: String) -> Bool { return false }
}
`), 0o644))

	consumerPath := filepath.Join(appDir, "Parser.swift")
	require.NoError(t, os.WriteFile(consumerPath, []byte(`
import Foundation

struct Parser {
    mutating func run(_ raw: String, line: Int) {
        if Self.isStatusOpener(raw) {
            parseStatusBlockLine(raw, line: line)
        }
    }
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	supplied := map[string]bool{
		extensionPath: true,
		consumerPath:  true,
	}

	imports, err := ResolveSwiftProjectImports(consumerPath, consumerPath, supplied, reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{extensionPath}, imports)
}
