package rust

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

type ProjectImportResolver struct {
	suppliedFiles map[string]bool
	contentReader vcs.ContentReader

	crateRootCache     sync.Map // directory path -> crate root (or "")
	targetDirsCache    sync.Map // crate root -> []string (custom target source dirs)
	crateNameCache     sync.Map // crate root -> map[string]bool
	depCrateRootsCache sync.Map // crate root -> map[importName]crateRoot
	modDepsCache       sync.Map // mod.rs path -> []string
	importsCache       sync.Map // file path -> []RustImport
	workspaceCache     sync.Map // directory path -> *cargoWorkspace (or nil sentinel)
}

func NewProjectImportResolver(suppliedFiles map[string]bool, contentReader vcs.ContentReader) *ProjectImportResolver {
	return &ProjectImportResolver{
		suppliedFiles: suppliedFiles,
		contentReader: contentReader,
	}
}

func (r *ProjectImportResolver) ResolveProjectImports(absPath string, filePath string) ([]string, error) {
	imports, err := r.importsForFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	projectImports := make([]string, 0, len(imports))
	for _, imp := range imports {
		switch imp.Kind {
		case RustImportUse:
			projectImports = append(projectImports, r.resolveRustUsePath(absPath, imp.Path)...)
		case RustImportModDecl:
			projectImports = append(projectImports, resolveRustModDecl(absPath, imp.Path, r.suppliedFiles)...)
		case RustImportExternCrate:
			// External crate imports do not map to local project files.
		}
	}

	projectImports = filterOutRustSelfDependency(projectImports, absPath)
	return deduplicateSuppliedFiles(projectImports, r.suppliedFiles), nil
}

func ResolveRustProjectImports(
	absPath string,
	filePath string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	resolver := NewProjectImportResolver(suppliedFiles, contentReader)
	return resolver.ResolveProjectImports(absPath, filePath)
}

func resolveRustModDecl(sourceFile, moduleName string, suppliedFiles map[string]bool) []string {
	if moduleName == "" {
		return nil
	}

	sourceDir := filepath.Dir(sourceFile)
	candidates := []string{
		filepath.Join(sourceDir, moduleName+".rs"),
		filepath.Join(sourceDir, moduleName, "mod.rs"),
	}

	return filterSuppliedFiles(candidates, suppliedFiles)
}

