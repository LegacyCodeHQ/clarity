package rust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRustProjectImports_ModDecl(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	libFile := filepath.Join(srcDir, "lib.rs")
	fooFile := filepath.Join(srcDir, "foo.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("mod foo;\n"), 0644))
	require.NoError(t, os.WriteFile(fooFile, []byte("pub fn bar() {}\n"), 0644))

	supplied := map[string]bool{
		cargoToml: true,
		libFile:   true,
		fooFile:   true,
	}

	imports, err := ResolveRustProjectImports(libFile, libFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, fooFile)
}

func TestResolveRustProjectImports_UseCratePath(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	libFile := filepath.Join(srcDir, "lib.rs")
	fooFile := filepath.Join(srcDir, "foo.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("use crate::foo::bar;\n"), 0644))
	require.NoError(t, os.WriteFile(fooFile, []byte("pub fn bar() {}\n"), 0644))

	supplied := map[string]bool{
		cargoToml: true,
		libFile:   true,
		fooFile:   true,
	}

	imports, err := ResolveRustProjectImports(libFile, libFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, fooFile)
}

func TestResolveRustProjectImports_UseCratePathWithoutSuppliedCargoToml(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	libFile := filepath.Join(srcDir, "lib.rs")
	fooFile := filepath.Join(srcDir, "foo.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("use crate::foo::bar;\n"), 0644))
	require.NoError(t, os.WriteFile(fooFile, []byte("pub fn bar() {}\n"), 0644))

	supplied := map[string]bool{
		libFile: true,
		fooFile: true,
	}

	imports, err := ResolveRustProjectImports(libFile, libFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, fooFile)
}

func TestResolveRustProjectImports_UseLocalCrateNamePathResolvesToLib(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "app-server")
	srcDir := filepath.Join(crateRoot, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	mainFile := filepath.Join(srcDir, "main.rs")
	libFile := filepath.Join(srcDir, "lib.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"codex-app-server\"\n[lib]\nname = \"codex_app_server\"\n"), 0644))
	require.NoError(t, os.WriteFile(mainFile, []byte("use codex_app_server::run_main_with_transport;\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("pub fn run_main_with_transport() {}\n"), 0644))

	supplied := map[string]bool{
		cargoToml: true,
		mainFile:  true,
		libFile:   true,
	}

	imports, err := ResolveRustProjectImports(mainFile, mainFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, libFile)
}
