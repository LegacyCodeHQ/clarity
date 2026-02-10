package c

import (
	"fmt"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveCProjectIncludes(
	absPath string,
	filePath string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	includes, parseErr := ParseCIncludes(content)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse includes in %s: %w", filePath, parseErr)
	}

	var projectIncludes []string
	for _, inc := range includes {
		if inc.Kind != IncludeLocal {
			continue
		}
		resolvedFiles := ResolveCIncludePath(absPath, inc.Path, suppliedFiles)
		projectIncludes = append(projectIncludes, resolvedFiles...)
	}

	return projectIncludes, nil
}
