package typescript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// npmWorkspace captures the subset of an npm/yarn/pnpm workspace manifest
// needed for cross-package import resolution: a map from package name (as
// declared in each package's own package.json) to the absolute directory
// containing that package.
type npmWorkspace struct {
	rootDir  string
	packages map[string]string // "@tanstack/query-core" -> "/abs/path/packages/query-core"
}

type npmWorkspaceEntry struct {
	ws *npmWorkspace
}

var npmWorkspaceCache sync.Map // dir -> *npmWorkspaceEntry

// loadNpmWorkspaceFor walks up from sourceFile looking for a workspace root
// (either a package.json with a "workspaces" field or a pnpm-workspace.yaml
// sibling). Returns nil if none is found.
//
// Caches per-directory results so a single source tree only pays the
// discovery cost once.
func loadNpmWorkspaceFor(sourceFile string) *npmWorkspace {
	dir := filepath.Dir(sourceFile)
	visited := []string{}
	for {
		if cached, ok := npmWorkspaceCache.Load(dir); ok {
			ws := cached.(*npmWorkspaceEntry).ws
			for _, d := range visited {
				npmWorkspaceCache.Store(d, &npmWorkspaceEntry{ws: ws})
			}
			return ws
		}
		visited = append(visited, dir)

		if ws := tryLoadNpmWorkspaceAt(dir); ws != nil {
			for _, d := range visited {
				npmWorkspaceCache.Store(d, &npmWorkspaceEntry{ws: ws})
			}
			return ws
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			for _, d := range visited {
				npmWorkspaceCache.Store(d, &npmWorkspaceEntry{ws: nil})
			}
			return nil
		}
		dir = parent
	}
}

// tryLoadNpmWorkspaceAt looks for workspace manifests in dir and, if found,
// returns a fully-populated npmWorkspace. Returns nil if dir is not a
// workspace root.
func tryLoadNpmWorkspaceAt(dir string) *npmWorkspace {
	patterns := readWorkspacePatterns(dir)
	if len(patterns) == 0 {
		return nil
	}

	packages := make(map[string]string)
	for _, pkgDir := range expandWorkspacePatterns(dir, patterns) {
		name := readPackageName(pkgDir)
		if name == "" {
			continue
		}
		packages[name] = pkgDir
	}
	if len(packages) == 0 {
		return nil
	}
	return &npmWorkspace{rootDir: dir, packages: packages}
}

// readWorkspacePatterns inspects dir for npm/yarn (package.json "workspaces")
// and pnpm (pnpm-workspace.yaml) manifests, returning the union of all
// declared glob patterns. Returns nil if dir has no workspace manifest.
func readWorkspacePatterns(dir string) []string {
	var patterns []string

	// 1. package.json "workspaces" (npm / yarn classic / yarn berry)
	if pkgJSON, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		patterns = append(patterns, parseWorkspacesFromPackageJSON(pkgJSON)...)
	}

	// 2. pnpm-workspace.yaml (pnpm)
	if pnpmYAML, err := os.ReadFile(filepath.Join(dir, "pnpm-workspace.yaml")); err == nil {
		patterns = append(patterns, parsePnpmWorkspaceYAML(pnpmYAML)...)
	}

	return patterns
}

// parseWorkspacesFromPackageJSON handles both shapes the npm / yarn ecosystem
// uses:
//
//	"workspaces": ["packages/*"]
//	"workspaces": {"packages": ["packages/*"], "nohoist": [...]}
func parseWorkspacesFromPackageJSON(data []byte) []string {
	var raw struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw.Workspaces) == 0 {
		return nil
	}

	var arr []string
	if err := json.Unmarshal(raw.Workspaces, &arr); err == nil {
		return arr
	}

	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw.Workspaces, &obj); err == nil {
		return obj.Packages
	}
	return nil
}

