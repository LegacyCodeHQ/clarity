package zig

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveZigProjectImports(
	absPath string,
	filePath string,
	ext string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, err := ParseImports(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	projectImports := make([]string, 0, len(imports))
	for _, imp := range imports {
		resolvedPath, ok := resolveImportPath(absPath, imp.Path, ext, suppliedFiles)
		if ok {
			projectImports = append(projectImports, resolvedPath)
		}
	}

	return projectImports, nil
}

func resolveImportPath(sourceFile, importPath, fileExt string, suppliedFiles map[string]bool) (string, bool) {
	if !strings.HasSuffix(importPath, fileExt) {
		return "", false
	}

	resolved := importPath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(sourceFile), resolved)
	}
	resolved = filepath.Clean(resolved)

	if !suppliedFiles[resolved] {
		return "", false
	}

	return resolved, true
}
