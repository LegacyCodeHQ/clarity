package java

import (
	"fmt"
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// BuildJavaIndices builds package and type indices for supplied Java files.
func BuildJavaIndices(
	javaFiles []string,
	contentReader vcs.ContentReader,
) (map[string][]string, map[string]map[string][]string, map[string]string) {
	if len(javaFiles) == 0 {
		return nil, nil, make(map[string]string)
	}

	packageToFiles := make(map[string][]string)
	packageToTypes := make(map[string]map[string][]string)
	fileToPackage := make(map[string]string)

	for _, filePath := range javaFiles {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			continue
		}

		content, err := contentReader(absPath)
		if err != nil {
			continue
		}

		pkg := ParsePackageDeclaration(content)
		if pkg == "" {
			continue
		}
		fileToPackage[absPath] = pkg

		packageToFiles[pkg] = append(packageToFiles[pkg], absPath)

		declaredTypes := ParseTopLevelTypeNames(content)
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

	return packageToFiles, packageToTypes, fileToPackage
}

// ResolveJavaProjectImports resolves Java project imports for a single file.
func ResolveJavaProjectImports(
	absPath string,
	_ string,
	javaPackageIndex map[string][]string,
	javaPackageTypes map[string]map[string][]string,
	javaFilePackages map[string]string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	projectPackages := make(map[string]bool, len(javaPackageIndex))
	for pkg := range javaPackageIndex {
		projectPackages[pkg] = true
	}

	imports := ParseJavaImports(content, projectPackages)
	typeReferences := ExtractTypeIdentifiers(content)
	declaredNames := make(map[string]bool)
	for _, name := range ParseTopLevelTypeNames(content) {
		if name != "" {
			declaredNames[name] = true
		}
	}
	projectImports := make([]string, 0, len(imports))
	for _, imp := range imports {
		internalImp, ok := imp.(InternalImport)
		if !ok {
			continue
		}
		projectImports = append(projectImports, resolveJavaImportPath(
			absPath,
			internalImp,
			javaPackageIndex,
			javaPackageTypes,
			suppliedFiles,
			typeReferences,
			declaredNames)...)
	}

	samePackageDeps := resolveJavaSamePackageDependencies(
		absPath,
		content,
		javaFilePackages,
		javaPackageTypes,
		imports,
		suppliedFiles)
	projectImports = append(projectImports, samePackageDeps...)

	return projectImports, nil
}

func resolveJavaImportPath(
	sourceFile string,
	imp InternalImport,
	packageIndex map[string][]string,
	packageTypeIndex map[string]map[string][]string,
	suppliedFiles map[string]bool,
	typeReferences []string,
	declaredNames map[string]bool,
) []string {
	pkg := imp.Package()
	resolved := []string{}
	seen := make(map[string]bool)

	addFile := func(path string) {
		if path == sourceFile || !suppliedFiles[path] || seen[path] {
			return
		}
		seen[path] = true
		resolved = append(resolved, path)
	}

	if imp.IsWildcard() {
		typeMap, ok := packageTypeIndex[pkg]
		if !ok {
			return resolved
		}
		for _, ref := range typeReferences {
			if declaredNames[ref] {
				continue
			}
			for _, file := range typeMap[ref] {
				addFile(file)
			}
		}
		return resolved
	}

	typeName := simpleTypeName(imp.Path())
	if typeName != "" {
		if typeMap, ok := packageTypeIndex[pkg]; ok {
			for _, file := range typeMap[typeName] {
				addFile(file)
			}
		}
	}

	if len(resolved) == 0 {
		for _, file := range packageIndex[pkg] {
			addFile(file)
		}
	}

	return resolved
}

func resolveJavaSamePackageDependencies(
	sourceFile string,
	sourceContent []byte,
	filePackages map[string]string,
	packageTypeIndex map[string]map[string][]string,
	imports []JavaImport,
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

	typeReferences := ExtractTypeIdentifiers(sourceContent)
	if len(typeReferences) == 0 {
		return []string{}
	}

	importedNames := make(map[string]bool)
	for _, imp := range imports {
		if imp.IsWildcard() {
			continue
		}
		name := simpleTypeName(imp.Path())
		if name != "" {
			importedNames[name] = true
		}
	}

	declaredNames := make(map[string]bool)
	for _, name := range ParseTopLevelTypeNames(sourceContent) {
		if name != "" {
			declaredNames[name] = true
		}
	}

	seen := make(map[string]bool)
	deps := []string{}
	for _, ref := range typeReferences {
		if importedNames[ref] || declaredNames[ref] {
			continue
		}
		files, ok := typeIndex[ref]
		if !ok {
			continue
		}
		for _, depFile := range files {
			if depFile == sourceFile || !suppliedFiles[depFile] || seen[depFile] {
				continue
			}
			seen[depFile] = true
			deps = append(deps, depFile)
		}
	}

	return deps
}
