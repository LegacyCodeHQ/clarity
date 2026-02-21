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

func TestResolveRustProjectImports_UsePathThroughModRs(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	fooDir := filepath.Join(srcDir, "foo")
	require.NoError(t, os.MkdirAll(fooDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	libFile := filepath.Join(srcDir, "lib.rs")
	modFile := filepath.Join(fooDir, "mod.rs")
	barFile := filepath.Join(fooDir, "bar.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("use crate::foo::Baz;\n"), 0644))
	require.NoError(t, os.WriteFile(modFile, []byte("pub mod bar;\npub use bar::Baz;\n"), 0644))
	require.NoError(t, os.WriteFile(barFile, []byte("pub struct Baz;\n"), 0644))

	supplied := map[string]bool{
		cargoToml: true,
		libFile:   true,
		modFile:   true,
		barFile:   true,
	}

	imports, err := ResolveRustProjectImports(libFile, libFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, barFile)
	assert.NotContains(t, imports, modFile)
}

func TestResolveRustProjectImports_UsePathDoesNotExpandParentMod(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	coreDir := filepath.Join(srcDir, "core")
	typesDir := filepath.Join(coreDir, "types")
	require.NoError(t, os.MkdirAll(typesDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	libFile := filepath.Join(srcDir, "lib.rs")
	coreMod := filepath.Join(coreDir, "mod.rs")
	typesMod := filepath.Join(typesDir, "mod.rs")
	constraintsFile := filepath.Join(typesDir, "constraints.rs")
	entityFile := filepath.Join(typesDir, "entity.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("use crate::core::types::constraints;\n"), 0644))
	require.NoError(t, os.WriteFile(coreMod, []byte("pub mod types;\n"), 0644))
	require.NoError(t, os.WriteFile(typesMod, []byte("pub mod constraints;\npub mod entity;\n"), 0644))
	require.NoError(t, os.WriteFile(constraintsFile, []byte("pub struct Constraints;\n"), 0644))
	require.NoError(t, os.WriteFile(entityFile, []byte("pub struct Entity;\n"), 0644))

	supplied := map[string]bool{
		cargoToml:       true,
		libFile:         true,
		coreMod:         true,
		typesMod:        true,
		constraintsFile: true,
		entityFile:      true,
	}

	imports, err := ResolveRustProjectImports(libFile, libFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, constraintsFile)
	assert.NotContains(t, imports, entityFile)
	assert.NotContains(t, imports, typesMod)
}

func TestResolveRustProjectImports_DoesNotReturnSelfDependency(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	engineDir := filepath.Join(srcDir, "engine")
	require.NoError(t, os.MkdirAll(engineDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	libFile := filepath.Join(srcDir, "lib.rs")
	engineMod := filepath.Join(engineDir, "mod.rs")
	astgrepFile := filepath.Join(engineDir, "astgrep.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("pub mod engine;\n"), 0644))
	require.NoError(t, os.WriteFile(engineMod, []byte("pub mod astgrep;\n"), 0644))
	require.NoError(t, os.WriteFile(astgrepFile, []byte("use crate::engine::astgrep::AstGrepEngine;\n"), 0644))

	supplied := map[string]bool{
		cargoToml:   true,
		libFile:     true,
		engineMod:   true,
		astgrepFile: true,
	}

	imports, err := ResolveRustProjectImports(astgrepFile, astgrepFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.NotContains(t, imports, astgrepFile)
}
