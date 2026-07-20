package rust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func importKey(path string, kind RustImportKind) RustImport {
	return RustImport{Path: path, Kind: kind}
}

func TestParseRustImports(t *testing.T) {
	source := `
use std::io;
use crate::utils::helper as h;
extern crate serde;
mod nested;
`
	imports, err := ParseRustImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 4)
	assert.Equal(t, "std::io", imports[0].Path)
	assert.Equal(t, RustImportUse, imports[0].Kind)
	assert.Equal(t, "crate::utils::helper", imports[1].Path)
	assert.Equal(t, RustImportUse, imports[1].Kind)
	assert.Equal(t, "serde", imports[2].Path)
	assert.Equal(t, RustImportExternCrate, imports[2].Kind)
	assert.Equal(t, "nested", imports[3].Path)
	assert.Equal(t, RustImportModDecl, imports[3].Kind)
}

func TestRustImports_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "lib.rs")

	content := `
use std::fmt;
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	imports, err := RustImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 1)
	assert.Equal(t, "std::fmt", imports[0].Path)
	assert.Equal(t, RustImportUse, imports[0].Kind)
}

func TestParseRustImports_CapturesImportsInFunctionBodies(t *testing.T) {
	source := `
use crate::top::level;

fn helper() {
  use crate::nested::only;
}

mod nested;
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	assert.Equal(t, []RustImport{
		{Path: "crate::top::level", Kind: RustImportUse},
		{Path: "crate::nested::only", Kind: RustImportUse, Nested: true},
		{Path: "nested", Kind: RustImportModDecl},
	}, imports)
}

// Regression for CLR-22: `use crate::X` inside an inner `mod` block, where the
// dependency is then reached only through the alias the import binds, produced
// no import at all.
func TestParseRustImports_CapturesImportsInNestedModBlocks(t *testing.T) {
	source := `
use crate::db;

pub mod status {
    use crate::daily::{self, Day, IssueSummary};

    pub async fn run() {
        let activity = daily::load().await;
    }
}
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	assert.Equal(t, []RustImport{
		{Path: "crate::db", Kind: RustImportUse},
		{Path: "crate::daily", Kind: RustImportUse, Nested: true},
		{Path: "crate::daily::Day", Kind: RustImportUse, Nested: true},
		{Path: "crate::daily::IssueSummary", Kind: RustImportUse, Nested: true},
		// From the separate qualified-path pass, not the `use` scanner.
		{Path: "daily::load", Kind: RustImportUse},
	}, imports)
}

// `self::` and `super::` are relative to the enclosing module, so inside an
// inner `mod` block they mean something different than they do at file scope —
// `super::` there is the file itself, not its parent. Promoting them would
// invent edges, so nested relative imports are dropped.
func TestParseRustImports_DropsNestedRelativeImports(t *testing.T) {
	source := `
use crate::db;

pub mod status {
    use super::*;
    use super::Helper;
    use self::inner::Thing;
    use crate::daily::Day;
}
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	assert.Equal(t, []RustImport{
		{Path: "crate::db", Kind: RustImportUse},
		{Path: "crate::daily::Day", Kind: RustImportUse, Nested: true},
	}, imports)
}

// `mod foo;` inside an inner `mod` block resolves relative to that inner
// module's directory, not the file's, so it is not a file-level mod decl.
func TestParseRustImports_DropsNestedModDeclarations(t *testing.T) {
	source := `
mod top;

pub mod outer {
    mod inner;
}
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	assert.Equal(t, []RustImport{
		{Path: "top", Kind: RustImportModDecl},
	}, imports)
}

func TestParseRustImports_VisibilityAndScopedUseList(t *testing.T) {
	source := `
#[cfg(feature = "x")]
pub(crate) use crate::alpha::{beta, gamma};
pub mod public_mod;
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	// Brace groups expand to one import per symbol — the dependency resolver
	// needs every name to reach its defining file.
	assert.Len(t, imports, 3)
	assert.Equal(t, "crate::alpha::beta", imports[0].Path)
	assert.Equal(t, RustImportUse, imports[0].Kind)
	assert.Equal(t, "crate::alpha::gamma", imports[1].Path)
	assert.Equal(t, RustImportUse, imports[1].Kind)
	assert.Equal(t, "public_mod", imports[2].Path)
	assert.Equal(t, RustImportModDecl, imports[2].Kind)
}

// A single-level brace group on a `use super::` import — exactly the spear
// `real_git.rs` pattern: `use super::{Git, Submodule};`. Each symbol must
// become its own import path so the resolver can follow it through the
// parent module's `pub use` re-exports.
func TestParseRustImports_BraceGroupExpandsToOnePathPerSymbol(t *testing.T) {
	source := `use super::{Git, Submodule};
use super::Other;
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	paths := make([]string, 0, len(imports))
	for _, imp := range imports {
		if imp.Kind == RustImportUse {
			paths = append(paths, imp.Path)
		}
	}
	assert.Contains(t, paths, "super::Git", "expected braced symbol Git to become super::Git")
	assert.Contains(t, paths, "super::Submodule", "expected braced symbol Submodule to become super::Submodule")
	assert.Contains(t, paths, "super::Other", "expected non-braced super::Other to be preserved")
	assert.NotContains(t, paths, "super", "the bare prefix should not leak into imports when symbols are present")
}

// Nested brace groups should also expand fully. `as` aliases are stripped
// (we only care about the source identifier path), and a literal `self`
// inside a group refers to the prefix itself.
func TestParseRustImports_NestedBraceGroupAndSelfAndAs(t *testing.T) {
	source := `use std::{io::{self, Read}, fs::File as F};`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	paths := make([]string, 0, len(imports))
	for _, imp := range imports {
		if imp.Kind == RustImportUse {
			paths = append(paths, imp.Path)
		}
	}
	assert.Contains(t, paths, "std::io", "self inside std::io group should reference the io prefix itself")
	assert.Contains(t, paths, "std::io::Read")
	assert.Contains(t, paths, "std::fs::File", "as alias should be stripped, leaving the underlying path")
}

func TestParseRustImports_CollectsQualifiedPathReferences(t *testing.T) {
	source := `
use crate::alpha::beta;

fn run() {
  crate_b::foo::run();
  crate::core::do_work();
  let x = "crate_b::ignored::in_string";
  // crate_b::ignored::in_comment
}
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	assert.Contains(t, imports, importKey("crate_b::foo::run", RustImportUse))
	assert.Contains(t, imports, importKey("crate::core::do_work", RustImportUse))
	assert.Contains(t, imports, importKey("crate::alpha::beta", RustImportUse))
}

func TestParseRustImports_CollectsQualifiedPathsWhenLifetimesPresent(t *testing.T) {
	source := `
const fn marker() -> &'static str { "ok" }

fn run() {
  s8_parser::analyze();
  s8_flow::build_flow_graph();
}
`
	imports, err := ParseRustImports([]byte(source))
	require.NoError(t, err)

	assert.Contains(t, imports, importKey("s8_parser::analyze", RustImportUse))
	assert.Contains(t, imports, importKey("s8_flow::build_flow_graph", RustImportUse))
}
