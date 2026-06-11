package depgraph

import (
	"fmt"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

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
