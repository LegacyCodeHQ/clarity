package rust

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

type ProjectImportResolver struct {
	suppliedFiles map[string]bool
	contentReader vcs.ContentReader

	crateRootCache     sync.Map // directory path -> crate root (or "")
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
		baseDir = filepath.Join(root, "src")
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
			baseDir = filepath.Join(root, "src")
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
	if len(parts) > 1 && len(candidates) == 0 {
		candidates = append(candidates, resolveRustModuleCandidates(baseDir, parts[:len(parts)-1], r.suppliedFiles)...)
	}
	if rootedInLocalCrate && len(parts) == 1 && len(candidates) == 0 {
		candidates = append(candidates, resolveRustCrateRootCandidates(crateRoot, r.suppliedFiles)...)
	}

	candidates = deduplicateSuppliedFiles(candidates, r.suppliedFiles)
	candidates = r.expandRustModRsCandidates(candidates)
	return deduplicateSuppliedFiles(candidates, r.suppliedFiles)
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
	if isRustDirectoryModuleFile(sourceFile) {
		return nil
	}
	parts := strings.Split(path, "::")
	if len(parts) == 0 || parts[0] == "" {
		return nil
	}
	base := filepath.Base(sourceFile)
	stem := strings.TrimSuffix(base, ".rs")
	if stem == "" || stem == base {
		return nil
	}
	siblingDir := filepath.Join(filepath.Dir(sourceFile), stem)
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

		modChildren := r.expandRustModRsDependencies(candidate)
		if len(modChildren) == 0 {
			expanded = append(expanded, candidate)
			continue
		}
		expanded = append(expanded, modChildren...)
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
