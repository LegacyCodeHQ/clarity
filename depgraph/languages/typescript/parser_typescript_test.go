package typescript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTypeScriptImports_ESMImports(t *testing.T) {
	source := `
import { foo, bar } from 'lodash';
import React from 'react';
import * as fs from 'fs';

const x = 1;
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	// Check import paths
	paths := extractPaths(imports)
	assert.Contains(t, paths, "lodash")
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "fs")

	// Verify classification
	assertImportType(t, imports, "lodash", ExternalImport{})
	assertImportType(t, imports, "react", ExternalImport{})
	assertImportType(t, imports, "fs", NodeBuiltinImport{})
}

func TestParseTypeScriptImports_DefaultImports(t *testing.T) {
	source := `
import React from 'react';
import express from 'express';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "express")
}

func TestParseTypeScriptImports_NamespaceImports(t *testing.T) {
	source := `
import * as fs from 'fs';
import * as path from 'path';
import * as lodash from 'lodash';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	// fs and path should be NodeBuiltinImport
	assertImportType(t, imports, "fs", NodeBuiltinImport{})
	assertImportType(t, imports, "path", NodeBuiltinImport{})
	// lodash should be ExternalImport
	assertImportType(t, imports, "lodash", ExternalImport{})
}

func TestParseTypeScriptImports_TypeOnlyImports(t *testing.T) {
	source := `
import type { User } from './models/user';
import type { Config } from 'config';
import { useState } from 'react';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	// Check type-only status
	for _, imp := range imports {
		if imp.Path() == "./models/user" || imp.Path() == "config" {
			assert.True(t, imp.IsTypeOnly(), "Expected %s to be type-only", imp.Path())
		} else if imp.Path() == "react" {
			assert.False(t, imp.IsTypeOnly(), "Expected react import to not be type-only")
		}
	}
}

func TestParseTypeScriptImports_SideEffectImports(t *testing.T) {
	source := `
import './styles.css';
import './polyfills';
import 'reflect-metadata';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "./styles.css")
	assert.Contains(t, paths, "./polyfills")
	assert.Contains(t, paths, "reflect-metadata")

	// First two should be InternalImport
	assertImportType(t, imports, "./styles.css", InternalImport{})
	assertImportType(t, imports, "./polyfills", InternalImport{})
	// Third should be ExternalImport
	assertImportType(t, imports, "reflect-metadata", ExternalImport{})
}

func TestParseTypeScriptImports_ReExports(t *testing.T) {
	source := `
export { foo, bar } from './utils';
export * from './helpers';
export { default as MyComponent } from './MyComponent';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "./utils")
	assert.Contains(t, paths, "./helpers")
	assert.Contains(t, paths, "./MyComponent")

	// All should be InternalImport
	for _, imp := range imports {
		_, ok := imp.(InternalImport)
		assert.True(t, ok, "Expected InternalImport for %s", imp.Path())
	}
}

func TestParseTypeScriptImports_NodeBuiltinsWithPrefix(t *testing.T) {
	source := `
import fs from 'node:fs';
import path from 'node:path';
import { createServer } from 'node:http';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	// All should be NodeBuiltinImport
	for _, imp := range imports {
		_, ok := imp.(NodeBuiltinImport)
		assert.True(t, ok, "Expected NodeBuiltinImport for %s", imp.Path())
	}
}

func TestParseTypeScriptImports_NodeBuiltinsWithoutPrefix(t *testing.T) {
	source := `
import fs from 'fs';
import path from 'path';
import { createServer } from 'http';
import crypto from 'crypto';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 4)

	// All should be NodeBuiltinImport
	for _, imp := range imports {
		_, ok := imp.(NodeBuiltinImport)
		assert.True(t, ok, "Expected NodeBuiltinImport for %s", imp.Path())
	}
}

