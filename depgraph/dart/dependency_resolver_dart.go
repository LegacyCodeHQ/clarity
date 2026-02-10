package dart

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveDartProjectImports(
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

	var projectImports []string
	for _, imp := range imports {
		if projImp, ok := imp.(ProjectImport); ok {
			resolvedPath := resolveImportPath(absPath, projImp.URI(), ext)
			if suppliedFiles[resolvedPath] {
				projectImports = append(projectImports, resolvedPath)
			}
		}
	}

	return projectImports, nil
}

// resolveImportPath converts a relative import URI to an absolute path
func resolveImportPath(sourceFile, importURI, fileExt string) string {
	// Get directory of source file
	sourceDir := filepath.Dir(sourceFile)

	// Resolve relative import
	absImport := filepath.Join(sourceDir, importURI)

	// Add file extension if not present
	if !strings.HasSuffix(absImport, fileExt) {
		absImport += fileExt
	}

	return filepath.Clean(absImport)
}
