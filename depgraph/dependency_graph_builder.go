package depgraph

import (
	"errors"
	"fmt"
	"path/filepath"

	graphlib "github.com/dominikbraun/graph"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// BuildDependencyGraph analyzes a list of files and builds a dependency graph
// containing only project imports (excluding package:/dart: imports for Dart,
// and standard library/external imports for Go).
// Only dependencies that are in the supplied file list are included in the graph.
// The contentReader function is used to read file contents (from filesystem, git commit, etc.)
func BuildDependencyGraph(filePaths []string, contentReader vcs.ContentReader) (DependencyGraph, error) {
	ctx, err := buildDependencyGraphContext(filePaths, contentReader)
	if err != nil {
		return nil, err
	}

	return BuildDependencyGraphWithResolver(filePaths, NewDefaultDependencyResolver(ctx, contentReader))
}

// BuildDependencyGraphWithResolver builds a graph using the provided DependencyResolver implementation.
func BuildDependencyGraphWithResolver(
	filePaths []string,
	dependencyResolver DependencyResolver,
) (DependencyGraph, error) {
	graph := NewDependencyGraph()

	if dependencyResolver == nil {
		return nil, fmt.Errorf("dependency resolver is required")
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
		if !dependencyResolver.SupportsFileExtension(ext) {
			// Unsupported files are included in the graph with no dependencies
			if err := graph.AddVertex(absPath); err != nil && !errors.Is(err, graphlib.ErrVertexAlreadyExists) {
				return nil, fmt.Errorf("failed to add graph vertex %s: %w", absPath, err)
			}
			continue
		}

		projectImports, err := dependencyResolver.ResolveProjectImports(absPath, filePath, ext)
		if err != nil {
			return nil, err
		}

		if len(projectImports) > 0 {
			projectImports = deduplicatePaths(projectImports)
		}
		if err := graph.AddVertex(absPath); err != nil && !errors.Is(err, graphlib.ErrVertexAlreadyExists) {
			return nil, fmt.Errorf("failed to add graph vertex %s: %w", absPath, err)
		}
		for _, dep := range projectImports {
			if err := graph.AddVertex(dep); err != nil && !errors.Is(err, graphlib.ErrVertexAlreadyExists) {
				return nil, fmt.Errorf("failed to add graph dependency vertex %s: %w", dep, err)
			}
			if err := graph.AddEdge(absPath, dep); err != nil && !errors.Is(err, graphlib.ErrEdgeAlreadyExists) {
				return nil, fmt.Errorf("failed to add graph edge %s -> %s: %w", absPath, dep, err)
			}
		}
	}

	// Third pass: add intra-package dependencies for languages that need it.
	if err := dependencyResolver.FinalizeGraph(graph); err != nil {
		return graph, fmt.Errorf("failed to add intra-package dependencies: %w", err)
	}

	return graph, nil
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
