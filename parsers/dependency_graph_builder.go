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

	ctx, err := buildDependencyGraphContext(filePaths, contentReader)
	if err != nil {
		return nil, err
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

		projectImports, err := buildProjectImports(absPath, filePath, ext, ctx, contentReader)
		if err != nil {
			return nil, err
		}

		if len(projectImports) > 0 {
			projectImports = deduplicatePaths(projectImports)
		}

		graph[absPath] = projectImports
	}

	// Third pass: Add intra-package dependencies for Go files
	// This handles dependencies between files in the same package (which don't import each other)
	// Note: goFiles was already collected in the first pass

	if err := addGoIntraPackageDependencies(graph, ctx.goFiles, contentReader); err != nil {
		return graph, fmt.Errorf("failed to add intra-package dependencies: %w", err)
	}

	return graph, nil
}

type dependencyGraphContext struct {
	suppliedFiles          map[string]bool
	dirToFiles             map[string][]string
	kotlinFiles            []string
	goFiles                []string
	goPackageExportIndices map[string]_go.GoPackageExportIndex
	kotlinPackageIndex     map[string][]string
	kotlinPackageTypes     map[string]map[string][]string
	kotlinFilePackages     map[string]string
}

func buildDependencyGraphContext(filePaths []string, contentReader vcs.ContentReader) (*dependencyGraphContext, error) {
	suppliedFiles, dirToFiles, kotlinFiles, goFiles, err := collectDependencyGraphFiles(filePaths)
	if err != nil {
		return nil, err
	}

	goPackageExportIndices := buildGoPackageExportIndices(dirToFiles, contentReader)
	kotlinPackageIndex, kotlinPackageTypes, kotlinFilePackages := buildKotlinIndices(kotlinFiles, contentReader)

	return &dependencyGraphContext{
		suppliedFiles:          suppliedFiles,
		dirToFiles:             dirToFiles,
		kotlinFiles:            kotlinFiles,
		goFiles:                goFiles,
		goPackageExportIndices: goPackageExportIndices,
		kotlinPackageIndex:     kotlinPackageIndex,
		kotlinPackageTypes:     kotlinPackageTypes,
		kotlinFilePackages:     kotlinFilePackages,
	}, nil
}

func collectDependencyGraphFiles(filePaths []string) (map[string]bool, map[string][]string, []string, []string, error) {
	suppliedFiles := make(map[string]bool)
	dirToFiles := make(map[string][]string)
	var kotlinFiles []string
	var goFiles []string

	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
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

	return suppliedFiles, dirToFiles, kotlinFiles, goFiles, nil
}

func buildGoPackageExportIndices(dirToFiles map[string][]string, contentReader vcs.ContentReader) map[string]_go.GoPackageExportIndex {
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
			if err != nil {
				continue
			}
			goPackageExportIndices[dir] = exportIndex
		}
	}

	return goPackageExportIndices
}

