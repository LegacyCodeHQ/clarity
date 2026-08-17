package python

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePythonImports_ImportStatements(t *testing.T) {
	source := `
import os
import sys as system
import pkg.module
`
	imports, err := ParsePythonImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "os")
	assert.Contains(t, paths, "sys")
	assert.Contains(t, paths, "pkg.module")
}

func TestParsePythonImports_ImportFromStatements(t *testing.T) {
	source := `
from collections import defaultdict
from . import helpers
from ..utils import slugify
from .pkg import api
`
	imports, err := ParsePythonImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 4)

	// Path() is the submodule-shaped candidate to try first.
	paths := extractPaths(imports)
	assert.Contains(t, paths, "collections.defaultdict")
	assert.Contains(t, paths, ".helpers")
	assert.Contains(t, paths, "..utils.slugify")
	assert.Contains(t, paths, ".pkg.api")

	// FallbackPath() is the bare module, tried only if Path() doesn't
	// resolve to a real file (the imported name is an attribute, not a
	// submodule).
	fallbacks := extractFallbacks(imports)
	assert.Contains(t, fallbacks, "collections")
	assert.Contains(t, fallbacks, ".")
	assert.Contains(t, fallbacks, "..utils")
	assert.Contains(t, fallbacks, ".pkg")
}

func TestParsePythonImports_BareRelativeImportNamesEachTarget(t *testing.T) {
	// `from . import x, y` names its real targets in the import list, not in
	// the dots. Each name must resolve to its own sibling file, not collapse
	// onto a single "." edge that points at the package's own __init__.py.
	source := `
from . import packages, utils as u
from .. import foo
`
	imports, err := ParsePythonImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, ".packages")
	assert.Contains(t, paths, ".utils")
	assert.Contains(t, paths, "..foo")
	assert.NotContains(t, paths, ".")
}

func TestParsePythonImports_BareRelativeImportParenthesizedList(t *testing.T) {
	source := `
from . import (
    packages,
    utils,
)
`
	imports, err := ParsePythonImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, ".packages")
	assert.Contains(t, paths, ".utils")
}

func TestParsePythonImports_BareRelativeWildcardImportUnchanged(t *testing.T) {
	// `from . import *` pulls from the package's own namespace -- the dots
	// already resolve to that. Not the bug this covers; must stay a no-op.
	source := `
from . import *
`
	imports, err := ParsePythonImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 1)

	paths := extractPaths(imports)
	assert.Contains(t, paths, ".")
}

func TestPythonImports_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "app.py")

	content := `
import json
from . import helpers
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	imports, err := PythonImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "json")
	assert.Contains(t, paths, ".helpers")
}

func TestResolvePythonImportPath(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/pkg/__init__.py":           true,
		"/project/pkg/utils.py":              true,
		"/project/pkg/sub/__init__.py":       true,
		"/project/pkg/sub/helpers.py":        true,
		"/project/pkg/sub/tools/__init__.py": true,
	}

	sourceFile := "/project/pkg/sub/app.py"

	resolved := ResolvePythonImportPath(sourceFile, ".", suppliedFiles)
	assert.Contains(t, resolved, "/project/pkg/sub/__init__.py")

	resolved = ResolvePythonImportPath(sourceFile, "..utils", suppliedFiles)
	assert.Contains(t, resolved, "/project/pkg/utils.py")

	resolved = ResolvePythonImportPath(sourceFile, ".helpers", suppliedFiles)
	assert.Contains(t, resolved, "/project/pkg/sub/helpers.py")

	resolved = ResolvePythonImportPath(sourceFile, ".tools", suppliedFiles)
	assert.Contains(t, resolved, "/project/pkg/sub/tools/__init__.py")
}

func TestResolvePythonAbsoluteImportPath(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/legacy/src/dexter/model.py":                   true,
		"/project/legacy/src/dexter/tools/__init__.py":          true,
		"/project/legacy/src/dexter/tools/finance/api.py":       true,
		"/project/legacy/src/dexter/utils/logger.py":            true,
		"/project/legacy/src/other/external_lookalike/model.py": true,
	}

	sourcePath := "/project/legacy/src/dexter/caller.py"

	resolved := ResolvePythonAbsoluteImportPath(sourcePath, "dexter.model", suppliedFiles)
	assert.Equal(t, []string{"/project/legacy/src/dexter/model.py"}, resolved)

	resolved = ResolvePythonAbsoluteImportPath(sourcePath, "dexter.tools", suppliedFiles)
	assert.Equal(t, []string{"/project/legacy/src/dexter/tools/__init__.py"}, resolved)

	resolved = ResolvePythonAbsoluteImportPath(sourcePath, "dexter.tools.finance.api", suppliedFiles)
	assert.Equal(t, []string{"/project/legacy/src/dexter/tools/finance/api.py"}, resolved)
}

// Helper functions

func extractPaths(imports []PythonImport) []string {
	paths := make([]string, len(imports))
	for i, imp := range imports {
		paths[i] = imp.Path()
	}
	return paths
}

func extractFallbacks(imports []PythonImport) []string {
	fallbacks := make([]string, len(imports))
	for i, imp := range imports {
		fallbacks[i] = imp.FallbackPath()
	}
	return fallbacks
}
