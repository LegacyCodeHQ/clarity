package depgraph

import (
	"sort"
	"testing"
)

func testGraph(adjacency map[string][]string) DependencyGraph {
	return MustDependencyGraph(adjacency)
}

func TestFindPathNodes_Linear(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
	})

	result := FindPathNodes(graph, []string{"A", "C"})
	assertGraphContainsNodes(t, result, []string{"A", "B", "C"})
}

func TestFindPathNodes_Diamond(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
		"D": {},
	})

	result := FindPathNodes(graph, []string{"A", "D"})
	assertGraphContainsNodes(t, result, []string{"A", "B", "C", "D"})
}

func TestFindPathNodes_Disconnected(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {},
		"C": {"D"},
		"D": {},
	})

	result := FindPathNodes(graph, []string{"A", "C"})
	assertGraphContainsNodes(t, result, []string{"A", "C"})
}

func TestFindPathNodes_MultiFile(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"D"},
		"D": {},
	})

	result := FindPathNodes(graph, []string{"A", "C", "D"})
	assertGraphContainsNodes(t, result, []string{"A", "B", "C", "D"})
}

func TestFindPathNodes_AllPaths(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B", "D"},
		"B": {"C"},
		"C": {},
		"D": {"E"},
		"E": {"C"},
	})

	result := FindPathNodes(graph, []string{"A", "C"})
	assertGraphContainsNodes(t, result, []string{"A", "B", "C", "D", "E"})
}

func TestFindPathNodes_Bidirectional(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {},
	})

	result := FindPathNodes(graph, []string{"B", "A"})
	assertGraphContainsNodes(t, result, []string{"A", "B"})
}

func TestFindPathNodes_SingleTarget(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {},
	})

	result := FindPathNodes(graph, []string{"A"})
	assertGraphContainsNodes(t, result, []string{"A"})

	adjacency, err := AdjacencyList(result)
	if err != nil {
		t.Fatalf("AdjacencyList() error = %v", err)
	}
	if len(adjacency) != 1 {
		t.Errorf("Expected exactly 1 node, got %d", len(adjacency))
	}
}

func TestFindPathNodes_NoTargets(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {},
	})

	result := FindPathNodes(graph, []string{})
	adjacency, err := AdjacencyList(result)
	if err != nil {
		t.Fatalf("AdjacencyList() error = %v", err)
	}
	if len(adjacency) > 0 {
		t.Errorf("Expected empty result for no targets, got %d nodes", len(adjacency))
	}
}

func TestFindPathNodes_InvalidTarget(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {},
	})

	result := FindPathNodes(graph, []string{"A", "X"})
	assertGraphContainsNodes(t, result, []string{"A"})
}

func TestFindPathNodes_PreservesEdges(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {},
	})

	result := FindPathNodes(graph, []string{"A", "C"})

	deps, ok, err := DependenciesOf(result, "A")
	if err != nil {
		t.Fatalf("DependenciesOf(A) error = %v", err)
	}
	if !ok || len(deps) != 1 || deps[0] != "B" {
		t.Errorf("Expected A -> B edge, got %v", deps)
	}

	deps, ok, err = DependenciesOf(result, "B")
	if err != nil {
		t.Fatalf("DependenciesOf(B) error = %v", err)
	}
	if !ok || len(deps) != 1 || deps[0] != "C" {
		t.Errorf("Expected B -> C edge, got %v", deps)
	}
}

func TestFindPathNodes_ComplexGraph(t *testing.T) {
	graph := testGraph(map[string][]string{
		"A": {"B", "C"},
		"B": {"D"},
		"C": {"D"},
		"D": {"E"},
		"E": {},
	})

	result := FindPathNodes(graph, []string{"A", "E"})
	assertGraphContainsNodes(t, result, []string{"A", "B", "C", "D", "E"})
}

func TestExtractSubgraph(t *testing.T) {
	original := map[string][]string{
		"A": {"B", "C"},
		"B": {"C"},
		"C": {},
	}

	nodesToKeep := map[string]bool{
		"A": true,
		"B": true,
	}

	result := extractSubgraph(original, nodesToKeep)

	if !ContainsNode(result, "A") {
		t.Error("A should be in result")
	}
	if !ContainsNode(result, "B") {
		t.Error("B should be in result")
	}
	if ContainsNode(result, "C") {
		t.Error("C should not be in result")
	}

	deps, ok, err := DependenciesOf(result, "A")
	if err != nil {
		t.Fatalf("DependenciesOf(A) error = %v", err)
	}
	if !ok || len(deps) != 1 || deps[0] != "B" {
		t.Errorf("A should only have B as dep, got %v", deps)
	}

	deps, ok, err = DependenciesOf(result, "B")
	if err != nil {
		t.Fatalf("DependenciesOf(B) error = %v", err)
	}
	if !ok || len(deps) > 0 {
		t.Errorf("B should have no deps, got %v", deps)
	}
}

func assertGraphContainsNodes(t *testing.T, graph DependencyGraph, expectedNodes []string) {
	t.Helper()

	adjacency, err := AdjacencyList(graph)
	if err != nil {
		t.Fatalf("AdjacencyList() error = %v", err)
	}

	for _, node := range expectedNodes {
		if _, ok := adjacency[node]; !ok {
			t.Errorf("Expected node %s not found in graph", node)
		}
	}

	actualNodes := make([]string, 0, len(adjacency))
	for node := range adjacency {
		actualNodes = append(actualNodes, node)
	}

	sort.Strings(actualNodes)
	sort.Strings(expectedNodes)

	if len(actualNodes) != len(expectedNodes) {
		t.Errorf("Expected %d nodes %v, got %d nodes %v", len(expectedNodes), expectedNodes, len(actualNodes), actualNodes)
	}
}
