package depgraph

// FindPathNodes returns all nodes on any path between specified files.
// Treats the graph bidirectionally (paths from A to B OR from B to A).
// A node X is included if it lies on any directed path between any pair of target files.
// If a file isn't in the graph, it's skipped.
func FindPathNodes(graph DependencyGraph, targetFiles []string) DependencyGraph {
	adjacency, err := AdjacencyList(graph)
	if err != nil {
		return NewDependencyGraph()
	}

	// Filter target files to only those that exist in the graph
	var validTargets []string
	for _, f := range targetFiles {
		if _, ok := adjacency[f]; ok {
			validTargets = append(validTargets, f)
		}
	}

	if len(validTargets) < 2 {
		// Not enough targets to find paths
		// Return subgraph containing just the valid targets (if any)
		result := make(map[string][]string)
		for _, f := range validTargets {
			result[f] = []string{}
		}
		return MustDependencyGraph(result)
	}

	// Build forward and reverse adjacency lists from directed graph
	forward, reverse := buildAdjacencyLists(adjacency)

	// Find all nodes on any path between all pairs of targets
	nodesToKeep := make(map[string]bool)

	// Always include target nodes
	for _, f := range validTargets {
		nodesToKeep[f] = true
	}

	// For each pair of targets, find nodes on directed paths (both directions)
	for i := 0; i < len(validTargets); i++ {
		for j := i + 1; j < len(validTargets); j++ {
			// Find paths from i to j
			pathNodes := findDirectedPathNodes(forward, reverse, validTargets[i], validTargets[j])
			for node := range pathNodes {
				nodesToKeep[node] = true
			}
			// Find paths from j to i
			pathNodes = findDirectedPathNodes(forward, reverse, validTargets[j], validTargets[i])
			for node := range pathNodes {
				nodesToKeep[node] = true
			}
		}
	}

	// Extract subgraph with only the nodes to keep
	return extractSubgraph(adjacency, nodesToKeep)
}

// buildAdjacencyLists creates forward and reverse adjacency lists from the graph.
// Forward: A→B means forward[A] contains B
// Reverse: A→B means reverse[B] contains A
func buildAdjacencyLists(adjacency map[string][]string) (forward, reverse map[string][]string) {
	forward = make(map[string][]string)
	reverse = make(map[string][]string)

	// Initialize all nodes
	for node := range adjacency {
		forward[node] = []string{}
		reverse[node] = []string{}
	}

	// Build adjacency lists
	for node, deps := range adjacency {
		for _, dep := range deps {
			forward[node] = append(forward[node], dep)
			reverse[dep] = append(reverse[dep], node)
		}
	}

	return forward, reverse
}

// findDirectedPathNodes finds all nodes on any directed path from source to target.
// A node X is on a path from source to target if:
// 1. X is reachable from source (following forward edges)
// 2. Target is reachable from X (following forward edges)
func findDirectedPathNodes(forward, reverse map[string][]string, source, target string) map[string]bool {
	result := make(map[string]bool)

	// Find all nodes reachable from source (forward BFS)
	reachableFromSource := bfsReachable(forward, source)

	// Find all nodes that can reach target (backward BFS using reverse edges)
	canReachTarget := bfsReachable(reverse, target)

	// Intersection: nodes reachable from source AND can reach target
	for node := range reachableFromSource {
		if canReachTarget[node] {
			result[node] = true
		}
	}

	return result
}

// bfsReachable returns all nodes reachable from source.
func bfsReachable(adjacency map[string][]string, source string) map[string]bool {
	reachable := make(map[string]bool)
	reachable[source] = true

	queue := []string{source}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range adjacency[current] {
			if !reachable[neighbor] {
				reachable[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return reachable
}

// extractSubgraph creates a new graph containing only the specified nodes.
// Edges are preserved only if both endpoints are in the node set.
func extractSubgraph(original map[string][]string, nodesToKeep map[string]bool) DependencyGraph {
	result := make(map[string][]string)

	for node := range nodesToKeep {
		if deps, ok := original[node]; ok {
			// Filter dependencies to only those in nodesToKeep
			var filteredDeps []string
			for _, dep := range deps {
				if nodesToKeep[dep] {
					filteredDeps = append(filteredDeps, dep)
				}
			}
			result[node] = filteredDeps
		} else {
			// Node not in original graph, add with empty deps
			result[node] = []string{}
		}
	}

	return MustDependencyGraph(result)
}