func TestParseTypeScriptImports_RelativePaths(t *testing.T) {
	source := `
import { helper } from './utils/helper';
import { config } from '../config';
import { model } from './models/user';
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	// All should be InternalImport
	for _, imp := range imports {
		_, ok := imp.(InternalImport)
		assert.True(t, ok, "Expected InternalImport for %s", imp.Path())
	}

	paths := extractPaths(imports)
	assert.Contains(t, paths, "./utils/helper")
	assert.Contains(t, paths, "../config")
	assert.Contains(t, paths, "./models/user")
}

func TestParseTypeScriptImports_EmptyFile(t *testing.T) {
	source := ``
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestParseTypeScriptImports_NoImports(t *testing.T) {
	source := `
const x = 1;
const y = 2;

function add(a: number, b: number): number {
	return a + b;
}
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestParseTypeScriptImports_TSX(t *testing.T) {
	source := `
import React from 'react';
import { useState, useEffect } from 'react';
import { Button } from './components/Button';

const App: React.FC = () => {
	const [count, setCount] = useState(0);
	return <Button onClick={() => setCount(count + 1)}>Count: {count}</Button>;
};

export default App;
`
	imports, err := ParseTypeScriptImports([]byte(source), true)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "./components/Button")

	// Check types
	assertImportType(t, imports, "react", ExternalImport{})
	assertImportType(t, imports, "./components/Button", InternalImport{})
}

func TestParseTypeScriptImports_MixedQuotes(t *testing.T) {
	source := `
import foo from 'foo';
import bar from "bar";
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "foo")
	assert.Contains(t, paths, "bar")
}

func TestTypeScriptImports_FileNotFound(t *testing.T) {
	_, err := TypeScriptImports("/nonexistent/file/path.ts")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestTypeScriptImports_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.ts")

	content := `
import { foo } from './utils';
import React from 'react';
import fs from 'fs';

export const x = 1;
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	imports, err := TypeScriptImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "./utils")
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "fs")
}

