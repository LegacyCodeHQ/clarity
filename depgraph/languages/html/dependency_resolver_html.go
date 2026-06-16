package html

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// ResolveHTMLProjectImports extracts link/asset references from an HTML file and
// returns the supplied project paths they resolve to.
func ResolveHTMLProjectImports(
	absPath string,
	_ string,
	_ string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	var projectImports []string
	seen := make(map[string]bool)
	for _, link := range ParseHTMLLinks(content) {
		for _, resolved := range ResolveHTMLLinkPath(absPath, link, suppliedFiles) {
			if seen[resolved] {
				continue
			}
			seen[resolved] = true
			projectImports = append(projectImports, resolved)
		}
	}

	return projectImports, nil
}

// ResolveHTMLLinkPath resolves an HTML link target against the source file's
// directory and returns matching project files.
//
// Server-absolute links ("/css/app.css") do not share the filesystem root, so
// they are resolved by suffix-matching the supplied project files, mirroring the
// Markdown resolver.
func ResolveHTMLLinkPath(sourceFile, linkPath string, suppliedFiles map[string]bool) []string {
	if linkPath == "" {
		return nil
	}

	var resolved []string
	if strings.HasPrefix(linkPath, "/") {
		resolved = resolveServerAbsoluteLink(linkPath, suppliedFiles)
	} else {
		sourceDir := filepath.Dir(sourceFile)
		candidate := filepath.Clean(filepath.Join(sourceDir, linkPath))
		if suppliedFiles[candidate] {
			resolved = []string{candidate}
		}
	}

	if len(resolved) == 1 && resolved[0] == sourceFile {
		return nil
	}
	return resolved
}

// resolveServerAbsoluteLink matches a "/foo/bar.css" style link against the
// supplied file set by suffix. An ambiguous match (the same server path served
// from multiple roots) is dropped to avoid fan-out edges.
func resolveServerAbsoluteLink(linkPath string, suppliedFiles map[string]bool) []string {
	suffix := filepath.FromSlash(linkPath)

	var matches []string
	for path := range suppliedFiles {
		if strings.HasSuffix(path, suffix) {
			matches = append(matches, path)
		}
	}
	if len(matches) == 1 {
		return matches
	}
	return nil
}
