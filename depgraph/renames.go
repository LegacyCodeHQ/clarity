package depgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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

// CollapseRenames removes each rename source (old path) from the graph,
// redirecting any edge that pointed at the old path to the new path. The new
// path keeps its own dependencies (parsed from its current content), so a
// rename renders as a single node rather than an old->new pair.
func CollapseRenames(graph DependencyGraph, renames map[string]string) (DependencyGraph, error) {
	if len(renames) == 0 {
		return graph, nil
	}

	adjacency, err := AdjacencyList(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to build adjacency list: %w", err)
	}

	rewritten := make(map[string][]string, len(adjacency))
	for node, deps := range adjacency {
		if _, isRenameSource := renames[node]; isRenameSource {
			continue // drop the old node and its outgoing edges
		}
		seen := make(map[string]bool, len(deps))
		var out []string
		for _, dep := range deps {
			if newPath, ok := renames[dep]; ok {
				dep = newPath // redirect an edge that pointed at the old path
			}
			if dep == node || seen[dep] {
				continue
			}
			seen[dep] = true
			out = append(out, dep)
		}
		rewritten[node] = out
	}

	newGraph, err := NewDependencyGraphFromAdjacency(rewritten)
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild graph without rename sources: %w", err)
	}
	return newGraph, nil
}

// renameChurn approximates the line-level churn between a rename's old and new
// content via a line multiset difference: identical content yields +0 -0 (a
// pure move), edits yield the count of lines added and removed. IsNew stays
// false — a rename is not a new file.
func renameChurn(oldContent, newContent []byte) vcs.FileStats {
	counts := make(map[string]int)
	for _, line := range splitLines(oldContent) {
		counts[line]++
	}
	additions := 0
	for _, line := range splitLines(newContent) {
		if counts[line] > 0 {
			counts[line]--
		} else {
			additions++
		}
	}
	deletions := 0
	for _, remaining := range counts {
		deletions += remaining
	}
	return vcs.FileStats{Additions: additions, Deletions: deletions}
}

func splitLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	return strings.Split(string(content), "\n")
}