func (r *ProjectImportResolver) resolveRustUsePath(sourceFile, importPath string) []string {
	path := strings.TrimSpace(importPath)
	if path == "" {
		return nil
	}

	firstSegment := firstRustPathSegment(path)
	if firstSegment == "" {
		return nil
	}

	var parts []string
	baseDir := ""
	crateRoot := ""
	rootedInLocalCrate := false

	switch firstSegment {
	case "crate":
		parts = strings.Split(path, "::")
		root, ok := r.findRustCrateRoot(sourceFile)
		if !ok {
			return nil
		}
		crateRoot = root
		baseDir = r.rustCrateSourceDir(root, sourceFile)
		rootedInLocalCrate = true
		parts = parts[1:]
	case "self", "super":
		parts = strings.Split(path, "::")
		baseDir = filepath.Dir(sourceFile)
		// For leaf .rs files (not mod.rs/lib.rs/main.rs), the file is a
		// child of the directory module. baseDir already points to the
		// parent module's directory, so the first `super` is a no-op.
		leafFile := !isRustDirectoryModuleFile(sourceFile)
		for len(parts) > 0 {
			switch parts[0] {
			case "self":
				parts = parts[1:]
			case "super":
				if leafFile {
					leafFile = false
				} else {
					baseDir = filepath.Dir(baseDir)
				}
				parts = parts[1:]
			default:
				goto resolved
			}
		}
	default:
		// In Rust 2018+, a file like `src/app.rs` that declares `mod foo;`
		// brings `foo` into its own module namespace. A `use foo::bar;` from
		// app.rs is resolved relative to app.rs's own module — i.e., it
		// looks up `src/app/foo.rs`. Try that before falling through to
		// crate-root / external-crate resolution; sibling submodules shadow
		// external crates of the same name.
		if siblings := r.resolveRustSiblingSubmoduleCandidates(sourceFile, path); len(siblings) > 0 {
			candidates := r.expandRustModRsCandidates(siblings)
			return deduplicateSuppliedFiles(candidates, r.suppliedFiles)
		}
		root, ok := r.findRustCrateRoot(sourceFile)
		if !ok {
			return nil
		}
		parts = strings.Split(path, "::")
		if r.isLocalRustCrateImport(firstSegment, root) {
			crateRoot = root
			baseDir = r.rustCrateSourceDir(root, sourceFile)
			rootedInLocalCrate = true
		} else if depCrateRoot, ok := r.resolveRustDependencyCrateRoot(firstSegment, root); ok {
			crateRoot = depCrateRoot
			baseDir = filepath.Join(depCrateRoot, "src")
			rootedInLocalCrate = true
		} else {
			return nil
		}
		parts = parts[1:]
	}

resolved:
	if len(parts) == 0 && rootedInLocalCrate {
		return resolveRustCrateRootCandidates(crateRoot, r.suppliedFiles)
	}
	if len(parts) == 0 {
		return nil
	}

	candidates := resolveRustModuleCandidates(baseDir, parts, r.suppliedFiles)
	// Track whether the full import path matched a real file/directory. If it
	// didn't, the last segment is most likely a *symbol* — and the dependency
	// belongs to the file that defines that symbol, not to every sibling of
	// the prefix module.
	matchedFullPath := len(candidates) > 0

	if !matchedFullPath && len(parts) > 1 {
		// Symbol path (e.g. `crate::git::Git`). Look up the symbol against the
		// prefix module's `pub use` re-exports first — that's the form Rust
		// programs use to expose child items from a directory's mod.rs.
		if reExported := r.resolveRustReExportedSymbol(sourceFile, baseDir, parts); len(reExported) > 0 {
			return reExported
		}
		// No re-export; attribute the edge to the prefix module file (where
		// the symbol could live as a top-level item) — and crucially, do NOT
		// run `expandRustModRsCandidates`, because the user named one symbol
		// rather than importing the whole module. Expanding here would draw
		// false-positive edges to every child of the prefix mod.rs.
		candidates = resolveRustModuleCandidates(baseDir, parts[:len(parts)-1], r.suppliedFiles)
		return deduplicateSuppliedFiles(candidates, r.suppliedFiles)
	}
	if rootedInLocalCrate && len(parts) == 1 && len(candidates) == 0 {
		// `use crate::Symbol` — a single segment, so the symbol-path branch
		// above (guarded on len(parts) > 1) never ran. Check the crate root's
		// own `pub use` re-exports before assuming the symbol is defined
		// there: flattening a public API through lib.rs is the standard
		// library shape, and attributing every consumer's dependency to lib.rs
		// both loses the real edge and manufactures a cycle, since lib.rs
		// depends on those consumers in turn.
		if reExported := r.resolveRustReExportedSymbol(sourceFile, baseDir, parts); len(reExported) > 0 {
			return reExported
		}
		candidates = append(candidates, resolveRustCrateRootCandidates(crateRoot, r.suppliedFiles)...)
	}

	if len(candidates) == 0 {
		if reExported := r.resolveRustReExportedSymbol(sourceFile, baseDir, parts); len(reExported) > 0 {
			return reExported
		}
		// Bare identifier reached via `super::`/`self::` with no matching
		// file, directory, or re-export — the symbol must be a top-level item
		// (const, type, function) defined directly in the parent module file.
		// Without this fallback, `use super::DAYS_IN_WEEK;` produces no edge
		// at all when DAYS_IN_WEEK lives in the parent `mod.rs`.
		if !rootedInLocalCrate && len(parts) == 1 {
			if moduleFile := r.findRustModuleFileForDir(baseDir); moduleFile != "" && moduleFile != sourceFile {
				return []string{moduleFile}
			}
		}
	}

	candidates = deduplicateSuppliedFiles(candidates, r.suppliedFiles)
	candidates = r.expandRustModRsCandidates(candidates)
	return deduplicateSuppliedFiles(candidates, r.suppliedFiles)
}

