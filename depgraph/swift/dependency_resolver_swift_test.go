package swift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/sanity/vcs"
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
