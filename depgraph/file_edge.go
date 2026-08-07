package depgraph

import "sort"

// FileEdge identifies a directed edge between two files.
type FileEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func sortFileEdges(edges []FileEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
}
