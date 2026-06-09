package depgraph

import (
	"fmt"
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type dependencyGraphContext = moduleapi.Context

func buildDependencyGraphContext(filePaths []string, contentReader vcs.ContentReader) (*dependencyGraphContext, error) {
	suppliedFiles, dirToFiles, filesByExtension, err := collectDependencyGraphFiles(filePaths)
	if err != nil {
		return nil, err
	}

	return &dependencyGraphContext{
		SuppliedFiles:    suppliedFiles,
		DirToFiles:       dirToFiles,
		FilesByExtension: filesByExtension,
	}, nil
}

func collectDependencyGraphFiles(filePaths []string) (map[string]bool, map[string][]string, map[string][]string, error) {
	suppliedFiles := make(map[string]bool)
	dirToFiles := make(map[string][]string)
	filesByExtension := make(map[string][]string)

	for _, filePath := range filePaths {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
		}
		suppliedFiles[absPath] = true

		// Map directory to file for Go package imports
		dir := filepath.Dir(absPath)
		dirToFiles[dir] = append(dirToFiles[dir], absPath)

		// Group by extension so providers can fetch the files they declare
		// without this builder knowing about specific languages.
		ext := filepath.Ext(absPath)
		filesByExtension[ext] = append(filesByExtension[ext], absPath)
	}

	return suppliedFiles, dirToFiles, filesByExtension, nil
}
