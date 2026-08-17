package python

import (
	"fmt"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolvePythonProjectImports(
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

	imports, parseErr := ParsePythonImports(content)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	var projectImports []string
	for _, imp := range imports {
		resolved := resolvePythonImportTarget(absPath, imp.Path(), suppliedFiles)
		if len(resolved) == 0 && imp.FallbackPath() != "" {
			// The name isn't a submodule that exists on disk -- it's an
			// attribute defined inside the module's own file/package instead.
			resolved = resolvePythonImportTarget(absPath, imp.FallbackPath(), suppliedFiles)
		}
		projectImports = append(projectImports, resolved...)
	}

	return projectImports, nil
}

// resolvePythonImportTarget tries both the relative and absolute resolvers
// for a single import path. Exactly one is a match for any given path shape
// (relative paths start with ".", absolute ones don't), so only one ever
// returns anything.
func resolvePythonImportTarget(absPath, importPath string, suppliedFiles map[string]bool) []string {
	resolved := ResolvePythonImportPath(absPath, importPath, suppliedFiles)
	resolved = append(resolved, ResolvePythonAbsoluteImportPath(absPath, importPath, suppliedFiles)...)
	return resolved
}
