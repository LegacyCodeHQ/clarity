package javascript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
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

func TestParseJavaScriptImports_CommonJSRequire(t *testing.T) {
	source := `
const fs = require('fs');
const express = require('express');
const utils = require('./utils');
`
	imports, err := ParseJavaScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "fs")
	assert.Contains(t, paths, "express")
	assert.Contains(t, paths, "./utils")

	assertImportType(t, imports, "fs", NodeBuiltinImport{})
	assertImportType(t, imports, "express", ExternalImport{})
	assertImportType(t, imports, "./utils", InternalImport{})
}

func TestParseJavaScriptImports_MixedESMAndRequire(t *testing.T) {
	source := `
import path from 'path';
const utils = require('./utils');
`
	imports, err := ParseJavaScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "path")
	assert.Contains(t, paths, "./utils")
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

func TestResolveJavaScriptImportPath_MJS(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/viewer_state.mjs":  true,
		"/project/src/feature/index.mjs": true,
	}

	sourceFile := "/project/src/viewer.js"

	resolved := ResolveJavaScriptImportPath(sourceFile, "./viewer_state.mjs", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/viewer_state.mjs")

	resolved = ResolveJavaScriptImportPath(sourceFile, "./feature", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/feature/index.mjs")
}

func TestResolveJavaScriptImportPath_CJS(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/legacy_state.cjs": true,
		"/project/src/legacy/index.cjs": true,
	}

	sourceFile := "/project/src/viewer.js"

	resolved := ResolveJavaScriptImportPath(sourceFile, "./legacy_state.cjs", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/legacy_state.cjs")

	resolved = ResolveJavaScriptImportPath(sourceFile, "./legacy", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/legacy/index.cjs")
}

func TestResolveJavaScriptImportPath_ExplicitNonJSExtension(t *testing.T) {
	// Side-effect imports like `import './storybook.css'` from .jsx files must
	// resolve to the CSS file when it exists in suppliedFiles. Mirrors the
	// TypeScript resolver's behaviour for .css/.json side-effect imports.
	suppliedFiles := map[string]bool{
		"/project/.storybook/storybook.css": true,
		"/project/src/data.json":            true,
	}

	sourceFile := "/project/.storybook/preview.jsx"

	resolved := ResolveJavaScriptImportPath(sourceFile, "./storybook.css", suppliedFiles)
	assert.Contains(t, resolved, "/project/.storybook/storybook.css")

	resolved = ResolveJavaScriptImportPath("/project/src/app.js", "./data.json", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/data.json")
}

func TestJavaScriptImports_MJSFile(t *testing.T) {
	filePath := filepath.Join("testdata", "sample.test.mjs")
	imports, err := JavaScriptImports(filePath)
	require.NoError(t, err)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "./sample.mjs")
}

func TestResolveJavaScriptProjectImports_MJSFile(t *testing.T) {
	testFile, err := filepath.Abs(filepath.Join("testdata", "sample.test.mjs"))
	require.NoError(t, err)
	sampleFile, err := filepath.Abs(filepath.Join("testdata", "sample.mjs"))
	require.NoError(t, err)

	resolved, err := ResolveJavaScriptProjectImports(testFile, testFile, ".mjs", map[string]bool{
		testFile:   true,
		sampleFile: true,
	}, vcs.FilesystemContentReader())
	require.NoError(t, err)
	assert.Contains(t, resolved, sampleFile)
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
