package depgraph

import (
	"fmt"
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/depgraph/langsupport"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type dependencyGraphContext = langsupport.Context

func buildDependencyGraphContext(filePaths []string, contentReader vcs.ContentReader) (*dependencyGraphContext, error) {
	suppliedFiles, dirToFiles, javaFiles, kotlinFiles, goFiles, err := collectDependencyGraphFiles(filePaths)
	if err != nil {
		return nil, err
	}

	return &dependencyGraphContext{
		SuppliedFiles: suppliedFiles,
		DirToFiles:    dirToFiles,
		JavaFiles:     javaFiles,
		KotlinFiles:   kotlinFiles,
		GoFiles:       goFiles,
	}, nil
}

func collectDependencyGraphFiles(filePaths []string) (map[string]bool, map[string][]string, []string, []string, []string, error) {
	suppliedFiles := make(map[string]bool)
	dirToFiles := make(map[string][]string)
	var javaFiles []string
	var kotlinFiles []string
	var goFiles []string

	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
		}
		suppliedFiles[absPath] = true

		// Map directory to file for Go package imports
		dir := filepath.Dir(absPath)
		dirToFiles[dir] = append(dirToFiles[dir], absPath)

		// Collect Java files for package/type indexing
		if filepath.Ext(absPath) == ".java" {
			javaFiles = append(javaFiles, absPath)
		}

		// Collect Kotlin files for package indexing
		if ext := filepath.Ext(absPath); ext == ".kt" || ext == ".kts" {
			kotlinFiles = append(kotlinFiles, absPath)
		}

		// Collect Go files for export indexing
		if filepath.Ext(absPath) == ".go" {
			goFiles = append(goFiles, absPath)
		}
	}

	return suppliedFiles, dirToFiles, javaFiles, kotlinFiles, goFiles, nil
}
