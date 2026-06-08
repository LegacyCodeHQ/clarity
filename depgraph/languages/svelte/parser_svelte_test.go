package svelte

import (
	"fmt"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph/languages/javascript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSvelteImports_Basic(t *testing.T) {
	source := `
<script>
	import { onMount } from 'svelte';
	import { count } from './stores';
	import Button from './components/Button.svelte';
</script>

<h1>Hello</h1>
`
	imports, err := ParseSvelteImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "svelte")
	assert.Contains(t, paths, "./stores")
	assert.Contains(t, paths, "./components/Button.svelte")
}

func TestParseSvelteImports_ModuleScript(t *testing.T) {
	source := `
<script context="module">
	import { API_URL } from './config';
</script>

<script>
	import { onMount } from 'svelte';
	import { fetchData } from './api';
</script>

<p>Content</p>
`
	imports, err := ParseSvelteImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	paths := extractPaths(imports)
	assert.Contains(t, paths, "./config")
	assert.Contains(t, paths, "svelte")
	assert.Contains(t, paths, "./api")
}

func TestParseSvelteImports_NoScript(t *testing.T) {
	source := `
<h1>Hello</h1>
<p>No script tag here</p>
`
	imports, err := ParseSvelteImports([]byte(source))

	require.NoError(t, err)
	assert.Empty(t, imports)
}

func TestParseSvelteImports_ImportClassification(t *testing.T) {
	source := `
<script>
	import fs from 'fs';
	import axios from 'axios';
	import { helper } from './utils';
</script>
`
	imports, err := ParseSvelteImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 3)

	assertImportType(t, imports, "fs", javascript.NodeBuiltinImport{})
	assertImportType(t, imports, "axios", javascript.ExternalImport{})
	assertImportType(t, imports, "./utils", javascript.InternalImport{})
}

func TestResolveSvelteImportPath(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/stores.js":               true,
		"/project/src/Button.svelte":           true,
		"/project/src/components/index.svelte": true,
	}

	sourceFile := "/project/src/App.svelte"

	resolved := ResolveSvelteImportPath(sourceFile, "./stores", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/stores.js")

	resolved = ResolveSvelteImportPath(sourceFile, "./Button", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/Button.svelte")

	resolved = ResolveSvelteImportPath(sourceFile, "./components", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/components/index.svelte")
}

func TestResolveSvelteImportPath_ExplicitExtension(t *testing.T) {
	suppliedFiles := map[string]bool{
		"/project/src/Header.svelte": true,
	}

	sourceFile := "/project/src/App.svelte"

	resolved := ResolveSvelteImportPath(sourceFile, "./Header.svelte", suppliedFiles)
	assert.Contains(t, resolved, "/project/src/Header.svelte")
}

func TestResolveSvelteProjectImports_ExtensionlessTypeScriptImport(t *testing.T) {
	sourceFile := "/project/src/components/Timeline.svelte"
	targetFile := "/project/src/lib/viewer/timelineKeyboard.ts"
	suppliedFiles := map[string]bool{
		sourceFile: true,
		targetFile: true,
	}
	contentByPath := map[string][]byte{
		sourceFile: []byte(`<script lang="ts">
	import { shouldHandleTimelineKeydown } from '../lib/viewer/timelineKeyboard';
</script>`),
		targetFile: []byte(`export function shouldHandleTimelineKeydown() { return true; }`),
	}

	resolved, err := ResolveSvelteProjectImports(sourceFile, sourceFile, suppliedFiles, func(path string) ([]byte, error) {
		content, ok := contentByPath[path]
		if !ok {
			return nil, fmt.Errorf("missing fixture content for %s", path)
		}
		return content, nil
	})

	require.NoError(t, err)
	assert.Contains(t, resolved, targetFile)
}

// Helper functions

func extractPaths(imports []javascript.JavaScriptImport) []string {
	paths := make([]string, len(imports))
	for i, imp := range imports {
		paths[i] = imp.Path()
	}
	return paths
}

func assertImportType(t *testing.T, imports []javascript.JavaScriptImport, path string, expectedType javascript.JavaScriptImport) {
	t.Helper()
	for _, imp := range imports {
		if imp.Path() == path {
			switch expectedType.(type) {
			case javascript.NodeBuiltinImport:
				_, ok := imp.(javascript.NodeBuiltinImport)
				assert.True(t, ok, "Expected %s to be NodeBuiltinImport, got %T", path, imp)
			case javascript.ExternalImport:
				_, ok := imp.(javascript.ExternalImport)
				assert.True(t, ok, "Expected %s to be ExternalImport, got %T", path, imp)
			case javascript.InternalImport:
				_, ok := imp.(javascript.InternalImport)
				assert.True(t, ok, "Expected %s to be InternalImport, got %T", path, imp)
			}
			return
		}
	}
	t.Errorf("Import with path %s not found", path)
}