// resolveRustReExportedSymbol handles the case where a `use` path's final
// segment names a symbol that the containing module re-exports via
// `pub use child::Symbol;`. For example, `use super::Fs;` from a sibling of
// `trait_fs.rs` lands in the parent `fs/` directory with no file matching
// "Fs". The fix: inspect the directory's module file (`mod.rs` or `<dir>.rs`)
// for a `use *::Symbol;` whose last segment matches, then resolve that path
// from the module file's perspective.
//
// This mirrors how Rust resolves the lookup: the symbol is brought into the
// parent module's namespace by the re-export, so callers reach the original
// defining file transparently.
func (r *ProjectImportResolver) resolveRustReExportedSymbol(sourceFile, baseDir string, parts []string) []string {
	if baseDir == "" || len(parts) == 0 {
		return nil
	}
	symbol := parts[len(parts)-1]
	if symbol == "" {
		return nil
	}
	targetDir := baseDir
	if len(parts) > 1 {
		targetDir = filepath.Join(append([]string{baseDir}, parts[:len(parts)-1]...)...)
	}
	moduleFile := r.findRustModuleFileForDir(targetDir)
	// `moduleFile == sourceFile` means we'd ask the importing file to act as
	// its own re-export source — and chase its own use statement back into
	// `resolveRustUsePath`, which blows the stack. (Example: `use
	// crate::engine::astgrep::Foo;` written inside `engine/astgrep.rs`.)
	if moduleFile == "" || moduleFile == sourceFile {
		return nil
	}
	imports, err := r.importsForFile(moduleFile)
	if err != nil {
		return nil
	}

	moduleDir := rustModuleOwnedDir(moduleFile)
	var resolved []string
	for _, imp := range imports {
		if imp.Kind != RustImportUse {
			continue
		}
		// A re-export has to live at the module file's own scope. An import
		// inside an inner `mod` block (a `#[cfg(test)] mod tests`, say) is
		// private to that block and re-exports nothing, so following it here
		// would attribute the edge to whatever that inner module happened to
		// import rather than to the prefix module that defines the symbol.
		if imp.Nested {
			continue
		}
		if lastRustPathSegment(imp.Path) != symbol {
			continue
		}
		resolved = append(resolved, r.resolveRustReExportTarget(moduleDir, moduleFile, imp.Path)...)
	}
	return deduplicateSuppliedFiles(resolved, r.suppliedFiles)
}

// resolveRustReExportTarget resolves the target of a `pub use foo::bar::Symbol;`
// re-export found in `moduleFile`. The path is interpreted relative to
// `moduleDir` (the directory that owns the module file). For absolute paths
// (crate::, super::, self::) we delegate to the normal resolver since those
// forms work from any source file. For relative paths like `trait_fs::Fs`,
// the segment names a child module declared by `moduleFile` and lives as a
// sibling file in `moduleDir`.
func (r *ProjectImportResolver) resolveRustReExportTarget(moduleDir, moduleFile, importPath string) []string {
	path := strings.TrimSpace(importPath)
	if path == "" {
		return nil
	}
	switch firstRustPathSegment(path) {
	case "crate", "super", "self":
		return r.resolveRustUsePath(moduleFile, path)
	}
	parts := strings.Split(path, "::")
	if len(parts) == 0 {
		return nil
	}
	candidates := resolveRustModuleCandidates(moduleDir, parts, r.suppliedFiles)
	if len(parts) > 1 && len(candidates) == 0 {
		candidates = append(candidates, resolveRustModuleCandidates(moduleDir, parts[:len(parts)-1], r.suppliedFiles)...)
	}
	return deduplicateSuppliedFiles(candidates, r.suppliedFiles)
}

