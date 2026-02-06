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

	paths := extractPaths(imports)
	assert.Contains(t, paths, "collections")
	assert.Contains(t, paths, ".")
	assert.Contains(t, paths, "..utils")
	assert.Contains(t, paths, ".pkg")
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
	assert.Contains(t, paths, ".")
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

// Helper functions

func extractPaths(imports []PythonImport) []string {
	paths := make([]string, len(imports))
	for i, imp := range imports {
		paths[i] = imp.Path()
	}
	return paths
}
