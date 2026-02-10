package javascript

import (
	"fmt"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveJavaScriptProjectImports(
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

	imports, parseErr := ParseJavaScriptImports(content, ext == ".jsx")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	var projectImports []string
	for _, imp := range imports {
		if internalImp, ok := imp.(InternalImport); ok {
			resolvedFiles := ResolveJavaScriptImportPath(absPath, internalImp.Path(), suppliedFiles)
			projectImports = append(projectImports, resolvedFiles...)
		}
	}

	return projectImports, nil
}
