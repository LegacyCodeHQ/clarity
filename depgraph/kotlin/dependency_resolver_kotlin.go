package kotlin

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/sanity/vcs"
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

	var projectImports []string
	for _, imp := range imports {
		if internalImp, ok := imp.(InternalImport); ok {
			resolvedFiles := resolveKotlinImportPath(absPath, internalImp, kotlinPackageIndex, suppliedFiles)
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

// resolveKotlinImportPath resolves a Kotlin import to absolute file paths
func resolveKotlinImportPath(
	sourceFile string,
	imp KotlinImport,
	packageIndex map[string][]string,
	suppliedFiles map[string]bool,
) []string {
	var resolvedFiles []string

	if imp.IsWildcard() {
		// Wildcard: find all files in the package
		pkg := imp.Package()
		if files, ok := packageIndex[pkg]; ok {
			for _, file := range files {
				if file != sourceFile && suppliedFiles[file] {
					resolvedFiles = append(resolvedFiles, file)
				}
			}
		}
	} else {
		// Specific import: find files in the package
		pkg := imp.Package()
		if files, ok := packageIndex[pkg]; ok {
			for _, file := range files {
				if file != sourceFile && suppliedFiles[file] {
					resolvedFiles = append(resolvedFiles, file)
				}
			}
		}

		// Also check if the full import path is a package
		fullPath := imp.Path()
		if fullPath != pkg {
			if files, ok := packageIndex[fullPath]; ok {
				for _, file := range files {
					if file != sourceFile && suppliedFiles[file] {
						resolvedFiles = append(resolvedFiles, file)
					}
				}
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
		if importedNames[ref] {
			continue
		}
		files, ok := typeIndex[ref]
		if !ok {
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
