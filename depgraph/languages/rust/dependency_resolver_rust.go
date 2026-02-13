package rust

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveRustProjectImports(
	absPath string,
	filePath string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, parseErr := ParseRustImports(content)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	var projectImports []string
	for _, imp := range imports {
		switch imp.Kind {
		case RustImportUse:
			projectImports = append(projectImports, resolveRustUsePath(absPath, imp.Path, suppliedFiles, contentReader)...)
		case RustImportModDecl:
			projectImports = append(projectImports, resolveRustModDecl(absPath, imp.Path, suppliedFiles)...)
		case RustImportExternCrate:
			// External crate imports do not map to local project files.
		}
	}

	return projectImports, nil
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

func resolveRustUsePath(sourceFile, importPath string, suppliedFiles map[string]bool, contentReader vcs.ContentReader) []string {
	path := strings.TrimSpace(importPath)
	if path == "" {
		return nil
	}

	parts := strings.Split(path, "::")
	baseDir := ""
	crateRoot := ""
	rootedInLocalCrate := false

	switch parts[0] {
	case "crate":
		root, ok := findRustCrateRoot(sourceFile, suppliedFiles, contentReader)
		if !ok {
			return nil
		}
		crateRoot = root
		baseDir = filepath.Join(root, "src")
		rootedInLocalCrate = true
		parts = parts[1:]
	case "self", "super":
		baseDir = filepath.Dir(sourceFile)
		for len(parts) > 0 {
			switch parts[0] {
			case "self":
				parts = parts[1:]
			case "super":
				baseDir = filepath.Dir(baseDir)
				parts = parts[1:]
			default:
				goto resolved
			}
		}
	default:
		root, ok := findRustCrateRoot(sourceFile, suppliedFiles, contentReader)
		if !ok || !isLocalRustCrateImport(parts[0], root, contentReader) {
			// Likely external crate or standard library.
			return nil
		}
		crateRoot = root
		baseDir = filepath.Join(root, "src")
		rootedInLocalCrate = true
		parts = parts[1:]
	}

resolved:
	if len(parts) == 0 && rootedInLocalCrate {
		return resolveRustCrateRootCandidates(crateRoot, suppliedFiles)
	}
	if len(parts) == 0 {
		return nil
	}

	candidates := resolveRustModuleCandidates(baseDir, parts, suppliedFiles)
	if len(parts) > 1 {
		candidates = append(candidates, resolveRustModuleCandidates(baseDir, parts[:len(parts)-1], suppliedFiles)...)
	}
	if rootedInLocalCrate && len(parts) == 1 {
		candidates = append(candidates, resolveRustCrateRootCandidates(crateRoot, suppliedFiles)...)
	}

	return deduplicateSuppliedFiles(candidates, suppliedFiles)
}

func resolveRustCrateRootCandidates(crateRoot string, suppliedFiles map[string]bool) []string {
	if crateRoot == "" {
		return nil
	}
	return filterSuppliedFiles([]string{filepath.Join(crateRoot, "src", "lib.rs")}, suppliedFiles)
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

func findRustCrateRoot(sourceFile string, suppliedFiles map[string]bool, contentReader vcs.ContentReader) (string, bool) {
	dir := filepath.Dir(sourceFile)
	for {
		candidate := filepath.Join(dir, "Cargo.toml")
		if suppliedFiles[candidate] {
			return dir, true
		}
		if contentReader != nil {
			if _, err := contentReader(candidate); err == nil {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func isLocalRustCrateImport(firstSegment, crateRoot string, contentReader vcs.ContentReader) bool {
	if firstSegment == "" || crateRoot == "" || contentReader == nil {
		return false
	}
	cargoToml := filepath.Join(crateRoot, "Cargo.toml")
	content, err := contentReader(cargoToml)
	if err != nil {
		return false
	}
	names := parseRustCrateNamesFromCargoToml(string(content))
	return names[firstSegment]
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

func normalizeCargoCrateName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
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
