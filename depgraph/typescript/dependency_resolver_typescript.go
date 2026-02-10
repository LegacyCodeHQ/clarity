package typescript

import (
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

	imports, parseErr := ParseTypeScriptImports(content, ext == ".tsx")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	var projectImports []string
	for _, imp := range imports {
		if internalImp, ok := imp.(InternalImport); ok {
			resolvedFiles := ResolveTypeScriptImportPath(absPath, internalImp.Path(), suppliedFiles)
			projectImports = append(projectImports, resolvedFiles...)
		}
	}

	return projectImports, nil
}
