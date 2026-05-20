package rust

import (
	"bufio"
	"path/filepath"
	"strings"
	"sync"
)

// cargoWorkspace captures the subset of a Cargo workspace manifest needed for
// cross-crate import resolution: a mapping from crate name (as declared in
// `[workspace.dependencies]` or in each member crate's `[package] name`)
// to the absolute directory of that crate.
type cargoWorkspace struct {
	rootDir string
	crates  map[string]string // "uv-resolver" -> "/abs/path/crates/uv-resolver"
}

// loadCargoWorkspaceFor walks up from crateRoot looking for an ancestor
// Cargo.toml that declares a `[workspace]` section. When found, it returns a
// cargoWorkspace populated from:
//
//  1. `[workspace] members = [...]` (glob-expanded; each crate's [package] name
//     is read from its own Cargo.toml).
//  2. `[workspace.dependencies]` entries with a `path = "..."` field (covers
//     crates that are central-declared but aren't members of the workspace).
//
// Returns nil if no workspace root is found above crateRoot.
//
// Results are cached per resolver instance (keyed by directory) so repeated
// calls within a single graph build pay the discovery cost only once.
func (r *ProjectImportResolver) loadCargoWorkspaceFor(crateRoot string) *cargoWorkspace {
	if crateRoot == "" {
		return nil
	}
	if cached, ok := r.workspaceCache.Load(crateRoot); ok {
		if cached == nil {
			return nil
		}
		return cached.(*cargoWorkspace)
	}

	visited := make([]string, 0, 8)
	current := crateRoot
	for {
		visited = append(visited, current)

		if cached, ok := r.workspaceCache.Load(current); ok {
			ws := cached.(*cargoWorkspace)
			for _, d := range visited {
				r.workspaceCache.Store(d, ws)
			}
			return ws
		}

		if ws := r.tryLoadCargoWorkspaceAt(current); ws != nil {
			for _, d := range visited {
				r.workspaceCache.Store(d, ws)
			}
			return ws
		}

		parent := filepath.Dir(current)
		if parent == current {
			for _, d := range visited {
				r.workspaceCache.Store(d, (*cargoWorkspace)(nil))
			}
			return nil
		}
		current = parent
	}
}

// tryLoadCargoWorkspaceAt reads `<dir>/Cargo.toml` and, if it declares a
// `[workspace]` section, returns a fully-populated cargoWorkspace.
// Returns nil if dir is not a workspace root.
func (r *ProjectImportResolver) tryLoadCargoWorkspaceAt(dir string) *cargoWorkspace {
	if r.contentReader == nil {
		return nil
	}
	cargoTomlPath := filepath.Join(dir, "Cargo.toml")
	content, err := r.contentReader(cargoTomlPath)
	if err != nil {
		return nil
	}
	if !hasCargoWorkspaceSection(string(content)) {
		return nil
	}

	crates := make(map[string]string)

	// 1. Members glob expansion. Each member directory's [package] name is
	// the canonical crate name.
	for _, member := range parseCargoWorkspaceMembers(string(content)) {
		for _, memberDir := range expandCargoMemberPattern(dir, member) {
			memberCargoToml := filepath.Join(memberDir, "Cargo.toml")
			memberContent, err := r.contentReader(memberCargoToml)
			if err != nil {
				continue
			}
			for name := range parseRustCrateNamesFromCargoToml(string(memberContent)) {
				crates[name] = memberDir
			}
		}
	}

	// 2. [workspace.dependencies] path entries. Covers crates referenced
	// centrally but not in the `members` glob.
	for _, dep := range parseCargoWorkspaceDependencies(string(content)) {
		depDir := dep.path
		if !filepath.IsAbs(depDir) {
			depDir = filepath.Join(dir, depDir)
		}
		depDir = filepath.Clean(depDir)

		// Prefer the package name from the dependency's own Cargo.toml; fall
		// back to the key used in the workspace.dependencies table.
		named := false
		if depContent, err := r.contentReader(filepath.Join(depDir, "Cargo.toml")); err == nil {
			for name := range parseRustCrateNamesFromCargoToml(string(depContent)) {
				crates[name] = depDir
				named = true
			}
		}
		if !named {
			for _, name := range dep.importNames {
				if name != "" {
					crates[name] = depDir
				}
			}
		}
	}

	if len(crates) == 0 {
		return nil
	}
	return &cargoWorkspace{rootDir: dir, crates: crates}
}

