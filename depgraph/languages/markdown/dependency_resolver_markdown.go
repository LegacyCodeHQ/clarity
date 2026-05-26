package markdown

import (
	"fmt"
	"path/filepath"
	"strings"

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
//
// Static-site generators (VitePress, Docusaurus, Astro Starlight) link
// between docs with site-absolute URLs ("/guide/foo"). Those don't share the
// filesystem root, so we resolve them by suffix-matching the supplied project
// files.
func ResolveMarkdownLinkPath(sourceFile, linkPath string, suppliedFiles map[string]bool) []string {
	if linkPath == "" {
		return nil
	}

	linkPath = stripRenderedExtension(linkPath)

	var resolved []string
	if strings.HasPrefix(linkPath, "/") {
		resolved = resolveSiteAbsoluteLink(linkPath, suppliedFiles)
	} else {
		sourceDir := filepath.Dir(sourceFile)
		candidate := filepath.Clean(filepath.Join(sourceDir, linkPath))
		resolved = resolveCandidate(candidate, suppliedFiles)
	}

	return dropSelfReferences(sourceFile, resolved)
}

// dropSelfReferences removes the source file from a resolved result set so
// `[anchor]: same-file.html#anchor` style links don't materialize as self-loops
// in the dependency graph.
func dropSelfReferences(sourceFile string, resolved []string) []string {
	if len(resolved) == 0 {
		return resolved
	}
	filtered := resolved[:0]
	for _, r := range resolved {
		if r == sourceFile {
			continue
		}
		filtered = append(filtered, r)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// stripRenderedExtension drops the rendered HTML extension some authors write
// (e.g. `/guide/foo.html`) so the underlying `.md` source can be located.
func stripRenderedExtension(linkPath string) string {
	if strings.HasSuffix(linkPath, ".html") {
		return strings.TrimSuffix(linkPath, ".html")
	}
	if strings.HasSuffix(linkPath, ".htm") {
		return strings.TrimSuffix(linkPath, ".htm")
	}
	return linkPath
}

// resolveSiteAbsoluteLink finds project files that match a "/foo/bar" style
// link. It first tries exact-suffix matches in the supplied file set; if none
// matches, it tries the ".md", "README.md", and "index.md" expansions.
func resolveSiteAbsoluteLink(linkPath string, suppliedFiles map[string]bool) []string {
	candidates := siteAbsoluteCandidateSuffixes(linkPath)

	for _, suffix := range candidates {
		var matches []string
		for path := range suppliedFiles {
			if strings.HasSuffix(path, suffix) {
				matches = append(matches, path)
			}
		}
		if len(matches) == 1 {
			return matches
		}
		// Ambiguous suffix match (same site path served from multiple roots)
		// or no match — fall through to the next expansion. Skip ambiguous
		// matches to avoid fan-out edges.
		if len(matches) > 1 {
			return nil
		}
	}
	return nil
}

// siteAbsoluteCandidateSuffixes returns the file-suffix expansions to try for
// a site-absolute link, in priority order. Trailing slashes are stripped
// because they are a URL convention, not a filesystem directory marker; a
// Hugo URL like `/docs/foo/` may resolve to either `docs/foo.md` (leaf) or
// `docs/foo/_index.md` (section index).
func siteAbsoluteCandidateSuffixes(linkPath string) []string {
	sep := string(filepath.Separator)
	clean := strings.TrimPrefix(linkPath, "/")
	clean = strings.TrimRight(clean, "/")
	clean = filepath.FromSlash(clean)

	var suffixes []string
	if filepath.Ext(clean) != "" {
		suffixes = append(suffixes, sep+clean)
	} else {
		suffixes = append(
			suffixes,
			sep+clean+".md",
			sep+clean+sep+"_index.md",
			sep+clean+sep+"README.md",
			sep+clean+sep+"index.md")
	}
	return suffixes
}

// resolveCandidate matches a fully-joined relative candidate against the
// supplied file set, with `.md` and directory-index fallbacks. Hugo section
// pages live in `_index.md`; mdBook/GitHub use `README.md` or `index.md`.
func resolveCandidate(candidate string, suppliedFiles map[string]bool) []string {
	if suppliedFiles[candidate] {
		return []string{candidate}
	}

	for _, indexName := range []string{"_index.md", "README.md", "index.md"} {
		indexed := filepath.Join(candidate, indexName)
		if suppliedFiles[indexed] {
			return []string{indexed}
		}
	}

	if filepath.Ext(candidate) == "" {
		withExt := candidate + ".md"
		if suppliedFiles[withExt] {
			return []string{withExt}
		}
	}

	return nil
}