func TestTypeScriptImports_TSXFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "App.tsx")

	content := `
import React from 'react';
import { Component } from './Component';

const App = () => <Component />;
export default App;
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	imports, err := TypeScriptImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 2)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "./Component")
}

func TestResolveTypeScriptImportPath_WithExtension(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/utils.ts":       true,
		"/project/src/helper.tsx":     true,
		"/project/src/index.ts":       true,
		"/project/src/lib/index.ts":   true,
		"/project/src/config/main.ts": true,
	}

	sourceFile := "/project/src/app.ts"

	// Test direct resolution
	resolved := ResolveTypeScriptImportPath(sourceFile, "./utils", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/utils.ts")

	// Test TSX resolution
	resolved = ResolveTypeScriptImportPath(sourceFile, "./helper", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/helper.tsx")

	// Test index file resolution
	resolved = ResolveTypeScriptImportPath(sourceFile, "./lib", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/lib/index.ts")

	// Test parent directory
	sourceFile = "/project/src/components/Button.tsx"
	resolved = ResolveTypeScriptImportPath(sourceFile, "../utils", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/utils.ts")
}

func TestResolveTypeScriptImportPath_NotFound(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/utils.ts": true,
	}

	sourceFile := "/project/src/app.ts"

	// Test non-existent file
	resolved := ResolveTypeScriptImportPath(sourceFile, "./nonexistent", suppliedFiles)
	assert.Empty(t, resolved)
}

func TestResolveTypeScriptImportPath_JSImportResolvesToTypeScriptSource(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/utils/cache.ts": true,
	}

	sourceFile := "/project/src/tools/finance/api.ts"

	resolved := ResolveTypeScriptImportPath(sourceFile, "../../utils/cache.js", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/utils/cache.ts")
}

func TestResolveTypeScriptImportPath_JSXImportResolvesToTSXSource(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/components/Button.tsx": true,
	}

	sourceFile := "/project/src/pages/Home.tsx"

	resolved := ResolveTypeScriptImportPath(sourceFile, "../components/Button.jsx", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/components/Button.tsx")
}

func TestResolveTypeScriptImportPath_AliasAtPrefixResolvesToSrc(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/components/file-tree-panel.tsx": true,
	}

	sourceFile := "/project/src/App.tsx"

	resolved := ResolveTypeScriptImportPath(sourceFile, "@/components/file-tree-panel", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/components/file-tree-panel.tsx")
}

func TestClassifyTypeScriptImport_NodeBuiltins(t *testing.T) {
	testCases := []struct {
		path     string
		expected string
	}{
		{"fs", "NodeBuiltinImport"},
		{"path", "NodeBuiltinImport"},
		{"http", "NodeBuiltinImport"},
		{"https", "NodeBuiltinImport"},
		{"crypto", "NodeBuiltinImport"},
		{"node:fs", "NodeBuiltinImport"},
		{"node:path", "NodeBuiltinImport"},
		{"fs/promises", "NodeBuiltinImport"},
	}

	for _, tc := range testCases {
		imp := classifyTypeScriptImport(tc.path, false)
		_, ok := imp.(NodeBuiltinImport)
		assert.True(t, ok, "Expected %s to be NodeBuiltinImport", tc.path)
	}
}

func TestClassifyTypeScriptImport_External(t *testing.T) {
	testCases := []string{
		"react",
		"lodash",
		"express",
		"@types/node",
		"@angular/core",
	}

	for _, path := range testCases {
		imp := classifyTypeScriptImport(path, false)
		_, ok := imp.(ExternalImport)
		assert.True(t, ok, "Expected %s to be ExternalImport", path)
	}
}

func TestClassifyTypeScriptImport_Internal(t *testing.T) {
	testCases := []string{
		"./utils",
		"../config",
		"./components/Button",
		"../../../lib/helper",
		"@/components/file-tree-panel",
	}

	for _, path := range testCases {
		imp := classifyTypeScriptImport(path, false)
		_, ok := imp.(InternalImport)
		assert.True(t, ok, "Expected %s to be InternalImport", path)
	}
}

func TestParseTypeScriptImports_ComplexExample(t *testing.T) {
	source := `
// External dependencies
import React, { useState, useEffect } from 'react';
import * as _ from 'lodash';
import express from 'express';

// Node.js builtins
import fs from 'fs';
import path from 'path';
import { createServer } from 'node:http';

// Type-only imports
import type { User } from './models/user';
import type { Config } from './config';

// Internal imports
import { helper } from './utils/helper';
import { formatDate } from '../lib/formatters';

// Side effect imports
import './styles.css';
import 'reflect-metadata';

// Re-exports
export { Button } from './components/Button';
export * from './constants';

const app = express();
`
	imports, err := ParseTypeScriptImports([]byte(source), false)

	require.NoError(t, err)

	paths := extractPaths(imports)

	// External
	assert.Contains(t, paths, "react")
	assert.Contains(t, paths, "lodash")
	assert.Contains(t, paths, "express")
	assert.Contains(t, paths, "reflect-metadata")

	// Node.js builtins
	assert.Contains(t, paths, "fs")
	assert.Contains(t, paths, "path")
	assert.Contains(t, paths, "node:http")

	// Internal
	assert.Contains(t, paths, "./models/user")
	assert.Contains(t, paths, "./config")
	assert.Contains(t, paths, "./utils/helper")
	assert.Contains(t, paths, "../lib/formatters")
	assert.Contains(t, paths, "./styles.css")
	assert.Contains(t, paths, "./components/Button")
	assert.Contains(t, paths, "./constants")
}

// Helper functions

func extractPaths(imports []TypeScriptImport) []string {
	paths := make([]string, len(imports))
	for i, imp := range imports {
		paths[i] = imp.Path()
	}
	return paths
}

func assertImportType(t *testing.T, imports []TypeScriptImport, path string, expectedType TypeScriptImport) {
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
