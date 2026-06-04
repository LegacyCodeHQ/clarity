package formatters

import (
	"hash/fnv"
	"sort"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

// EdgeLabel returns a deterministic 3-character alphanumeric label for the
// directed edge from → to. The same pair always produces the same label.
func EdgeLabel(from, to string) string {
	h := fnv.New64a()
	h.Write([]byte(from))
	h.Write([]byte{0})
	h.Write([]byte(to))
	return encodeAlpha(h.Sum64(), 3)
}

// edgeLabels returns the deterministic labels to render for a rendered edge
// source → dep. When the edge collapses one or more original dependencies into
// a module (recorded in EdgeOrigins), it yields one label per original edge,
// keyed by the original endpoints, so labels are unchanged by collapsing.
// Otherwise it yields a single label for the edge's own endpoints.
func edgeLabels(g depgraph.FileDependencyGraph, source, dep, basePath string) []string {
	origins := g.Meta.EdgeOrigins[depgraph.FileEdge{From: source, To: dep}]
	if len(origins) == 0 {
		return []string{EdgeLabel(nodeKey(source, basePath), nodeKey(dep, basePath))}
	}

	sorted := append([]depgraph.FileEdge(nil), origins...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].From != sorted[j].From {
			return sorted[i].From < sorted[j].From
		}
		return sorted[i].To < sorted[j].To
	})

	labels := make([]string, 0, len(sorted))
	for _, origin := range sorted {
		labels = append(labels, EdgeLabel(nodeKey(origin.From, basePath), nodeKey(origin.To, basePath)))
	}
	return labels
}

const alphaChars = "abcdefghijklmnopqrstuvwxyz"

func encodeAlpha(n uint64, length int) string {
	buf := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		buf[i] = alphaChars[n%26]
		n /= 26
	}
	return string(buf)
}
