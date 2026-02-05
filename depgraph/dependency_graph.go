package depgraph

import (
	"errors"
	"sort"

	graphlib "github.com/dominikbraun/graph"
)

// DependencyGraph is the shared graph type used across the codebase.
type DependencyGraph = graphlib.Graph[string, string]

// NewDependencyGraph creates an empty directed dependency graph.
func NewDependencyGraph() DependencyGraph {
	return graphlib.New(graphlib.StringHash, graphlib.Directed())
}

// NewDependencyGraphFromAdjacency builds a graph from adjacency data.
func NewDependencyGraphFromAdjacency(adjacency map[string][]string) (DependencyGraph, error) {
	g := NewDependencyGraph()

	nodes := make(map[string]struct{}, len(adjacency))
	for source, deps := range adjacency {
		nodes[source] = struct{}{}
		for _, dep := range deps {
			nodes[dep] = struct{}{}
		}
	}

	sortedNodes := make([]string, 0, len(nodes))
	for node := range nodes {
		sortedNodes = append(sortedNodes, node)
	}
	sort.Strings(sortedNodes)
	for _, node := range sortedNodes {
		if err := g.AddVertex(node); err != nil && !errors.Is(err, graphlib.ErrVertexAlreadyExists) {
			return nil, err
		}
	}

	sortedSources := make([]string, 0, len(adjacency))
	for source := range adjacency {
		sortedSources = append(sortedSources, source)
	}
	sort.Strings(sortedSources)

	for _, source := range sortedSources {
		deps := append([]string(nil), adjacency[source]...)
		sort.Strings(deps)
		for _, dep := range deps {
			if err := g.AddEdge(source, dep); err != nil && !errors.Is(err, graphlib.ErrEdgeAlreadyExists) {
				return nil, err
			}
		}
	}

	return g, nil
}

// MustDependencyGraph builds a graph from adjacency data and panics on errors.
// Intended for tests.
func MustDependencyGraph(adjacency map[string][]string) DependencyGraph {
	g, err := NewDependencyGraphFromAdjacency(adjacency)
	if err != nil {
		panic(err)
	}
	return g
}

// AdjacencyList returns a plain adjacency list for the given graph.
func AdjacencyList(g DependencyGraph) (map[string][]string, error) {
	adjacencyMap, err := g.AdjacencyMap()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string, len(adjacencyMap))
	for source, deps := range adjacencyMap {
		targets := make([]string, 0, len(deps))
		for target := range deps {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		result[source] = targets
	}
	return result, nil
}

// ContainsNode reports whether node exists in the graph.
func ContainsNode(g DependencyGraph, node string) bool {
	_, err := g.Vertex(node)
	return err == nil
}

// DependenciesOf returns outgoing dependencies for a node.
func DependenciesOf(g DependencyGraph, node string) ([]string, bool, error) {
	adjacencyMap, err := g.AdjacencyMap()
	if err != nil {
		return nil, false, err
	}
	deps, ok := adjacencyMap[node]
	if !ok {
		return nil, false, nil
	}
	targets := make([]string, 0, len(deps))
	for target := range deps {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets, true, nil
}
