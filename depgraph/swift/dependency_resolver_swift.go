package swift

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveSwiftProjectImports(
	absPath string,
	filePath string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, parseErr := ParseSwiftImports(content)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	moduleIndex := buildSwiftModuleIndex(suppliedFiles)
	typeReferences := ExtractSwiftTypeIdentifiers(content)
	if len(typeReferences) == 0 {
		return []string{}, nil
	}

	typeReferenceSet := make(map[string]bool, len(typeReferences))
	for _, name := range typeReferences {
		if name != "" {
			typeReferenceSet[name] = true
		}
	}

	var projectImports []string
	typeIndex := make(map[string][]string)
	visitedModules := make(map[string]bool)

	if moduleName := swiftModuleFromPath(absPath); moduleName != "" {
		visitedModules[moduleName] = true
		projectImports = append(projectImports, resolveSwiftModuleImport(
			absPath,
			moduleName,
			moduleIndex,
			typeReferenceSet,
			typeIndex,
			contentReader)...)
	} else {
		projectImports = append(projectImports, resolveSwiftCandidatesByTypeReferences(
			absPath,
			allSwiftCandidates(suppliedFiles),
			typeReferenceSet,
			typeIndex,
			contentReader)...)
	}

	for _, imp := range imports {
		moduleName := strings.TrimSpace(imp.Path)
		if moduleName == "" || visitedModules[moduleName] {
			continue
		}
		visitedModules[moduleName] = true
		projectImports = append(projectImports, resolveSwiftModuleImport(
			absPath,
			moduleName,
			moduleIndex,
			typeReferenceSet,
			typeIndex,
			contentReader)...)
	}

	return deduplicateSwiftPaths(projectImports), nil
}

func buildSwiftModuleIndex(suppliedFiles map[string]bool) map[string][]string {
	index := make(map[string][]string)
	for filePath, ok := range suppliedFiles {
		if !ok {
			continue
		}
		if filepath.Ext(filePath) != ".swift" {
			continue
		}
		module := swiftModuleFromPath(filePath)
		if module == "" {
			continue
		}
		index[module] = append(index[module], filePath)
	}
	return index
}

func resolveSwiftModuleImport(
	sourceFile string,
	moduleName string,
	moduleIndex map[string][]string,
	typeReferences map[string]bool,
	typeIndex map[string][]string,
	contentReader vcs.ContentReader,
) []string {
	if moduleName == "" {
		return nil
	}

	candidates := moduleIndex[moduleName]
	if len(candidates) == 0 {
		if strings.HasSuffix(moduleName, "Tests") {
			candidates = moduleIndex[strings.TrimSuffix(moduleName, "Tests")]
		} else if strings.HasSuffix(moduleName, "Test") {
			candidates = moduleIndex[strings.TrimSuffix(moduleName, "Test")]
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	return resolveSwiftCandidatesByTypeReferences(
		sourceFile,
		candidates,
		typeReferences,
		typeIndex,
		contentReader)
}

func resolveSwiftCandidatesByTypeReferences(
	sourceFile string,
	candidates []string,
	typeReferences map[string]bool,
	typeIndex map[string][]string,
	contentReader vcs.ContentReader,
) []string {
	var resolved []string
	for _, path := range candidates {
		if path == sourceFile {
			continue
		}
		if fileDeclaresReferencedType(path, typeReferences, typeIndex, contentReader) {
			resolved = append(resolved, path)
		}
	}
	return resolved
}

func fileDeclaresReferencedType(
	filePath string,
	typeReferences map[string]bool,
	typeIndex map[string][]string,
	contentReader vcs.ContentReader,
) bool {
	if _, ok := typeIndex[filePath]; !ok {
		content, err := contentReader(filePath)
		if err != nil {
			typeIndex[filePath] = nil
		} else {
			typeIndex[filePath] = ParseSwiftTopLevelTypeNames(content)
		}
	}

	for _, declared := range typeIndex[filePath] {
		if typeReferences[declared] {
			return true
		}
	}
	return false
}

func swiftModuleFromPath(filePath string) string {
	path := filepath.ToSlash(filePath)
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "Sources" || part == "Source" || part == "Tests" {
			if i+1 < len(parts) && parts[i+1] != "" {
				return parts[i+1]
			}
		}
	}
	return ""
}

func deduplicateSwiftPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}

func allSwiftCandidates(suppliedFiles map[string]bool) []string {
	candidates := make([]string, 0, len(suppliedFiles))
	for filePath, ok := range suppliedFiles {
		if !ok || filepath.Ext(filePath) != ".swift" {
			continue
		}
		candidates = append(candidates, filePath)
	}
	sort.Strings(candidates)
	return candidates
}
