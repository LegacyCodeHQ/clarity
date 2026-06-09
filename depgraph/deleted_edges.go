package depgraph

import "github.com/LegacyCodeHQ/clarity/vcs"

// MergeDeletedNeighborhood reconstructs each deleted file's pre-deletion edges
// and merges them into graph. A deleted file no longer exists in the viewed
// snapshot, so the files that imported it have dropped the reference and it
// imports nothing resolvable — leaving the node floating. We rebuild the parent
// snapshot (via parentReader) over the candidate files that still existed there
// and copy every edge touching a deleted file, both incoming and outgoing, into
// graph. Callers run MarkDeletedFiles afterward to style these as removed edges.
//
// candidateFiles is the full node set (present + deleted); files absent from the
// parent (e.g. newly added ones) are skipped automatically.
func MergeDeletedNeighborhood(
	graph DependencyGraph,
	candidateFiles []string,
	deletedFiles []string,
	parentReader vcs.ContentReader,
) (DependencyGraph, error) {
	if len(deletedFiles) == 0 {
		return graph, nil
	}

	deleted := make(map[string]bool, len(deletedFiles))
	for _, d := range deletedFiles {
		deleted[d] = true
	}

	// Restrict to files that exist in the parent snapshot. Newly added files have
	// no parent content and would otherwise error during the rebuild.
	parentFiles := make([]string, 0, len(candidateFiles))
	seen := make(map[string]bool, len(candidateFiles))
	for _, f := range candidateFiles {
		if seen[f] {
			continue
		}
		seen[f] = true
		if _, err := parentReader(f); err == nil {
			parentFiles = append(parentFiles, f)
		}
	}
	if len(parentFiles) == 0 {
		return graph, nil
	}

	parentGraph, err := BuildDependencyGraph(parentFiles, parentReader)
	if err != nil {
		return nil, err
	}
	parentAdj, err := AdjacencyList(parentGraph)
	if err != nil {
		return nil, err
	}

	adj, err := AdjacencyList(graph)
	if err != nil {
		return nil, err
	}
	for d := range deleted {
		if _, ok := adj[d]; !ok {
			adj[d] = nil
		}
	}
	for src, dsts := range parentAdj {
		for _, dst := range dsts {
			if !deleted[src] && !deleted[dst] {
				continue
			}
			adj[src] = appendUnique(adj[src], dst)
		}
	}

	return NewDependencyGraphFromAdjacency(adj)
}

func appendUnique(list []string, item string) []string {
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}
