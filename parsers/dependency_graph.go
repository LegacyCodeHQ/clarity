package parsers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/sanity/git"
	"github.com/LegacyCodeHQ/sanity/parsers/dart"
	"github.com/LegacyCodeHQ/sanity/parsers/go"
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
	suppliedFiles := make(map[string]bool)
	dirToFiles := make(map[string][]string)
	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
		}
		suppliedFiles[absPath] = true

		// Map directory to file for Go package imports
		dir := filepath.Dir(absPath)
		dirToFiles[dir] = append(dirToFiles[dir], absPath)
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
		if ext != ".dart" && ext != ".go" {
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

			if repoPath != "" && commitID != "" {
				// Read file from git commit
				relPath := getRelativePath(absPath, repoPath)
				content, err := git.GetFileContentFromCommit(repoPath, commitID, relPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read %s from commit %s: %w", relPath, commitID, err)
				}
				imports, err = _go.ParseGoImports(content)
			} else {
				// Read file from filesystem
				imports, err = _go.GoImports(filePath)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
			}

			// Determine if this is a test file
			isTestFile := strings.HasSuffix(absPath, "_test.go")

			// Filter for internal imports only that are in the supplied file list
			for _, imp := range imports {
				if intImp, ok := imp.(_go.InternalImport); ok {
					// Resolve import path to package directory
					packageDir := resolveGoImportPath(absPath, intImp.Path())

					// Find all files in the supplied list that are in this package
					if files, ok := dirToFiles[packageDir]; ok {
						for _, depFile := range files {
							// Don't add self-dependencies
							if depFile != absPath {
								// Non-test files should not depend on test files from imported packages
								if !isTestFile && strings.HasSuffix(depFile, "_test.go") {
									continue
								}
								projectImports = append(projectImports, depFile)
							}
						}
					}
				}
			}
		}

		graph[absPath] = projectImports
	}

	// Third pass: Add intra-package dependencies for Go files
	// This handles dependencies between files in the same package (which don't import each other)
	goFiles := []string{}
	for _, filePath := range filePaths {
		absPath, _ := filepath.Abs(filePath)
		if filepath.Ext(absPath) == ".go" {
			goFiles = append(goFiles, absPath)
		}
	}

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
func resolveGoImportPath(sourceFile, importPath string) string {
	// For Go files, we need to find the module root and resolve the import
	// This is a simplified version that assumes the project follows standard Go module structure

	// Find the go.mod file by walking up from the source file
	moduleRoot := findModuleRoot(filepath.Dir(sourceFile))
	if moduleRoot == "" {
		// If no module root found, return empty string
		return ""
	}

	// Get the module name from go.mod
	moduleName := getModuleName(moduleRoot)
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

// ToJSON converts the dependency graph to JSON format
func (g DependencyGraph) ToJSON() ([]byte, error) {
	return json.MarshalIndent(g, "", "  ")
}

// ToDOT converts the dependency graph to Graphviz DOT format
func (g DependencyGraph) ToDOT() string {
	var sb strings.Builder
	sb.WriteString("digraph dependencies {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box];\n\n")

	// Find all files with the most dependencies (nodes)
	maxDeps := 0
	for _, deps := range g {
		if len(deps) > maxDeps {
			maxDeps = len(deps)
		}
	}

	// Track all files that have the maximum dependency count
	filesWithMostDeps := make(map[string]bool)
	for source, deps := range g {
		if len(deps) == maxDeps {
			filesWithMostDeps[source] = true
		}
	}

	// Count unique file extensions to determine if we need extension-based coloring
	uniqueExtensions := make(map[string]bool)
	for source := range g {
		ext := filepath.Ext(filepath.Base(source))
		uniqueExtensions[ext] = true
	}
	hasMultipleExtensions := len(uniqueExtensions) > 1

	// Available colors for dynamic assignment to extensions
	availableColors := []string{
		"lightblue", "lightyellow", "lightcoral", "lightcyan",
		"lightsalmon", "lightpink", "lavender", "mistyrose",
		"peachpuff", "plum", "powderblue", "khaki",
		"palegreen", "palegoldenrod", "paleturquoise", "thistle",
	}

	// Dynamically assign colors to extensions as they are encountered
	extensionColors := make(map[string]string)
	colorIndex := 0

	// Helper function to get color for an extension
	getColorForExtension := func(ext string) string {
		if color, ok := extensionColors[ext]; ok {
			return color
		}
		// Assign a new color to this extension
		color := availableColors[colorIndex%len(availableColors)]
		extensionColors[ext] = color
		colorIndex++
		return color
	}

	// Helper function to check if a file is a test file
	isTestFile := func(source string) bool {
		sourceBase := filepath.Base(source)

		// Go test files: must have _test.go suffix
		if strings.HasSuffix(sourceBase, "_test.go") {
			return true
		}

		// Dart test files: check if in test/ directory
		if filepath.Ext(sourceBase) == ".dart" && strings.Contains(filepath.ToSlash(source), "/test/") {
			return true
		}

		return false
	}

	// Track which nodes have been styled to avoid duplicates
	styledNodes := make(map[string]bool)

	// First, define node styles based on file extensions
	for source := range g {
		sourceBase := filepath.Base(source)

		if !styledNodes[sourceBase] {
			var color string

			// Priority 1: Test files are always light green
			if isTestFile(source) {
				color = "lightgreen"
			} else if filesWithMostDeps[source] {
				// Priority 2: Files with most dependencies are always white
				color = "white"
			} else if hasMultipleExtensions {
				// Priority 3: Color based on extension (only if multiple extensions exist)
				ext := filepath.Ext(sourceBase)
				color = getColorForExtension(ext)
			} else {
				// Priority 4: Single extension - use white (no need to differentiate)
				color = "white"
			}

			sb.WriteString(fmt.Sprintf("  %q [style=filled, fillcolor=%s];\n", sourceBase, color))
			styledNodes[sourceBase] = true
		}
	}
	if len(styledNodes) > 0 {
		sb.WriteString("\n")
	}

	for source, deps := range g {
		// Use base filename for cleaner visualization
		sourceBase := filepath.Base(source)
		for _, dep := range deps {
			depBase := filepath.Base(dep)
			sb.WriteString(fmt.Sprintf("  %q -> %q;\n", sourceBase, depBase))
		}

		// Handle files with no dependencies
		if len(deps) == 0 {
			sb.WriteString(fmt.Sprintf("  %q;\n", sourceBase))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
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
