package parsers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/sanity/git"
	"github.com/LegacyCodeHQ/sanity/parsers/dart"
	_go "github.com/LegacyCodeHQ/sanity/parsers/go"
	"github.com/LegacyCodeHQ/sanity/parsers/kotlin"
	"github.com/LegacyCodeHQ/sanity/parsers/typescript"
)

// DependencyGraph represents a mapping from file paths to their project dependencies
type DependencyGraph map[string][]string

// BuildDependencyGraph analyzes a list of files and builds a dependency graph
// containing only project imports (excluding package:/dart: imports for Dart,
// and standard library/external imports for Go).
// Only dependencies that are in the supplied file list are included in the graph.
// If repoPath and commitID are provided, files are read from the git commit instead of the filesystem.
func BuildDependencyGraph(filePaths []string, repoPath, commitID string) (DependencyGraph, error) {
	graph := make(DependencyGraph)

	// First pass: build a set of all supplied file paths (as absolute paths)
	// Also build a map from directories to files for Go package resolution
	// And collect Kotlin files for package indexing
	suppliedFiles := make(map[string]bool)
	dirToFiles := make(map[string][]string)
	kotlinFiles := []string{}
	goFiles := []string{}

	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
		}
		suppliedFiles[absPath] = true

		// Map directory to file for Go package imports
		dir := filepath.Dir(absPath)
		dirToFiles[dir] = append(dirToFiles[dir], absPath)

		// Collect Kotlin files for package indexing
		if filepath.Ext(absPath) == ".kt" {
			kotlinFiles = append(kotlinFiles, absPath)
		}

		// Collect Go files for export indexing
		if filepath.Ext(absPath) == ".go" {
			goFiles = append(goFiles, absPath)
		}
	}

	// Build Go package export indices for symbol-level cross-package resolution
	goPackageExportIndices := make(map[string]_go.GoPackageExportIndex) // packageDir -> export index
	for dir, files := range dirToFiles {
		// Check if this directory has Go files
		hasGoFiles := false
		var goFilesInDir []string
		for _, f := range files {
			if filepath.Ext(f) == ".go" {
				hasGoFiles = true
				goFilesInDir = append(goFilesInDir, f)
			}
		}
		if hasGoFiles {
			exportIndex, err := _go.BuildPackageExportIndex(goFilesInDir, repoPath, commitID)
			if err == nil {
				goPackageExportIndices[dir] = exportIndex
			}
		}
	}

	// Build Kotlin package index if we have Kotlin files
	var kotlinPackageIndex map[string][]string
	var kotlinPackageTypes map[string]map[string][]string
	kotlinFilePackages := make(map[string]string)
	if len(kotlinFiles) > 0 {
		kotlinPackageIndex, kotlinPackageTypes = buildKotlinPackageIndex(kotlinFiles, repoPath, commitID)
		for pkg, files := range kotlinPackageIndex {
			for _, file := range files {
				kotlinFilePackages[file] = pkg
			}
		}
	}

	// Second pass: build the dependency graph
	for _, filePath := range filePaths {
		// Get absolute path
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
		}

		ext := filepath.Ext(absPath)

		// Check if this is a supported file type
		if ext != ".dart" && ext != ".go" && ext != ".kt" && ext != ".ts" && ext != ".tsx" {
			// Unsupported files are included in the graph with no dependencies
			graph[absPath] = []string{}
			continue
		}

		// Parse imports based on file type
		var projectImports []string

		if ext == ".dart" {
			var imports []dart.Import
			var err error

			if repoPath != "" && commitID != "" {
				// Read file from git commit
				relPath := getRelativePath(absPath, repoPath)
				content, err := git.GetFileContentFromCommit(repoPath, commitID, relPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read %s from commit %s: %w", relPath, commitID, err)
				}
				imports, err = dart.ParseImports(content)
			} else {
				// Read file from filesystem
				imports, err = dart.Imports(filePath)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
			}

			// Filter for project imports only that are in the supplied file list
			for _, imp := range imports {
				if projImp, ok := imp.(dart.ProjectImport); ok {
					// Resolve relative path to absolute
					resolvedPath := resolveImportPath(absPath, projImp.URI(), ext)

					// Only include if the dependency is in the supplied files
					if suppliedFiles[resolvedPath] {
						projectImports = append(projectImports, resolvedPath)
					}
				}
			}
		} else if ext == ".go" {
			var imports []_go.GoImport
			var err error
			var sourceContent []byte

			if repoPath != "" && commitID != "" {
				// Read file from git commit
				relPath := getRelativePath(absPath, repoPath)
				sourceContent, err = git.GetFileContentFromCommit(repoPath, commitID, relPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read %s from commit %s: %w", relPath, commitID, err)
				}
				imports, err = _go.ParseGoImports(sourceContent)
			} else {
				// Read file from filesystem
				imports, err = _go.GoImports(filePath)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
			}

			// Extract export info for symbol-level cross-package resolution
			var exportInfo *_go.GoExportInfo
			if sourceContent != nil {
				exportInfo, _ = _go.ExtractGoExportInfoFromContent(absPath, sourceContent)
			} else {
				exportInfo, _ = _go.ExtractGoExportInfo(absPath)
			}

			// Determine if this is a test file
			isTestFile := strings.HasSuffix(absPath, "_test.go")

			// Filter for internal imports (including those that look external but are part of the module)
			for _, imp := range imports {
				var importPath string

				// Check both InternalImport and ExternalImport types
				// resolveGoImportPath will determine if they're actually part of this module
				switch typedImp := imp.(type) {
				case _go.InternalImport:
					importPath = typedImp.Path()
				case _go.ExternalImport:
					importPath = typedImp.Path()
				default:
					continue
				}

				// Resolve import path to package directory
				packageDir := resolveGoImportPath(absPath, importPath, repoPath, commitID)

				// Skip if packageDir is empty (means it's truly external or couldn't be resolved)
				if packageDir == "" {
					continue
				}

				// Check if we're importing from the same directory
				sourceDir := filepath.Dir(absPath)
				sameDir := sourceDir == packageDir

				// Get the export index for this package (if available)
				exportIndex, hasExportIndex := goPackageExportIndices[packageDir]

				// Get the symbols actually used from this import
				var usedSymbols map[string]bool
				if exportInfo != nil {
					usedSymbols = _go.GetUsedSymbolsFromPackage(exportInfo, importPath)
				}

				// Find files in the supplied list that are in this package
				if files, ok := dirToFiles[packageDir]; ok {
					for _, depFile := range files {
						// Don't add self-dependencies
						if depFile == absPath {
							continue
						}

						// Non-test files should not depend on test files from imported packages
						if !isTestFile && strings.HasSuffix(depFile, "_test.go") {
							continue
						}

						// If importing from same directory, only include .go files
						// If importing from different directory, include all files (including C files for CGo)
						if sameDir && filepath.Ext(depFile) != ".go" {
							continue
						}

						// Symbol-level filtering for cross-package imports (different directory)
						if !sameDir && hasExportIndex && usedSymbols != nil && len(usedSymbols) > 0 {
							// Only include this file if it defines a symbol we actually use
							fileDefinesUsedSymbol := false
							for symbol := range usedSymbols {
								if definingFiles, ok := exportIndex[symbol]; ok {
									for _, defFile := range definingFiles {
										if defFile == depFile {
											fileDefinesUsedSymbol = true
											break
										}
									}
								}
								if fileDefinesUsedSymbol {
									break
								}
							}

							if !fileDefinesUsedSymbol {
								continue
							}
						}

						projectImports = append(projectImports, depFile)
					}
				}
			}
		} else if ext == ".kt" {
			var imports []kotlin.KotlinImport
			var err error

			if repoPath != "" && commitID != "" {
				// Read file from git commit
				relPath := getRelativePath(absPath, repoPath)
				content, err := git.GetFileContentFromCommit(repoPath, commitID, relPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read %s from commit %s: %w", relPath, commitID, err)
				}
				imports, err = kotlin.ParseKotlinImports(content)
			} else {
				// Read file from filesystem
				imports, err = kotlin.KotlinImports(filePath)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
			}

			// Extract project packages for classification
			projectPackages := make(map[string]bool)
			for pkg := range kotlinPackageIndex {
				projectPackages[pkg] = true
			}

			// Reclassify imports with knowledge of project packages
			imports = kotlin.ClassifyWithProjectPackages(imports, projectPackages)

			// Filter for internal imports and resolve to files
			for _, imp := range imports {
				if internalImp, ok := imp.(kotlin.InternalImport); ok {
					// Resolve to file paths
					resolvedFiles := resolveKotlinImportPath(absPath, internalImp, kotlinPackageIndex, suppliedFiles)
					projectImports = append(projectImports, resolvedFiles...)
				}
			}

			if len(kotlinPackageTypes) > 0 {
				samePackageDeps := resolveKotlinSamePackageDependencies(
					absPath,
					repoPath,
					commitID,
					kotlinFilePackages,
					kotlinPackageTypes,
					imports,
					suppliedFiles,
				)
				projectImports = append(projectImports, samePackageDeps...)
			}
		} else if ext == ".ts" || ext == ".tsx" {
			var imports []typescript.TypeScriptImport
			var parseErr error

			if repoPath != "" && commitID != "" {
				// Read file from git commit
				relPath := getRelativePath(absPath, repoPath)
				content, err := git.GetFileContentFromCommit(repoPath, commitID, relPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read %s from commit %s: %w", relPath, commitID, err)
				}
				imports, parseErr = typescript.ParseTypeScriptImports(content, ext == ".tsx")
			} else {
				// Read file from filesystem
				imports, parseErr = typescript.TypeScriptImports(filePath)
			}

			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
			}

			// Filter for internal imports and resolve to files
			for _, imp := range imports {
				if internalImp, ok := imp.(typescript.InternalImport); ok {
					resolvedFiles := typescript.ResolveTypeScriptImportPath(absPath, internalImp.Path(), suppliedFiles)
					projectImports = append(projectImports, resolvedFiles...)
				}
			}
		}

		if len(projectImports) > 0 {
			projectImports = deduplicatePaths(projectImports)
		}

		graph[absPath] = projectImports
	}

	// Third pass: Add intra-package dependencies for Go files
	// This handles dependencies between files in the same package (which don't import each other)
	// Note: goFiles was already collected in the first pass

	if len(goFiles) > 0 {
		intraDeps, err := _go.BuildIntraPackageDependencies(goFiles, repoPath, commitID)
		if err != nil {
			// Don't fail if intra-package analysis fails, just skip it
			return graph, nil
		}

		// Merge intra-package dependencies into the graph
		for file, deps := range intraDeps {
			if existingDeps, ok := graph[file]; ok {
				// Combine and deduplicate
				depSet := make(map[string]bool)
				for _, dep := range existingDeps {
					depSet[dep] = true
				}
				for _, dep := range deps {
					depSet[dep] = true
				}

				// Convert back to slice
				merged := make([]string, 0, len(depSet))
				for dep := range depSet {
					merged = append(merged, dep)
				}
				graph[file] = merged
			}
		}
	}

	return graph, nil
}

