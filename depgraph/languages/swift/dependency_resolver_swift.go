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
	typeReferences := ExtractSwiftReferencedSymbols(content)
	qualifiedReferences := ExtractSwiftQualifiedReferences(content)
	sourceTypes := make(map[string]bool)
	for _, name := range ParseSwiftTopLevelTypeNames(content) {
		sourceTypes[name] = true
	}
	for _, name := range ParseSwiftExtensionOwners(content) {
		sourceTypes[name] = true
	}
	if len(typeReferences) == 0 && len(qualifiedReferences) == 0 {
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
	extensionIndex := make(map[string][]SwiftExtensionMember)
	visitedModules := make(map[string]bool)

	if moduleName := swiftModuleFromPath(absPath); moduleName != "" {
		visitedModules[moduleName] = true
		projectImports = append(projectImports, resolveSwiftModuleImport(
			absPath,
			moduleName,
			moduleIndex,
			typeReferenceSet,
			typeIndex,
			qualifiedReferences,
			sourceTypes,
			extensionIndex,
			contentReader)...)
	} else {
		projectImports = append(projectImports, resolveSwiftCandidatesByTypeReferences(
			absPath,
			allSwiftCandidates(suppliedFiles),
			typeReferenceSet,
			typeIndex,
			qualifiedReferences,
			sourceTypes,
			extensionIndex,
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
			qualifiedReferences,
			sourceTypes,
			extensionIndex,
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
	qualifiedReferences []SwiftQualifiedReference,
	sourceTypes map[string]bool,
	extensionIndex map[string][]SwiftExtensionMember,
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
		qualifiedReferences,
		sourceTypes,
		extensionIndex,
		contentReader)
}

func resolveSwiftCandidatesByTypeReferences(
	sourceFile string,
	candidates []string,
	typeReferences map[string]bool,
	typeIndex map[string][]string,
	qualifiedReferences []SwiftQualifiedReference,
	sourceTypes map[string]bool,
	extensionIndex map[string][]SwiftExtensionMember,
	contentReader vcs.ContentReader,
) []string {
	var resolved []string
	for _, path := range candidates {
		if path == sourceFile {
			continue
		}
		if fileDeclaresReferencedType(
			path,
			typeReferences,
			typeIndex,
			qualifiedReferences,
			sourceTypes,
			extensionIndex,
			contentReader) {
			resolved = append(resolved, path)
		}
	}
	return resolved
}

func fileDeclaresReferencedType(
	filePath string,
	typeReferences map[string]bool,
	typeIndex map[string][]string,
	qualifiedReferences []SwiftQualifiedReference,
	sourceTypes map[string]bool,
	extensionIndex map[string][]SwiftExtensionMember,
	contentReader vcs.ContentReader,
) bool {
	if _, ok := typeIndex[filePath]; !ok {
		content, err := contentReader(filePath)
		if err != nil {
			typeIndex[filePath] = nil
		} else {
			typeIndex[filePath] = ParseSwiftTopLevelSymbolNames(content)
		}
	}

	for _, declared := range typeIndex[filePath] {
		if typeReferences[declared] {
			return true
		}
	}
	if _, ok := extensionIndex[filePath]; !ok {
		content, err := contentReader(filePath)
		if err != nil {
			extensionIndex[filePath] = nil
		} else {
			extensionIndex[filePath] = ParseSwiftExtensionMembers(content)
		}
	}
	for _, ref := range qualifiedReferences {
		for _, member := range extensionIndex[filePath] {
			ownerMatches := ref.Qualifier == member.Owner ||
				(ref.Qualifier == "Self" && sourceTypes[member.Owner])
			if ownerMatches && ref.Member == member.Member {
				return true
			}
		}
	}
	for _, member := range extensionIndex[filePath] {
		if sourceTypes[member.Owner] && typeReferences[member.Member] {
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
