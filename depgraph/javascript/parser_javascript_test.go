package javascript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJavaScriptImports_Basic(t *testing.T) {
	source := `
import React from 'react';
import { useState } from 'react';
import { Button } from './components/Button';
import fs from 'fs';
`
	imports, err := ParseJavaScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 4)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "./components/Button")
	assert.Contains(t, paths, "fs")

	assertImportType(t, imports, "fs", NodeBuiltinImport{})
	assertImportType(t, imports, "./components/Button", InternalImport{})
}

func TestParseJavaScriptImports_JSX(t *testing.T) {
	source := `
import React from 'react';
import { Button } from './components/Button';

export default function App() {
	return <Button />;
}
`
	imports, err := ParseJavaScriptImports([]byte(source), true)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "./components/Button")
}

func TestJavaScriptImports_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsx")

	content := `
import { foo } from './utils';
import React from 'react';
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	imports, err := JavaScriptImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "./utils")
	assert.Contains(t, paths, "react")
}

func TestResolveJavaScriptImportPath(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/utils.js":     true,
		"/project/src/helper.jsx":   true,
		"/project/src/lib/index.js": true,
	}

	sourceFile := "/project/src/app.js"

	resolved := ResolveJavaScriptImportPath(sourceFile, "./utils", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/utils.js")

	resolved = ResolveJavaScriptImportPath(sourceFile, "./helper", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/helper.jsx")

	resolved = ResolveJavaScriptImportPath(sourceFile, "./lib", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/lib/index.js")
}

// Helper functions

func extractPaths(imports []JavaScriptImport) []string {
	paths := make([]string, len(imports))
	for i, imp := range imports {
		paths[i] = imp.Path()
	}
	return paths
}

func assertImportType(t *testing.T, imports []JavaScriptImport, path string, expectedType JavaScriptImport) {
	t.Helper()
	for _, imp := range imports {
		if imp.Path() == path {
			switch expectedType.(type) {
			case NodeBuiltinImport:
				_, ok := imp.(NodeBuiltinImport)
				assert.True(t, ok, "Expected %s to be NodeBuiltinImport, got %T", path, imp)
			case ExternalImport:
				_, ok := imp.(ExternalImport)
				assert.True(t, ok, "Expected %s to be ExternalImport, got %T", path, imp)
			case InternalImport:
				_, ok := imp.(InternalImport)
				assert.True(t, ok, "Expected %s to be InternalImport, got %T", path, imp)
			}
			return
		}
	}
	t.Errorf("Import with path %s not found", path)
}
