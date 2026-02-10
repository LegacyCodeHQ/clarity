package kotlin

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func ResolveKotlinProjectImports(
	absPath string,
	filePath string,
	kotlinPackageIndex map[string][]string,
	kotlinPackageTypes map[string]map[string][]string,
	kotlinFilePackages map[string]string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, err := ParseKotlinImports(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	projectPackages := make(map[string]bool)
	for pkg := range kotlinPackageIndex {
		projectPackages[pkg] = true
	}

	imports = ClassifyWithProjectPackages(imports, projectPackages)
	typeReferences := ExtractTypeIdentifiers(content)
	referencedTypes := make(map[string]bool, len(typeReferences))
	for _, ref := range typeReferences {
		if ref != "" {
			referencedTypes[ref] = true
		}
	}

	var projectImports []string
	for _, imp := range imports {
		if internalImp, ok := imp.(InternalImport); ok {
			resolvedFiles := resolveKotlinImportPath(absPath, internalImp, kotlinPackageTypes, referencedTypes, suppliedFiles)
			projectImports = append(projectImports, resolvedFiles...)
		}
	}

	if len(kotlinPackageTypes) > 0 {
		samePackageDeps := resolveKotlinSamePackageDependencies(
			absPath,
			contentReader,
			kotlinFilePackages,
			kotlinPackageTypes,
			imports,
			suppliedFiles)
		projectImports = append(projectImports, samePackageDeps...)
	}

	return projectImports, nil
}

func BuildKotlinIndices(
	kotlinFiles []string,
	contentReader vcs.ContentReader,
) (map[string][]string, map[string]map[string][]string, map[string]string) {
	if len(kotlinFiles) == 0 {
		return nil, nil, make(map[string]string)
	}

	kotlinPackageIndex, kotlinPackageTypes := buildKotlinPackageIndex(kotlinFiles, contentReader)
	kotlinFilePackages := make(map[string]string)
	for pkg, files := range kotlinPackageIndex {
		for _, file := range files {
			kotlinFilePackages[file] = pkg
		}
	}

	return kotlinPackageIndex, kotlinPackageTypes, kotlinFilePackages
}

func buildKotlinPackageIndex(filePaths []string, contentReader vcs.ContentReader) (map[string][]string, map[string]map[string][]string) {
	packageToFiles := make(map[string][]string)
	packageToTypes := make(map[string]map[string][]string)

	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			continue
		}

		content, err := contentReader(absPath)
		if err != nil {
			continue
		}

		pkg := ExtractPackageDeclaration(content)
		if pkg == "" {
			continue
		}

		packageToFiles[pkg] = append(packageToFiles[pkg], absPath)

		declaredTypes := ExtractTopLevelTypeNames(content)
		if len(declaredTypes) == 0 {
			continue
		}

		typeMap, ok := packageToTypes[pkg]
		if !ok {
			typeMap = make(map[string][]string)
			packageToTypes[pkg] = typeMap
		}

		for _, typeName := range declaredTypes {
			if typeName == "" {
				continue
			}
			typeMap[typeName] = append(typeMap[typeName], absPath)
		}
	}

	return packageToFiles, packageToTypes
}

// resolveKotlinImportPath resolves Kotlin imports strictly by referenced symbols.
func resolveKotlinImportPath(
	sourceFile string,
	imp KotlinImport,
	packageTypeIndex map[string]map[string][]string,
	referencedTypes map[string]bool,
	suppliedFiles map[string]bool,
) []string {
	if len(referencedTypes) == 0 {
		return nil
	}

	var resolvedFiles []string
	seen := make(map[string]bool)

	appendResolvedFiles := func(files []string) {
		for _, file := range files {
			if file == sourceFile || !suppliedFiles[file] || seen[file] {
				continue
			}
			seen[file] = true
			resolvedFiles = append(resolvedFiles, file)
		}
	}

	resolveReferencedTypes := func(typeMap map[string][]string) {
		if len(typeMap) == 0 {
			return
		}
		for ref := range referencedTypes {
			files := typeMap[ref]
			if len(files) != 1 {
				continue
			}
			appendResolvedFiles(files)
		}
	}

	if imp.IsWildcard() {
		// Wildcard import: only add files for actually referenced type symbols.
		if typeMap, ok := packageTypeIndex[imp.Package()]; ok {
			resolveReferencedTypes(typeMap)
		}
	} else {
		pkg := imp.Package()
		symbol := extractSimpleName(imp.Path())
		if !referencedTypes[symbol] {
			return resolvedFiles
		}
		if typeMap, ok := packageTypeIndex[pkg]; ok {
			if files, ok := typeMap[symbol]; ok {
				if len(files) != 1 {
					return resolvedFiles
				}
				appendResolvedFiles(files)
			}
		}
	}

	return resolvedFiles
}

// resolveKotlinSamePackageDependencies finds Kotlin dependencies that are referenced without imports (same-package references)
func resolveKotlinSamePackageDependencies(
	sourceFile string,
	contentReader vcs.ContentReader,
	filePackages map[string]string,
	packageTypeIndex map[string]map[string][]string,
	imports []KotlinImport,
	suppliedFiles map[string]bool,
) []string {
	pkg, ok := filePackages[sourceFile]
	if !ok {
		return []string{}
	}

	typeIndex, ok := packageTypeIndex[pkg]
	if !ok {
		return []string{}
	}

	sourceCode, err := contentReader(sourceFile)
	if err != nil {
		return []string{}
	}

	typeReferences := ExtractTypeIdentifiers(sourceCode)
	if len(typeReferences) == 0 {
		return []string{}
	}
	declaredTypes := ExtractTopLevelTypeNames(sourceCode)
	declaredTypeSet := make(map[string]bool, len(declaredTypes))
	for _, typeName := range declaredTypes {
		if typeName != "" {
			declaredTypeSet[typeName] = true
		}
	}

	importedNames := make(map[string]bool)
	for _, imp := range imports {
		if imp.IsWildcard() {
			continue
		}
		name := extractSimpleName(imp.Path())
		if name != "" {
			importedNames[name] = true
		}
	}

	seen := make(map[string]bool)
	var deps []string
	for _, ref := range typeReferences {
		// Ignore references to top-level types declared in the same file.
		// This avoids linking sibling source-set files that declare the same
		// expect/actual type names in Kotlin Multiplatform projects.
		if declaredTypeSet[ref] {
			continue
		}
		if importedNames[ref] {
			continue
		}
		files, ok := typeIndex[ref]
		if !ok {
			continue
		}
		if len(files) != 1 {
			continue
		}
		for _, depFile := range files {
			if depFile == sourceFile {
				continue
			}
			if !suppliedFiles[depFile] {
				continue
			}
			if !seen[depFile] {
				seen[depFile] = true
				deps = append(deps, depFile)
			}
		}
	}

	return deps
}

// extractSimpleName returns the trailing identifier from a dot-delimited path
func extractSimpleName(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}
