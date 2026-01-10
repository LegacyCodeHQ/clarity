package parser

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// DependencyGraph represents a mapping from file paths to their project dependencies
type DependencyGraph map[string][]string

// BuildDependencyGraph analyzes a list of files and builds a dependency graph
// containing only project imports (excluding package: and dart: imports).
// Only dependencies that are in the supplied file list are included in the graph.
func BuildDependencyGraph(filePaths []string) (DependencyGraph, error) {
	graph := make(DependencyGraph)

	// First pass: build a set of all supplied file paths (as absolute paths)
	suppliedFiles := make(map[string]bool)
	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
		}
		suppliedFiles[absPath] = true
	}

	// Second pass: build the dependency graph
	for _, filePath := range filePaths {
		// Get absolute path
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
		}

		// Check if this is a Dart file
		if filepath.Ext(absPath) != ".dart" {
			// Non-Dart files are included in the graph with no dependencies
			graph[absPath] = []string{}
			continue
		}

		// Parse imports for Dart files
		imports, err := Imports(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse imports in %s: %w", filePath, err)
		}

		// Filter for project imports only that are in the supplied file list
		var projectImports []string
		for _, imp := range imports {
			if projImp, ok := imp.(ProjectImport); ok {
				// Resolve relative path to absolute
				resolvedPath := resolveImportPath(absPath, projImp.URI())

				// Only include if the dependency is in the supplied files
				if suppliedFiles[resolvedPath] {
					projectImports = append(projectImports, resolvedPath)
				}
			}
		}

		graph[absPath] = projectImports
	}

	return graph, nil
}

// resolveImportPath converts a relative import URI to an absolute path
func resolveImportPath(sourceFile, importURI string) string {
	// Get directory of source file
	sourceDir := filepath.Dir(sourceFile)

	// Resolve relative import
	absImport := filepath.Join(sourceDir, importURI)

	// Add .dart extension if not present
	if !strings.HasSuffix(absImport, ".dart") {
		absImport += ".dart"
	}

	return filepath.Clean(absImport)
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

// ToList converts the dependency graph to a simple readable list format
func (g DependencyGraph) ToList() string {
	var sb strings.Builder
	for source, deps := range g {
		sb.WriteString(fmt.Sprintf("%s:\n", source))
		if len(deps) == 0 {
			sb.WriteString("  (no project dependencies)\n")
		} else {
			for _, dep := range deps {
				sb.WriteString(fmt.Sprintf("  -> %s\n", dep))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