// parsePnpmWorkspaceYAML extracts the `packages:` list from pnpm-workspace.yaml.
// Deliberately a regex-style line scanner rather than a full YAML parser —
// the file format is constrained and we don't want to take a new dependency
// just for one field.
//
// Supports the common shapes:
//
//	packages:
//	  - 'packages/*'
//	  - "apps/**"
//	  - dirs/foo
func parsePnpmWorkspaceYAML(data []byte) []string {
	var patterns []string
	lines := strings.Split(string(data), "\n")
	inPackages := false
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			continue
		}

		// Detect the `packages:` key (must start at indent 0; YAML siblings
		// would otherwise leak).
		if !inPackages {
			if strings.HasPrefix(line, "packages:") {
				inPackages = true
			}
			continue
		}

		// While we're in the packages: block, accept lines starting with `- `.
		// First non-list-item line at column 0 closes the block.
		if strings.HasPrefix(trimmed, "- ") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			value = strings.Trim(value, "'\"")
			if value != "" {
				patterns = append(patterns, value)
			}
			continue
		}
		// Any other content at column 0 means we left the packages: block.
		if line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
	}
	return patterns
}

// expandWorkspacePatterns expands glob patterns like "packages/*" and
// "apps/**" to absolute package directories rooted at workspaceRoot.
// Each returned directory must contain its own package.json — otherwise it
// isn't a real package.
func expandWorkspacePatterns(workspaceRoot string, patterns []string) []string {
	seen := make(map[string]bool)
	var pkgDirs []string

	for _, pat := range patterns {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			continue
		}
		matches := matchWorkspaceGlob(workspaceRoot, pat)
		for _, m := range matches {
			if seen[m] {
				continue
			}
			seen[m] = true
			if _, err := os.Stat(filepath.Join(m, "package.json")); err != nil {
				continue
			}
			pkgDirs = append(pkgDirs, m)
		}
	}
	return pkgDirs
}

// matchWorkspaceGlob handles the limited glob vocabulary npm / pnpm support:
// `*` matches a single path segment, `**` matches any number of segments.
// Anything more exotic falls back to filepath.Glob (which handles a single-*).
func matchWorkspaceGlob(root, pattern string) []string {
	// Negation patterns ("!foo/bar") are theoretically supported by yarn but
	// rare and tricky; not handled in the first cut.
	if strings.HasPrefix(pattern, "!") {
		return nil
	}

	if !strings.Contains(pattern, "**") {
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		var dirs []string
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || !info.IsDir() {
				continue
			}
			dirs = append(dirs, m)
		}
		return dirs
	}

	// "**" expansion — walk the tree under the static prefix, matching any
	// directory whose relative path satisfies the pattern.
	prefix := pattern
	if idx := strings.Index(prefix, "**"); idx >= 0 {
		prefix = strings.TrimSuffix(prefix[:idx], "/")
	}
	base := filepath.Join(root, prefix)
	var dirs []string
	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		// Skip node_modules to avoid catastrophic descents.
		if d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		// Match by trying filepath.Match with the pattern's wildcards
		// flattened: replace ** with *.
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		ok, _ := filepath.Match(strings.ReplaceAll(pattern, "**", "*"), rel)
		if ok {
			dirs = append(dirs, path)
		}
		return nil
	})
	return dirs
}

// readPackageName returns the "name" field from a package.json sitting in
// pkgDir, or the empty string if the file is missing / malformed / nameless.
func readPackageName(pkgDir string) string {
	data, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err != nil {
		return ""
	}
	var raw struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return strings.TrimSpace(raw.Name)
}

// resolveWorkspacePackage takes an import path like `@tanstack/query-core` or
// `@tanstack/query-core/devtools` and, if it points at a workspace package,
// returns:
//
//	(packageDir, subpath, true)
//
// packageDir is the absolute directory of the matching workspace package.
// subpath is the remainder of the import (empty for a bare-package import,
// e.g. "devtools" for "@tanstack/query-core/devtools").
//
// Returns ("", "", false) if importPath does not name a workspace package.
func (w *npmWorkspace) resolveWorkspacePackage(importPath string) (string, string, bool) {
	if w == nil {
		return "", "", false
	}
	// Try the full import path first; if it matches a package name exactly,
	// that's a bare-package import. Otherwise strip trailing segments until
	// either a package name matches or we run out of segments.
	parts := strings.Split(importPath, "/")
	for i := len(parts); i > 0; i-- {
		candidate := strings.Join(parts[:i], "/")
		if dir, ok := w.packages[candidate]; ok {
			sub := strings.TrimPrefix(importPath, candidate)
			sub = strings.TrimPrefix(sub, "/")
			return dir, sub, true
		}
	}
	return "", "", false
}