func (r *ProjectImportResolver) findRustModuleFileForDir(dir string) string {
	candidates := []string{
		filepath.Join(dir, "mod.rs"),
		dir + ".rs",
		filepath.Join(dir, "lib.rs"),
		filepath.Join(dir, "main.rs"),
	}
	for _, candidate := range candidates {
		if r.suppliedFiles[candidate] {
			return candidate
		}
	}
	// Fall back to disk: commit-scoped views (e.g., `clarity show -c HEAD`)
	// only put changed files in `suppliedFiles`. To follow `pub use` chains
	// through an unchanged `mod.rs`, we still need to read it.
	if r.contentReader != nil {
		for _, candidate := range candidates {
			if _, err := r.contentReader(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}

func lastRustPathSegment(path string) string {
	if idx := strings.LastIndex(path, "::"); idx >= 0 {
		return path[idx+2:]
	}
	return path
}

func resolveRustCrateRootCandidates(crateRoot string, suppliedFiles map[string]bool) []string {
	if crateRoot == "" {
		return nil
	}
	return filterSuppliedFiles([]string{filepath.Join(crateRoot, "src", "lib.rs")}, suppliedFiles)
}

// resolveRustSiblingSubmoduleCandidates handles use paths whose first segment
// names a sibling submodule of the source file. For a non-mod-rs leaf file
// like `src/app.rs`, `mod foo;` resolves to `src/app/foo.rs`, and
// `use foo::Bar;` from app.rs targets that file.
func (r *ProjectImportResolver) resolveRustSiblingSubmoduleCandidates(sourceFile, path string) []string {
	parts := strings.Split(path, "::")
	if len(parts) == 0 || parts[0] == "" {
		return nil
	}
	if !strings.HasSuffix(sourceFile, ".rs") {
		return nil
	}
	// Uniform paths (Rust 2018+) let a `use` begin with a module the file
	// itself declares. Where those children live depends on the layout:
	// `app.rs` declares them in `app/`, while `lib.rs`/`mod.rs` declare them
	// beside itself. Previously directory-module files bailed out entirely, so
	// `pub use daemon::heartbeat;` in lib.rs resolved to nothing.
	siblingDir := rustModuleOwnedDir(sourceFile)
	candidates := resolveRustModuleCandidates(siblingDir, parts, r.suppliedFiles)
	if len(parts) > 1 && len(candidates) == 0 {
		candidates = append(candidates, resolveRustModuleCandidates(siblingDir, parts[:len(parts)-1], r.suppliedFiles)...)
	}
	return deduplicateSuppliedFiles(candidates, r.suppliedFiles)
}

func resolveRustModuleCandidates(baseDir string, parts []string, suppliedFiles map[string]bool) []string {
	if baseDir == "" || len(parts) == 0 {
		return nil
	}

	modulePath := filepath.Join(append([]string{baseDir}, parts...)...)
	candidates := []string{
		modulePath + ".rs",
		filepath.Join(modulePath, "mod.rs"),
	}

	return filterSuppliedFiles(candidates, suppliedFiles)
}

func filterOutRustSelfDependency(imports []string, sourceFile string) []string {
	if len(imports) == 0 {
		return imports
	}
	filtered := imports[:0]
	for _, imp := range imports {
		if imp == sourceFile {
			continue
		}
		filtered = append(filtered, imp)
	}
	return filtered
}

func (r *ProjectImportResolver) findRustCrateRoot(sourceFile string) (string, bool) {
	dir := filepath.Dir(sourceFile)
	if cached, ok := r.crateRootCache.Load(dir); ok {
		root := cached.(string)
		return root, root != ""
	}

	current := dir
	visited := make([]string, 0, 8)
	for {
		visited = append(visited, current)

		if cached, ok := r.crateRootCache.Load(current); ok {
			root := cached.(string)
			for _, d := range visited {
				r.crateRootCache.Store(d, root)
			}
			return root, root != ""
		}

		candidate := filepath.Join(current, "Cargo.toml")
		if r.suppliedFiles[candidate] {
			for _, d := range visited {
				r.crateRootCache.Store(d, current)
			}
			return current, true
		}
		if r.contentReader != nil {
			if _, err := r.contentReader(candidate); err == nil {
				for _, d := range visited {
					r.crateRootCache.Store(d, current)
				}
				return current, true
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	for _, d := range visited {
		r.crateRootCache.Store(d, "")
	}
	return "", false
}

// rustCrateSourceDir returns the directory that `crate::` paths resolve
// against for sourceFile. That is usually <crateRoot>/src, but Cargo lets a
// [lib] or [[bin]] target set an arbitrary `path`, and then the crate's module
// tree is rooted at that file's directory instead. ripgrep declares
// `[[bin]] path = "crates/core/main.rs"` in its top-level manifest, so every
// `crate::` import under crates/core/ resolved against a src/ that does not
// exist.
func (r *ProjectImportResolver) rustCrateSourceDir(crateRoot, sourceFile string) string {
	defaultDir := filepath.Join(crateRoot, "src")
	for _, dir := range r.rustCustomTargetDirs(crateRoot) {
		if dir == defaultDir {
			continue
		}
		if rel, err := filepath.Rel(dir, sourceFile); err == nil &&
			rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return dir
		}
	}
	return defaultDir
}

// rustCustomTargetDirs lists the source directories of explicitly declared
// [lib] / [[bin]] targets, most specific first so a nested target wins over an
// enclosing one.
func (r *ProjectImportResolver) rustCustomTargetDirs(crateRoot string) []string {
	if cached, ok := r.targetDirsCache.Load(crateRoot); ok {
		return cached.([]string)
	}

	var dirs []string
	if r.contentReader != nil {
		if content, err := r.contentReader(filepath.Join(crateRoot, "Cargo.toml")); err == nil {
			for _, rel := range parseRustTargetPaths(string(content)) {
				dirs = append(dirs, filepath.Dir(filepath.Join(crateRoot, rel)))
			}
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })

	r.targetDirsCache.Store(crateRoot, dirs)
	return dirs
}

// parseRustTargetPaths pulls the `path = "..."` of every [lib] and [[bin]]
// section out of a Cargo.toml. Only those sections are considered — a `path`
// under [dependencies.foo] means something else entirely.
func parseRustTargetPaths(content string) []string {
	var paths []string
	inTargetSection := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			section := strings.Trim(trimmed, "[]")
			inTargetSection = section == "lib" || section == "bin"
			continue
		}
		if !inTargetSection {
			continue
		}
		key, value, found := strings.Cut(trimmed, "=")
		if !found || strings.TrimSpace(key) != "path" {
			continue
		}
		if p := strings.Trim(strings.TrimSpace(value), `"'`); p != "" {
			paths = append(paths, filepath.FromSlash(p))
		}
	}
	return paths
}

func (r *ProjectImportResolver) isLocalRustCrateImport(firstSegment, crateRoot string) bool {
	if firstSegment == "" || crateRoot == "" {
		return false
	}
	if cached, ok := r.crateNameCache.Load(crateRoot); ok {
		return cached.(map[string]bool)[firstSegment]
	}

	names := make(map[string]bool)
	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	if r.contentReader != nil {
		if content, err := r.contentReader(cargoToml); err == nil {
			names = parseRustCrateNamesFromCargoToml(string(content))
		}
	}
	r.crateNameCache.Store(crateRoot, names)
	return names[firstSegment]
}

func (r *ProjectImportResolver) resolveRustDependencyCrateRoot(importName, crateRoot string) (string, bool) {
	if importName == "" || crateRoot == "" {
		return "", false
	}

	depCrateRoots := r.dependencyCrateRoots(crateRoot)
	depCrateRoot, ok := depCrateRoots[importName]
	return depCrateRoot, ok
}

func (r *ProjectImportResolver) dependencyCrateRoots(crateRoot string) map[string]string {
	if cached, ok := r.depCrateRootsCache.Load(crateRoot); ok {
		return cached.(map[string]string)
	}

	result := make(map[string]string)
	if r.contentReader == nil {
		r.depCrateRootsCache.Store(crateRoot, result)
		return result
	}

	// 1. Cargo workspace lookup. Modern Rust workspaces declare dependencies
	// centrally in `[workspace.dependencies]` and members refer to them via
	// `{ workspace = true }`. Without this lookup, every cross-crate edge in
	// a workspace-style monorepo (uv, ruff, deno, oxc, bevy, …) is invisible.
	if ws := r.loadCargoWorkspaceFor(crateRoot); ws != nil {
		for name, dir := range ws.crates {
			result[name] = dir
		}
	}

	// 2. Per-crate `[dependencies]` path entries. These take precedence over
	// the workspace mapping for the rare cases where a member crate overrides
	// a workspace dependency.
	cargoTomlPath := filepath.Join(crateRoot, "Cargo.toml")
	content, err := r.contentReader(cargoTomlPath)
	if err != nil {
		r.depCrateRootsCache.Store(crateRoot, result)
		return result
	}

	for _, dep := range parseRustPathDependencyEntries(string(content)) {
		depRoot := dep.path
		if !filepath.IsAbs(depRoot) {
			depRoot = filepath.Join(crateRoot, depRoot)
		}
		depRoot = filepath.Clean(depRoot)

		for _, importName := range dep.importNames {
			if importName != "" {
				result[importName] = depRoot
			}
		}

		depCargoTomlPath := filepath.Join(depRoot, "Cargo.toml")
		depContent, depErr := r.contentReader(depCargoTomlPath)
		if depErr != nil {
			continue
		}
		for name := range parseRustCrateNamesFromCargoToml(string(depContent)) {
			result[name] = depRoot
		}
	}

	r.depCrateRootsCache.Store(crateRoot, result)
	return result
}

func (r *ProjectImportResolver) expandRustModRsCandidates(candidates []string) []string {
	if len(candidates) == 0 || r.contentReader == nil {
		return candidates
	}

	expanded := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if filepath.Base(candidate) != "mod.rs" {
			expanded = append(expanded, candidate)
			continue
		}

		// Keep the mod.rs itself as well as its children. A bare
		// `use crate::config;` names the module file directly, and the items
		// reached through it (`config::db_path()`) are as likely to be defined
		// in mod.rs as re-exported from a child. Dropping mod.rs here pointed
		// the edge at a child that defines nothing the importer uses, while
		// omitting the file that does.
		expanded = append(expanded, candidate)
		expanded = append(expanded, r.expandRustModRsDependencies(candidate)...)
	}

	return expanded
}

func (r *ProjectImportResolver) expandRustModRsDependencies(modRsPath string) []string {
	if cached, ok := r.modDepsCache.Load(modRsPath); ok {
		return cached.([]string)
	}

	imports, err := r.importsForFile(modRsPath)
	if err != nil {
		r.modDepsCache.Store(modRsPath, []string{})
		return nil
	}

	resolved := make([]string, 0, len(imports))
	for _, imp := range imports {
		if imp.Kind != RustImportModDecl {
			continue
		}
		resolved = append(resolved, resolveRustModDecl(modRsPath, imp.Path, r.suppliedFiles)...)
	}
	resolved = deduplicateSuppliedFiles(resolved, r.suppliedFiles)
	r.modDepsCache.Store(modRsPath, resolved)
	return resolved
}

func (r *ProjectImportResolver) importsForFile(path string) ([]RustImport, error) {
	if cached, ok := r.importsCache.Load(path); ok {
		return cached.([]RustImport), nil
	}
	if r.contentReader == nil {
		return nil, fmt.Errorf("content reader is required")
	}

	content, err := r.contentReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	imports, parseErr := ParseRustImports(content)
	if parseErr != nil {
		return nil, parseErr
	}

	r.importsCache.Store(path, imports)
	return imports, nil
}

func parseRustCrateNamesFromCargoToml(content string) map[string]bool {
	names := make(map[string]bool)
	section := ""
	packageName := ""
	libName := ""

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}

		if !strings.HasPrefix(line, "name") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")
		if value == "" {
			continue
		}
		switch section {
		case "package":
			packageName = value
		case "lib":
			libName = value
		}
	}

	if libName != "" {
		names[libName] = true
	}
	if packageName != "" {
		names[normalizeCargoCrateName(packageName)] = true
	}
	return names
}

