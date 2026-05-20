package typescript

import (
	"bytes"
	"fmt"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveTypeScriptProjectImports(
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
	// Skip files that almost certainly contain no imports. We accept any file
	// containing "import " (or "from " for the rare side-effect-import-less
	// pattern, or "require(" for CommonJS) because workspace-only files
	// don't contain ./, ../, or @/ but still need parsing.
	if !bytes.Contains(content, []byte("import ")) &&
		!bytes.Contains(content, []byte("import{")) &&
		!bytes.Contains(content, []byte("from \"")) &&
		!bytes.Contains(content, []byte("from '")) &&
		!bytes.Contains(content, []byte("require(")) {
		return nil, nil
	}

	imports, parseErr := ParseTypeScriptImports(content, ext == ".tsx")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	var projectImports []string
	seen := make(map[string]bool, len(imports))
	add := func(resolved string) {
		if seen[resolved] {
			return
		}
		seen[resolved] = true
		projectImports = append(projectImports, resolved)
	}

	for _, imp := range imports {
		switch typed := imp.(type) {
		case InternalImport:
			for _, resolved := range ResolveTypeScriptImportPath(absPath, typed.Path(), suppliedFiles) {
				add(resolved)
			}
		case ExternalImport:
			// External imports normally aren't project files. The exception
			// is when the same path is actually a sibling workspace package
			// in an npm/pnpm/yarn monorepo — ResolveTypeScriptImportPath
			// knows how to look that up via the workspace discovery layer.
			for _, resolved := range ResolveTypeScriptImportPath(absPath, typed.Path(), suppliedFiles) {
				add(resolved)
			}
		}
	}

	return projectImports, nil
}
