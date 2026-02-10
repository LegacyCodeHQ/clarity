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
		resolvedFiles := ResolvePythonImportPath(absPath, imp.Path(), suppliedFiles)
		projectImports = append(projectImports, resolvedFiles...)
		resolvedFiles = ResolvePythonAbsoluteImportPath(imp.Path(), suppliedFiles)
		projectImports = append(projectImports, resolvedFiles...)
	}

	return projectImports, nil
}