// resolveImportPath converts a relative import URI to an absolute path
func resolveImportPath(sourceFile, importURI, fileExt string) string {
	// Get directory of source file
	sourceDir := filepath.Dir(sourceFile)

	// Resolve relative import
	absImport := filepath.Join(sourceDir, importURI)

	// Add file extension if not present
	if !strings.HasSuffix(absImport, fileExt) {
		absImport += fileExt
	}

	return filepath.Clean(absImport)
}

// resolveGoImportPath resolves a Go import path to an absolute file path
// If repoPath and commitID are provided, it reads go.mod from the commit; otherwise from filesystem
func resolveGoImportPath(sourceFile, importPath, repoPath, commitID string) string {
	// For Go files, we need to find the module root and resolve the import
	// This is a simplified version that assumes the project follows standard Go module structure

	// Find the go.mod file by walking up from the source file
	moduleRoot := findModuleRoot(filepath.Dir(sourceFile))
	if moduleRoot == "" {
		// If no module root found, return empty string
		return ""
	}

	// Get the module name from go.mod (from commit if analyzing a commit, otherwise from filesystem)
	var moduleName string
	if repoPath != "" && commitID != "" {
		moduleName = getModuleNameFromCommit(repoPath, commitID, moduleRoot)
	} else {
		moduleName = getModuleName(moduleRoot)
	}

	if moduleName == "" {
		return ""
	}

	// Check if the import path starts with the module name
	if !strings.HasPrefix(importPath, moduleName) {
		// Not an internal import relative to this module
		return ""
	}

	// Remove module name prefix to get relative path
	relativePath := strings.TrimPrefix(importPath, moduleName+"/")

	// Construct absolute path
	absPath := filepath.Join(moduleRoot, relativePath)

	// For Go, we don't add .go extension here because imports refer to packages (directories)
	// We'll need to look for any .go file in that directory
	// For now, we'll return the directory path
	return filepath.Clean(absPath)
}