type rustPathDependencyEntry struct {
	importNames []string
	path        string
}

func parseRustPathDependencyEntries(content string) []rustPathDependencyEntry {
	scanner := bufio.NewScanner(strings.NewReader(content))
	section := ""
	entries := []rustPathDependencyEntry{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		if !isRustDependencySection(section) {
			continue
		}

		key, value, ok := parseTomlKeyValue(line)
		if !ok {
			continue
		}
		if !strings.HasPrefix(value, "{") {
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

func isRustDependencySection(section string) bool {
	if section == "dependencies" || section == "dev-dependencies" || section == "build-dependencies" {
		return true
	}
	return strings.HasPrefix(section, "target.") && strings.HasSuffix(section, ".dependencies")
}

func parseTomlKeyValue(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func parseTomlInlineString(value, field string) string {
	idx := strings.Index(value, field)
	if idx < 0 {
		return ""
	}
	remainder := value[idx+len(field):]
	eqIdx := strings.Index(remainder, "=")
	if eqIdx < 0 {
		return ""
	}
	remainder = strings.TrimSpace(remainder[eqIdx+1:])
	return trimQuotes(remainder)
}

func trimQuotes(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ",")
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "\"") && strings.Contains(trimmed[1:], "\"") {
		trimmed = trimmed[1:]
		if end := strings.Index(trimmed, "\""); end >= 0 {
			return trimmed[:end]
		}
	}
	return strings.Trim(trimmed, "\"")
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func normalizeCargoCrateName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

func isRustDirectoryModuleFile(path string) bool {
	base := filepath.Base(path)
	return base == "mod.rs" || base == "lib.rs" || base == "main.rs"
}

// rustModuleOwnedDir returns the directory whose child modules belong to
// moduleFile. The two layouts differ: `foo/mod.rs` owns the directory it sits
// in, while `foo.rs` owns the sibling `foo/` directory. Using filepath.Dir for
// both resolves `foo.rs`'s children one level too high, so a relative path in
// a `pub use inner::Thing;` re-export is looked up beside foo.rs instead of
// inside foo/.
func rustModuleOwnedDir(moduleFile string) string {
	if isRustDirectoryModuleFile(moduleFile) {
		return filepath.Dir(moduleFile)
	}
	return strings.TrimSuffix(moduleFile, ".rs")
}

func firstRustPathSegment(path string) string {
	if idx := strings.Index(path, "::"); idx >= 0 {
		return path[:idx]
	}
	return path
}

func filterSuppliedFiles(paths []string, suppliedFiles map[string]bool) []string {
	if len(paths) == 0 {
		return nil
	}
	var filtered []string
	for _, path := range paths {
		if suppliedFiles[path] {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func deduplicateSuppliedFiles(paths []string, suppliedFiles map[string]bool) []string {
	if len(paths) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for _, path := range paths {
		if !suppliedFiles[path] {
			continue
		}
		if !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	return result
}