// hasCargoWorkspaceSection returns true if content contains a [workspace]
// section header. We deliberately avoid a full TOML parser — Cargo manifests
// are constrained enough that a line scan suffices.
func hasCargoWorkspaceSection(content string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[workspace]" {
			return true
		}
		if strings.HasPrefix(line, "[workspace.") {
			return true
		}
	}
	return false
}

// parseCargoWorkspaceMembers extracts the `members` array from `[workspace]`.
// Supports the common forms:
//
//	[workspace]
//	members = ["crate-a", "crate-b"]
//
//	[workspace]
//	members = [
//	  "crates/*",
//	  "tools/foo",
//	]
func parseCargoWorkspaceMembers(content string) []string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	section := ""
	var members []string
	inMembersArray := false

	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if inMembersArray {
				// New section while inside the array — stop.
				inMembersArray = false
			}
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}

		if section != "workspace" && !inMembersArray {
			continue
		}

		if !inMembersArray {
			key, value, ok := parseTomlKeyValue(line)
			if !ok || key != "members" {
				continue
			}
			// Single-line: members = ["a", "b"]
			if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				members = append(members, extractStringsFromInlineArray(value)...)
				continue
			}
			// Multi-line: members = [
			if strings.HasPrefix(value, "[") {
				members = append(members, extractStringsFromInlineArray(value)...)
				inMembersArray = true
				continue
			}
			continue
		}

		// Inside a multi-line members array — read until closing ].
		members = append(members, extractStringsFromInlineArray(line)...)
		if strings.Contains(line, "]") {
			inMembersArray = false
		}
	}

	return dedupeNonEmptyStrings(members)
}

// extractStringsFromInlineArray pulls quoted strings out of a TOML-style
// inline array fragment. Tolerant of trailing commas, embedded ]s, etc.
func extractStringsFromInlineArray(fragment string) []string {
	var out []string
	for {
		open := strings.Index(fragment, "\"")
		if open < 0 {
			return out
		}
		fragment = fragment[open+1:]
		close := strings.Index(fragment, "\"")
		if close < 0 {
			return out
		}
		val := fragment[:close]
		if val != "" {
			out = append(out, val)
		}
		fragment = fragment[close+1:]
	}
}

// parseCargoWorkspaceDependencies extracts `[workspace.dependencies]` entries
// that declare a `path = "..."` field. Crates pulled from registries don't
// have a `path` and aren't useful for the dep graph.
func parseCargoWorkspaceDependencies(content string) []rustPathDependencyEntry {
	scanner := bufio.NewScanner(strings.NewReader(content))
	section := ""
	var entries []rustPathDependencyEntry

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		if section != "workspace.dependencies" {
			continue
		}

		key, value, ok := parseTomlKeyValue(line)
		if !ok || !strings.HasPrefix(value, "{") {
			continue
		}
		path := parseTomlInlineString(value, "path")
		if path == "" {
			continue
		}

		importNames := []string{normalizeCargoCrateName(trimQuotes(key))}
		if pkg := parseTomlInlineString(value, "package"); pkg != "" {
			importNames = append(importNames, normalizeCargoCrateName(pkg))
		}
		entries = append(entries, rustPathDependencyEntry{
			importNames: dedupeNonEmptyStrings(importNames),
			path:        path,
		})
	}
	return entries
}

// expandCargoMemberPattern handles the limited glob vocabulary Cargo uses for
// `[workspace] members`. Cargo supports `*` as a single-segment wildcard;
// `**` is not supported by Cargo at all.
func expandCargoMemberPattern(root, pattern string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}
	if !strings.Contains(pattern, "*") {
		return []string{filepath.Clean(filepath.Join(root, pattern))}
	}
	matches, _ := filepath.Glob(filepath.Join(root, pattern))
	var dirs []string
	for _, m := range matches {
		dirs = append(dirs, filepath.Clean(m))
	}
	return dirs
}

// Compile-time assertion that ProjectImportResolver has a workspaceCache
// field. The actual declaration lives in dependency_resolver_rust.go so
// the structure layout stays in one place.
var _ = sync.Map{}
