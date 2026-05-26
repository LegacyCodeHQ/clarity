package markdown

import (
	"fmt"
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// ResolveMarkdownProjectImports extracts link targets from a Markdown file and
// returns the supplied project paths they resolve to.
func ResolveMarkdownProjectImports(
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

	links := ParseMarkdownLinks(content)

	var projectImports []string
	seen := make(map[string]bool, len(links))
	for _, link := range links {
		resolved := ResolveMarkdownLinkPath(absPath, link.Path(), suppliedFiles)
		for _, r := range resolved {
			if seen[r] {
				continue
			}
			seen[r] = true
			projectImports = append(projectImports, r)
		}
	}

	return projectImports, nil
}

// ResolveMarkdownLinkPath resolves a Markdown link target against the source
// file's directory and returns matching project files.
func ResolveMarkdownLinkPath(sourceFile, linkPath string, suppliedFiles map[string]bool) []string {
	if linkPath == "" {
		return nil
	}

	if filepath.IsAbs(linkPath) {
		if suppliedFiles[linkPath] {
			return []string{linkPath}
		}
		return nil
	}

	sourceDir := filepath.Dir(sourceFile)
	candidate := filepath.Clean(filepath.Join(sourceDir, linkPath))
	if suppliedFiles[candidate] {
		return []string{candidate}
	}

	// If the link points at a directory, look for an index document.
	for _, indexName := range []string{"README.md", "index.md"} {
		indexed := filepath.Join(candidate, indexName)
		if suppliedFiles[indexed] {
			return []string{indexed}
		}
	}

	// Tolerate paths the author wrote without the `.md` extension.
	if filepath.Ext(candidate) == "" {
		withExt := candidate + ".md"
		if suppliedFiles[withExt] {
			return []string{withExt}
		}
	}

	return nil
}
