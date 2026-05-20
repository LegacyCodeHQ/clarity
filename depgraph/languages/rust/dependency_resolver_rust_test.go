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

// TestResolveRustProjectImports_WorkspaceTrueDependency exercises the modern
// Cargo-workspace dependency pattern, where the path lives in the root
// workspace's `[workspace.dependencies]` and each member crate refers to it
// via `{ workspace = true }`. This is the pattern uv, ruff, deno, and most
// large Rust workspaces use; the failure mode is that every cross-crate
// edge becomes invisible to clarity because `parseRustPathDependencyEntries`
// only catches direct `path = ...` declarations on the importing crate.
//
// Layout under tmpDir/workspace:
//
//	Cargo.toml                    [workspace] members + [workspace.dependencies]
//	crate-a/
//	  Cargo.toml                  [dependencies] crate-b = { workspace = true }
//	  src/main.rs                 use crate_b::foo::run;
//	crate-b/
//	  Cargo.toml                  [package] name = "crate-b"
//	  src/lib.rs                  pub mod foo;
//	  src/foo.rs                  pub fn run() {}
func TestResolveRustProjectImports_WorkspaceTrueDependency(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	crateADir := filepath.Join(workspaceRoot, "crate-a")
	crateBDir := filepath.Join(workspaceRoot, "crate-b")
	require.NoError(t, os.MkdirAll(filepath.Join(crateADir, "src"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(crateBDir, "src"), 0755))

	workspaceCargo := filepath.Join(workspaceRoot, "Cargo.toml")
	crateACargo := filepath.Join(crateADir, "Cargo.toml")
	crateAMain := filepath.Join(crateADir, "src", "main.rs")
	crateBCargo := filepath.Join(crateBDir, "Cargo.toml")
	crateBLib := filepath.Join(crateBDir, "src", "lib.rs")
	crateBFoo := filepath.Join(crateBDir, "src", "foo.rs")

	require.NoError(t, os.WriteFile(workspaceCargo, []byte(`
[workspace]
members = ["crate-a", "crate-b"]
resolver = "2"

[workspace.dependencies]
crate-b = { version = "0.1.0", path = "crate-b" }
`), 0644))

	require.NoError(t, os.WriteFile(crateACargo, []byte(`
[package]
name = "crate-a"
version = "0.1.0"

[dependencies]
crate-b = { workspace = true }
`), 0644))
	require.NoError(t, os.WriteFile(crateAMain, []byte(`
use crate_b::foo::run;

fn main() {
    run();
}
`), 0644))

	require.NoError(t, os.WriteFile(crateBCargo, []byte(`
[package]
name = "crate-b"
version = "0.1.0"
`), 0644))
	require.NoError(t, os.WriteFile(crateBLib, []byte("pub mod foo;\n"), 0644))
	require.NoError(t, os.WriteFile(crateBFoo, []byte("pub fn run() {}\n"), 0644))

	supplied := map[string]bool{
		workspaceCargo: true,
		crateACargo:    true,
		crateAMain:     true,
		crateBCargo:    true,
		crateBLib:      true,
		crateBFoo:      true,
	}

	imports, err := ResolveRustProjectImports(crateAMain, crateAMain, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, crateBFoo, "expected crate_b::foo::run to resolve via workspace = true delegation")
}

func TestResolveRustProjectImports_CrossCratePathDependency(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	crateADir := filepath.Join(workspaceRoot, "crate-a")
	crateBDir := filepath.Join(workspaceRoot, "crate-b")
	crateASrc := filepath.Join(crateADir, "src")
	crateBSrc := filepath.Join(crateBDir, "src")
	require.NoError(t, os.MkdirAll(crateASrc, 0755))
	require.NoError(t, os.MkdirAll(crateBSrc, 0755))

	crateACargo := filepath.Join(crateADir, "Cargo.toml")
	crateAMain := filepath.Join(crateASrc, "main.rs")
	crateBCargo := filepath.Join(crateBDir, "Cargo.toml")
	crateBLib := filepath.Join(crateBSrc, "lib.rs")
	crateBFoo := filepath.Join(crateBSrc, "foo.rs")

	require.NoError(t, os.WriteFile(crateACargo, []byte(`
[package]
name = "crate-a"
version = "0.1.0"

[dependencies]
crate-b = { path = "../crate-b" }
`), 0644))
	require.NoError(t, os.WriteFile(crateAMain, []byte(`
use crate_b::foo::run;

fn main() {
    run();
}
`), 0644))

	require.NoError(t, os.WriteFile(crateBCargo, []byte(`
[package]
name = "crate-b"
version = "0.1.0"
`), 0644))
	require.NoError(t, os.WriteFile(crateBLib, []byte("pub mod foo;\n"), 0644))
	require.NoError(t, os.WriteFile(crateBFoo, []byte("pub fn run() {}\n"), 0644))

	supplied := map[string]bool{
		crateACargo: true,
		crateAMain:  true,
		crateBCargo: true,
		crateBLib:   true,
		crateBFoo:   true,
	}

	imports, err := ResolveRustProjectImports(crateAMain, crateAMain, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, crateBFoo)
}

// In Rust 2018+, a parent-module file like `src/app.rs` can declare submodules
// via `mod child;` and re-export their items via `pub use child::item;`. The
// path `child::item` is interpreted relative to the current module, resolving
// to `src/app/child.rs`. The resolver must follow this — both for `pub use`
// and plain `use`.
func TestResolveRustProjectImports_UseSiblingSubmoduleFromNonModFile(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	appDir := filepath.Join(srcDir, "app")
	require.NoError(t, os.MkdirAll(appDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	mainFile := filepath.Join(srcDir, "main.rs")
	appFile := filepath.Join(srcDir, "app.rs")
	childFile := filepath.Join(appDir, "clarity_desktop.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(mainFile, []byte("mod app;\nfn main() { app::run_app(); }\n"), 0644))
	require.NoError(t, os.WriteFile(appFile, []byte("mod clarity_desktop;\n\npub use clarity_desktop::run_app;\n"), 0644))
	require.NoError(t, os.WriteFile(childFile, []byte("pub fn run_app() {}\n"), 0644))

	supplied := map[string]bool{
		cargoToml: true,
		mainFile:  true,
		appFile:   true,
		childFile: true,
	}

	imports, err := ResolveRustProjectImports(appFile, appFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, childFile, "pub use clarity_desktop::run_app from app.rs should resolve to app/clarity_desktop.rs")
}

func TestResolveRustProjectImports_UseSuperFromNonModFile(t *testing.T) {
	tmpDir := t.TempDir()
	crateRoot := filepath.Join(tmpDir, "mycrate")
	srcDir := filepath.Join(crateRoot, "src")
	fsDir := filepath.Join(srcDir, "fs")
	require.NoError(t, os.MkdirAll(fsDir, 0755))

	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	libFile := filepath.Join(srcDir, "lib.rs")
	fsMod := filepath.Join(fsDir, "mod.rs")
	traitFsFile := filepath.Join(fsDir, "trait_fs.rs")
	realFsFile := filepath.Join(fsDir, "real_fs.rs")

	require.NoError(t, os.WriteFile(cargoToml, []byte("[package]\nname = \"mycrate\"\n"), 0644))
	require.NoError(t, os.WriteFile(libFile, []byte("mod fs;\n"), 0644))
	require.NoError(t, os.WriteFile(fsMod, []byte("mod trait_fs;\nmod real_fs;\n"), 0644))
	require.NoError(t, os.WriteFile(traitFsFile, []byte("pub trait Fs {}\n"), 0644))
	require.NoError(t, os.WriteFile(realFsFile, []byte("use super::trait_fs::Fs;\npub struct RealFs;\nimpl Fs for RealFs {}\n"), 0644))

	supplied := map[string]bool{
		cargoToml:   true,
		libFile:     true,
		fsMod:       true,
		traitFsFile: true,
		realFsFile:  true,
	}

	imports, err := ResolveRustProjectImports(realFsFile, realFsFile, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, traitFsFile, "use super::trait_fs::Fs from real_fs.rs should resolve to trait_fs.rs")
}

func TestResolveRustProjectImports_CrossCratePathDependency_QualifiedCallWithoutUse(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	crateADir := filepath.Join(workspaceRoot, "crate-a")
	crateBDir := filepath.Join(workspaceRoot, "crate-b")
	crateASrc := filepath.Join(crateADir, "src")
	crateBSrc := filepath.Join(crateBDir, "src")
	require.NoError(t, os.MkdirAll(crateASrc, 0755))
	require.NoError(t, os.MkdirAll(crateBSrc, 0755))

	crateACargo := filepath.Join(crateADir, "Cargo.toml")
	crateAMain := filepath.Join(crateASrc, "main.rs")
	crateBCargo := filepath.Join(crateBDir, "Cargo.toml")
	crateBLib := filepath.Join(crateBSrc, "lib.rs")
	crateBFoo := filepath.Join(crateBSrc, "foo.rs")

	require.NoError(t, os.WriteFile(crateACargo, []byte(`
[package]
name = "crate-a"
version = "0.1.0"

[dependencies]
crate-b = { path = "../crate-b" }
`), 0644))
	require.NoError(t, os.WriteFile(crateAMain, []byte(`
fn main() {
    crate_b::foo::run();
}
`), 0644))

	require.NoError(t, os.WriteFile(crateBCargo, []byte(`
[package]
name = "crate-b"
version = "0.1.0"
`), 0644))
	require.NoError(t, os.WriteFile(crateBLib, []byte("pub mod foo;\n"), 0644))
	require.NoError(t, os.WriteFile(crateBFoo, []byte("pub fn run() {}\n"), 0644))

	supplied := map[string]bool{
		crateACargo: true,
		crateAMain:  true,
		crateBCargo: true,
		crateBLib:   true,
		crateBFoo:   true,
	}

	imports, err := ResolveRustProjectImports(crateAMain, crateAMain, supplied, os.ReadFile)
	require.NoError(t, err)
	assert.Contains(t, imports, crateBFoo)
}