func buildKotlinIndices(
	kotlinFiles []string,
	contentReader vcs.ContentReader,
) (map[string][]string, map[string]map[string][]string, map[string]string) {
	if len(kotlinFiles) == 0 {
		return nil, nil, map[string]string{}
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

func buildProjectImports(
	absPath string,
	filePath string,
	ext string,
	ctx *dependencyGraphContext,
	contentReader vcs.ContentReader,
) ([]string, error) {
	switch ext {
	case ".dart":
		return buildDartProjectImports(absPath, filePath, ext, ctx.suppliedFiles, contentReader)
	case ".go":
		return buildGoProjectImports(absPath, filePath, ctx, contentReader)
	case ".kt":
		return buildKotlinProjectImports(absPath, filePath, ctx, contentReader)
	case ".ts", ".tsx":
		return buildTypeScriptProjectImports(absPath, filePath, ext, ctx.suppliedFiles, contentReader)
	default:
		return []string{}, nil
	}
}

func buildDartProjectImports(
	absPath string,
	filePath string,
	ext string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, err := dart.ParseImports(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	var projectImports []string
	for _, imp := range imports {
		if projImp, ok := imp.(dart.ProjectImport); ok {
			resolvedPath := resolveImportPath(absPath, projImp.URI(), ext)
			if suppliedFiles[resolvedPath] {
				projectImports = append(projectImports, resolvedPath)
			}
		}
	}

	return projectImports, nil
}

func buildGoProjectImports(
	absPath string,
	filePath string,
	ctx *dependencyGraphContext,
	contentReader vcs.ContentReader,
) ([]string, error) {
	sourceContent, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, err := _go.ParseGoImports(sourceContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	var projectImports []string

	// Parse //go:embed directives
	embeds, _ := _go.ParseGoEmbeds(sourceContent)
	for _, embed := range embeds {
		embedPath := resolveGoEmbedPath(absPath, embed.Pattern, ctx.suppliedFiles)
		if embedPath != "" {
			projectImports = append(projectImports, embedPath)
		}
	}

	// Extract export info for symbol-level cross-package resolution
	exportInfo, _ := _go.ExtractGoExportInfoFromContent(absPath, sourceContent)

	// Determine if this is a test file
	isTestFile := strings.HasSuffix(absPath, "_test.go")

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

		packageDir := resolveGoImportPath(absPath, importPath, contentReader)
		if packageDir == "" {
			continue
		}

		sourceDir := filepath.Dir(absPath)
		sameDir := sourceDir == packageDir
		exportIndex, hasExportIndex := ctx.goPackageExportIndices[packageDir]

		var usedSymbols map[string]bool
		if exportInfo != nil {
			usedSymbols = _go.GetUsedSymbolsFromPackage(exportInfo, importPath)
		}

		if files, ok := ctx.dirToFiles[packageDir]; ok {
			for _, depFile := range files {
				if depFile == absPath {
					continue
				}

				if strings.HasSuffix(depFile, "_test.go") && !sameDir {
					continue
				}

				if filepath.Ext(depFile) != ".go" {
					continue
				}

				if (!sameDir || isTestFile) && hasExportIndex && usedSymbols != nil && len(usedSymbols) > 0 {
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

	return projectImports, nil
}

func buildKotlinProjectImports(
	absPath string,
	filePath string,
	ctx *dependencyGraphContext,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, err := kotlin.ParseKotlinImports(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
	}

	projectPackages := make(map[string]bool)
	for pkg := range ctx.kotlinPackageIndex {
		projectPackages[pkg] = true
	}

	imports = kotlin.ClassifyWithProjectPackages(imports, projectPackages)

	var projectImports []string
	for _, imp := range imports {
		if internalImp, ok := imp.(kotlin.InternalImport); ok {
			resolvedFiles := resolveKotlinImportPath(absPath, internalImp, ctx.kotlinPackageIndex, ctx.suppliedFiles)
			projectImports = append(projectImports, resolvedFiles...)
		}
	}

	if len(ctx.kotlinPackageTypes) > 0 {
		samePackageDeps := resolveKotlinSamePackageDependencies(
			absPath,
			contentReader,
			ctx.kotlinFilePackages,
			ctx.kotlinPackageTypes,
			imports,
			ctx.suppliedFiles,
		)
		projectImports = append(projectImports, samePackageDeps...)
	}

	return projectImports, nil
}

func buildTypeScriptProjectImports(
	absPath string,
	filePath string,
	ext string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	imports, parseErr := typescript.ParseTypeScriptImports(content, ext == ".tsx")
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, parseErr)
	}

	var projectImports []string
	for _, imp := range imports {
		if internalImp, ok := imp.(typescript.InternalImport); ok {
			resolvedFiles := typescript.ResolveTypeScriptImportPath(absPath, internalImp.Path(), suppliedFiles)
			projectImports = append(projectImports, resolvedFiles...)
		}
	}

	return projectImports, nil
}

func addGoIntraPackageDependencies(
	graph DependencyGraph,
	goFiles []string,
	contentReader vcs.ContentReader,
) error {
	if len(goFiles) == 0 {
		return nil
	}

	intraDeps, err := _go.BuildIntraPackageDependencies(goFiles, vcs.ContentReader(contentReader))
	if err != nil {
		return err
	}

	for file, deps := range intraDeps {
		if existingDeps, ok := graph[file]; ok {
			depSet := make(map[string]bool)
			for _, dep := range existingDeps {
				depSet[dep] = true
			}
			for _, dep := range deps {
				depSet[dep] = true
			}

			merged := make([]string, 0, len(depSet))
			for dep := range depSet {
				merged = append(merged, dep)
			}
			graph[file] = merged
		}
	}

	return nil
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
		if _, err := os.Stat(goModPath); err != nil {
			// keep walking up the tree
		} else {
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
		if err != nil {
			continue
		}
		if matched {
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

	typeReferences := kotlin.ExtractTypeIdentifiers(sourceCode)
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
