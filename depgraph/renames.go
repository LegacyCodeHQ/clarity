package depgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// DetectRenames identifies rename sources among the deleted files: a deleted
// path whose pre-deletion content reappears verbatim in a currently existing
// file is treated as having been renamed to that file. Returns a map of old
// path -> new path, or nil when there are no renames.
func DetectRenames(
	deletedFiles []string,
	deletedContent map[string][]byte,
	filePaths []string,
	contentReader vcs.ContentReader,
) map[string]string {
	if len(deletedFiles) == 0 {
		return nil
	}

	deleted := make(map[string]bool, len(deletedFiles))
	for _, d := range deletedFiles {
		deleted[d] = true
	}

	// Index currently-existing (non-deleted) files by content hash.
	byHash := make(map[string]string)
	for _, f := range filePaths {
		if deleted[f] {
			continue
		}
		content, err := contentReader(f)
		if err != nil || len(content) == 0 {
			continue
		}
		byHash[contentHash(content)] = f
	}

	renames := make(map[string]string)
	for _, d := range deletedFiles {
		content, ok := deletedContent[d]
		if !ok || len(content) == 0 {
			continue
		}
		if newPath, ok := byHash[contentHash(content)]; ok {
			renames[d] = newPath
		}
	}
	if len(renames) == 0 {
		return nil
	}
	return renames
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// AddRenameEdges adds an old->new edge for each detected rename so the move is
// drawn in the graph. The new path is already a node (it's a supplied file).
func AddRenameEdges(graph DependencyGraph, renames map[string]string) (DependencyGraph, error) {
	if len(renames) == 0 {
		return graph, nil
	}

	adjacency, err := AdjacencyList(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to build adjacency list: %w", err)
	}
	for oldPath, newPath := range renames {
		adjacency[oldPath] = append(adjacency[oldPath], newPath)
	}

	newGraph, err := NewDependencyGraphFromAdjacency(adjacency)
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild graph with rename edges: %w", err)
	}
	return newGraph, nil
}
