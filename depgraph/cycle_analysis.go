package depgraph

import "sort"

// AnalyzeCycleBreakSets populates verified break recommendations for cyclic
// components. It is explicit because exact/heuristic break analysis is more
// expensive than ordinary graph construction and is only needed by callers
// that present actionable cycle guidance.
func AnalyzeCycleBreakSets(components []FileCycle) {
	for index := range components {
		breakSets, exact := minimumCycleBreakSets(
			components[index].Nodes,
			components[index].Edges)
		components[index].BreakSets = breakSets
		components[index].BreakSetExact = exact
	}
}

const (
	exactBreakSetEdgeLimit  = 16
	maxBreakSetAlternatives = 8
)

func minimumCycleBreakSets(nodes []string, edges []FileEdge) ([]CycleBreakSet, bool) {
	if len(edges) == 0 {
		return nil, true
	}
	if len(edges) > exactBreakSetEdgeLimit {
		return []CycleBreakSet{{Edges: heuristicCycleBreakSet(nodes, edges)}}, false
	}

	for size := 1; size <= len(edges); size++ {
		var results []CycleBreakSet
		var choose func(start int, selected []int)
		choose = func(start int, selected []int) {
			if len(results) >= maxBreakSetAlternatives {
				return
			}
			if len(selected) == size {
				removed := make(map[FileEdge]bool, len(selected))
				breakEdges := make([]FileEdge, 0, len(selected))
				for _, index := range selected {
					edge := edges[index]
					removed[edge] = true
					breakEdges = append(breakEdges, edge)
				}
				if isAcyclicAfterRemoving(nodes, edges, removed) {
					results = append(results, CycleBreakSet{Edges: breakEdges})
				}
				return
			}
			remaining := size - len(selected)
			for i := start; i <= len(edges)-remaining; i++ {
				choose(i+1, append(selected, i))
			}
		}
		choose(0, nil)
		if len(results) > 0 {
			return results, true
		}
	}
	return nil, true
}

func heuristicCycleBreakSet(nodes []string, edges []FileEdge) []FileEdge {
	remaining := append([]FileEdge(nil), edges...)
	var removed []FileEdge
	for {
		if cyclicEdgeCount(nodes, remaining) == 0 {
			break
		}
		bestIndex := 0
		bestRemainingCycles := len(remaining) + 1
		for index := range remaining {
			candidate := make([]FileEdge, 0, len(remaining)-1)
			candidate = append(candidate, remaining[:index]...)
			candidate = append(candidate, remaining[index+1:]...)
			cycleEdges := cyclicEdgeCount(nodes, candidate)
			if cycleEdges < bestRemainingCycles {
				bestIndex = index
				bestRemainingCycles = cycleEdges
			}
		}
		edge := remaining[bestIndex]
		removed = append(removed, edge)
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
	}
	sortFileEdges(removed)
	return removed
}

func cyclicEdgeCount(nodes []string, edges []FileEdge) int {
	adjacency := adjacencyFromEdges(nodes, edges)
	count := 0
	for _, scc := range stronglyConnectedComponents(adjacency) {
		if !isCyclicSCC(adjacency, scc) {
			continue
		}
		allowed := make(map[string]bool, len(scc))
		for _, node := range scc {
			allowed[node] = true
		}
		for _, edge := range edges {
			if allowed[edge.From] && allowed[edge.To] {
				count++
			}
		}
	}
	return count
}

func isAcyclicAfterRemoving(
	nodes []string,
	edges []FileEdge,
	removed map[FileEdge]bool,
) bool {
	remaining := make([]FileEdge, 0, len(edges)-len(removed))
	for _, edge := range edges {
		if !removed[edge] {
			remaining = append(remaining, edge)
		}
	}
	adjacency := adjacencyFromEdges(nodes, remaining)
	for _, scc := range stronglyConnectedComponents(adjacency) {
		if isCyclicSCC(adjacency, scc) {
			return false
		}
	}
	return true
}

func adjacencyFromEdges(nodes []string, edges []FileEdge) map[string][]string {
	adjacency := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		adjacency[node] = nil
	}
	for _, edge := range edges {
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
	}
	for node := range adjacency {
		sort.Strings(adjacency[node])
	}
	return adjacency
}
