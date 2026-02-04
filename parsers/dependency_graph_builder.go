package parsers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/sanity/parsers/dart"
	_go "github.com/LegacyCodeHQ/sanity/parsers/go"
	"github.com/LegacyCodeHQ/sanity/parsers/kotlin"
	"github.com/LegacyCodeHQ/sanity/parsers/typescript"
	"github.com/LegacyCodeHQ/sanity/vcs"
)

// BuildDependencyGraph analyzes a list of files and builds a dependency graph
// containing only project imports (excluding package:/dart: imports for Dart,
// and standard library/external imports for Go).
// Only dependencies that are in the supplied file list are included in the graph.
// The contentReader function is used to read file contents (from filesystem, git commit, etc.)
func BuildDependencyGraph(filePaths []string, contentReader vcs.ContentReader) (DependencyGraph, error) {
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
			exportIndex, err := _go.BuildPackageExportIndex(goFilesInDir, vcs.ContentReader(contentReader))
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
		kotlinPackageIndex, kotlinPackageTypes = buildKotlinPackageIndex(kotlinFiles, contentReader)
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
			content, err := contentReader(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
			}

			imports, err := dart.ParseImports(content)
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
			sourceContent, err := contentReader(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
			}

			imports, err := _go.ParseGoImports(sourceContent)
			if err != nil {
				return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
			}

			// Parse //go:embed directives
			embeds, _ := _go.ParseGoEmbeds(sourceContent)
			for _, embed := range embeds {
				// Resolve embed pattern relative to the source file's directory
				embedPath := resolveGoEmbedPath(absPath, embed.Pattern, suppliedFiles)
				if embedPath != "" {
					projectImports = append(projectImports, embedPath)
				}
			}

			// Extract export info for symbol-level cross-package resolution
			exportInfo, _ := _go.ExtractGoExportInfoFromContent(absPath, sourceContent)

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
				packageDir := resolveGoImportPath(absPath, importPath, contentReader)

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

						// Skip test files from other packages (they can't export symbols)
						if strings.HasSuffix(depFile, "_test.go") && !sameDir {
							continue
						}

						// Go imports only create dependencies on Go source files
						// Non-Go file dependencies should come from //go:embed directives
						if filepath.Ext(depFile) != ".go" {
							continue
						}

						// Symbol-level filtering for cross-package imports:
						// - Different directory imports always use symbol filtering
						// - Same directory imports use symbol filtering when source is a test file
						//   (test files in package X_test import package X via explicit imports)
						if (!sameDir || isTestFile) && hasExportIndex && usedSymbols != nil && len(usedSymbols) > 0 {
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
			content, err := contentReader(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
			}

			imports, err := kotlin.ParseKotlinImports(content)
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
					contentReader,
					kotlinFilePackages,
					kotlinPackageTypes,
					imports,
					suppliedFiles)
				projectImports = append(projectImports, samePackageDeps...)
			}
		} else if ext == ".ts" || ext == ".tsx" {
			content, err := contentReader(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
			}

			imports, parseErr := typescript.ParseTypeScriptImports(content, ext == ".tsx")
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
		intraDeps, err := _go.BuildIntraPackageDependencies(goFiles, vcs.ContentReader(contentReader))
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
// The contentReader is used to read go.mod content
func resolveGoImportPath(sourceFile, importPath string, contentReader vcs.ContentReader) string {
	// For Go files, we need to find the module root and resolve the import
	// This is a simplified version that assumes the project follows standard Go module structure

	// Find the go.mod file by walking up from the source file
	moduleRoot := findModuleRoot(filepath.Dir(sourceFile))
	if moduleRoot == "" {
		// If no module root found, return empty string
		return ""
	}

	// Get the module name from go.mod using the content reader
	moduleName := getModuleName(moduleRoot, contentReader)
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

// getModuleName reads the module name from go.mod using the content reader
func getModuleName(moduleRoot string, contentReader vcs.ContentReader) string {
	goModPath := filepath.Join(moduleRoot, "go.mod")
	content, err := contentReader(goModPath)
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

// resolveGoEmbedPath resolves a Go embed pattern to an absolute file path
// Returns empty string if the pattern doesn't match any supplied file
func resolveGoEmbedPath(sourceFile, pattern string, suppliedFiles map[string]bool) string {
	// Get directory of source file
	sourceDir := filepath.Dir(sourceFile)

	// For simple file patterns (no glob characters), just resolve directly
	if !strings.ContainsAny(pattern, "*?[") {
		absPath := filepath.Join(sourceDir, pattern)
		absPath = filepath.Clean(absPath)

		// Check if this file is in the supplied files
		if suppliedFiles[absPath] {
			return absPath
		}
		return ""
	}

	// For glob patterns, we need to match against supplied files
	// Create a glob pattern with the full path
	globPattern := filepath.Join(sourceDir, pattern)

	// Check each supplied file to see if it matches the pattern
	for file := range suppliedFiles {
		matched, err := filepath.Match(globPattern, file)
		if err == nil && matched {
			// Return the first match (for simple cases)
			// TODO: For full glob support, return all matches
			return file
		}
	}

	return ""
}

// buildKotlinPackageIndex builds maps describing available Kotlin packages and their type declarations
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
	contentReader vcs.ContentReader,
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

	sourceCode, err := contentReader(sourceFile)
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