// findModuleRoot walks up the directory tree to find the go.mod file
func findModuleRoot(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			return ""
		}
		dir = parent
	}
}

// getModuleName reads the module name from go.mod
func getModuleName(moduleRoot string) string {
	goModPath := filepath.Join(moduleRoot, "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}

	return ""
}

// getModuleNameFromCommit reads the module name from go.mod at a specific commit
func getModuleNameFromCommit(repoPath, commitID, moduleRoot string) string {
	// Get absolute repo path
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return ""
	}

	// Get relative path from repo root to module root
	relPath, err := filepath.Rel(absRepoPath, moduleRoot)
	if err != nil {
		// If moduleRoot is not under repoPath, try reading from the commit root
		relPath = ""
	}

	// Construct path to go.mod in the commit
	var goModPath string
	if relPath != "" && relPath != "." {
		goModPath = filepath.Join(relPath, "go.mod")
	} else {
		goModPath = "go.mod"
	}

	// Read go.mod from the commit
	content, err := git.GetFileContentFromCommit(repoPath, commitID, goModPath)
	if err != nil {
		return ""
	}

	// Parse the module name from the content
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}

	return ""
}

// getRelativePath converts an absolute file path to a path relative to the repository root
func getRelativePath(absPath, repoPath string) string {
	// Get absolute repository path
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		// If we can't get absolute path, try relative path as-is
		relPath, err := filepath.Rel(repoPath, absPath)
		if err != nil {
			// Fallback to using the absolute path
			return absPath
		}
		return relPath
	}

	// Get path relative to repository root
	relPath, err := filepath.Rel(absRepoPath, absPath)
	if err != nil {
		// Fallback to using the absolute path
		return absPath
	}

	return relPath
}

