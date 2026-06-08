package svelte

import (
	"fmt"
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/depgraph/languages/javascript"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/typescript"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveSvelteProjectImports(
	absPath string,
	filePath string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, parseErr := ParseSvelteImports(content)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	var projectImports []string
	for _, imp := range imports {
		if internalImp, ok := imp.(javascript.InternalImport); ok {
			resolvedFiles := ResolveSvelteImportPath(absPath, internalImp.Path(), suppliedFiles)
			projectImports = append(projectImports, resolvedFiles...)
		}
	}

	return projectImports, nil
}

// ResolveSvelteImportPath resolves a Svelte import path to possible file paths.
// Svelte scripts can be JavaScript or TypeScript, so extensionless imports need
// both JS/JSX and TS/TSX candidates before falling back to .svelte.
func ResolveSvelteImportPath(sourceFile, importPath string, suppliedFiles map[string]bool) []string {
	var resolved []string
	seen := make(map[string]bool)
	add := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		resolved = append(resolved, path)
	}

	for _, path := range typescript.ResolveTypeScriptImportPath(sourceFile, importPath, suppliedFiles) {
		add(path)
	}
	for _, path := range javascript.ResolveJavaScriptImportPath(sourceFile, importPath, suppliedFiles) {
		add(path)
	}

	sourceDir := filepath.Dir(sourceFile)
	basePath := filepath.Join(sourceDir, importPath)
	basePath = filepath.Clean(basePath)

	// Try .svelte extension
	candidate := basePath + ".svelte"
	if suppliedFiles[candidate] {
		add(candidate)
	}

	// Try index.svelte for directory imports
	indexCandidate := filepath.Join(basePath, "index.svelte")
	if suppliedFiles[indexCandidate] {
		add(indexCandidate)
	}

	// If import already ends with .svelte, try exact path
	if filepath.Ext(importPath) == ".svelte" {
		if suppliedFiles[basePath] {
			add(basePath)
		}
	}

	return resolved
}
