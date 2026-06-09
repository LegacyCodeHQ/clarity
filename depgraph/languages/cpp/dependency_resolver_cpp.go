package cpp

import (
	"fmt"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveCppProjectIncludes(
	absPath string,
	filePath string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	includes, parseErr := ParseCppIncludes(content)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse includes in %s: %w", filePath, parseErr)
	}

	var projectIncludes []string
	for _, inc := range includes {
		// Resolve every include against the supplied files, not just quoted
		// ones: a project header is frequently included with angle brackets
		// (e.g. <proj/foo.h> via -I). System headers never match a supplied
		// file, so resolving angle-bracket includes stays precise.
		resolvedFiles := ResolveCppIncludePath(absPath, inc.Path, suppliedFiles)
		projectIncludes = append(projectIncludes, resolvedFiles...)
	}

	return projectIncludes, nil
}