// buildKotlinPackageIndex builds maps describing available Kotlin packages and their type declarations
func buildKotlinPackageIndex(filePaths []string, repoPath, commitID string) (map[string][]string, map[string]map[string][]string) {
	packageToFiles := make(map[string][]string)
	packageToTypes := make(map[string]map[string][]string)

	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			continue
		}

		content, err := readFileContent(absPath, repoPath, commitID)
		if err != nil {
			continue
		}

		pkg := kotlin.ExtractPackageDeclaration(content)
		if pkg == "" {
			continue
		}

		packageToFiles[pkg] = append(packageToFiles[pkg], absPath)

		declaredTypes := kotlin.ExtractTopLevelTypeNames(content)
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
	imp kotlin.KotlinImport,
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
	repoPath string,
	commitID string,
	filePackages map[string]string,
	packageTypeIndex map[string]map[string][]string,
	imports []kotlin.KotlinImport,
	suppliedFiles map[string]bool,
) []string {
	pkg, ok := filePackages[sourceFile]
	if !ok {
		return nil
	}

	typeIndex, ok := packageTypeIndex[pkg]
	if !ok {
		return nil
	}

	sourceCode, err := readFileContent(sourceFile, repoPath, commitID)
	if err != nil {
		return nil
	}

	typeReferences := kotlin.ExtractTypeIdentifiers(sourceCode)
	if len(typeReferences) == 0 {
		return nil
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

// readFileContent reads a file either from the working tree or a specific git commit
func readFileContent(absPath, repoPath, commitID string) ([]byte, error) {
	if repoPath != "" && commitID != "" {
		relPath := getRelativePath(absPath, repoPath)
		return git.GetFileContentFromCommit(repoPath, commitID, relPath)
	}

	return os.ReadFile(absPath)
}

// extractSimpleName returns the trailing identifier from a dot-delimited path
func extractSimpleName(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

// deduplicatePaths removes duplicate entries while preserving insertion order
func deduplicatePaths(paths []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}
