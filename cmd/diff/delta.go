package diff

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
)

type graphEdge struct {
	from string
	to   string
}

type graphDelta struct {
	nodesAdded   []string
	nodesRemoved []string
	edgesAdded   []graphEdge
	edgesRemoved []graphEdge
	findings     []string
	changedNodes map[string]struct{}
}

// SemanticAnalyzer computes optional semantic findings from two snapshots and their structural delta.
type SemanticAnalyzer func(base, target depgraph.DependencyGraph, delta graphDelta) ([]string, error)

func buildGraphDelta(base, target depgraph.DependencyGraph) (graphDelta, error) {
	baseAdj, err := depgraph.AdjacencyList(base)
	if err != nil {
		return graphDelta{}, fmt.Errorf("failed to read base adjacency: %w", err)
	}
	targetAdj, err := depgraph.AdjacencyList(target)
	if err != nil {
		return graphDelta{}, fmt.Errorf("failed to read target adjacency: %w", err)
	}

	baseNodes := collectNodes(baseAdj)
	targetNodes := collectNodes(targetAdj)

	delta := graphDelta{
		nodesAdded:   setDifference(targetNodes, baseNodes),
		nodesRemoved: setDifference(baseNodes, targetNodes),
		edgesAdded:   edgeDifference(collectEdges(targetAdj), collectEdges(baseAdj)),
		edgesRemoved: edgeDifference(collectEdges(baseAdj), collectEdges(targetAdj)),
	}

	sort.Strings(delta.nodesAdded)
	sort.Strings(delta.nodesRemoved)
	sort.Slice(delta.edgesAdded, func(i, j int) bool {
		leftFrom := filepath.Clean(delta.edgesAdded[i].from)
		rightFrom := filepath.Clean(delta.edgesAdded[j].from)
		if leftFrom == rightFrom {
			return filepath.Clean(delta.edgesAdded[i].to) < filepath.Clean(delta.edgesAdded[j].to)
		}
		return leftFrom < rightFrom
	})
	sort.Slice(delta.edgesRemoved, func(i, j int) bool {
		leftFrom := filepath.Clean(delta.edgesRemoved[i].from)
		rightFrom := filepath.Clean(delta.edgesRemoved[j].from)
		if leftFrom == rightFrom {
			return filepath.Clean(delta.edgesRemoved[i].to) < filepath.Clean(delta.edgesRemoved[j].to)
		}
		return leftFrom < rightFrom
	})

	return delta, nil
}

func applySemanticAnalyzers(base, target depgraph.DependencyGraph, delta graphDelta, analyzers []SemanticAnalyzer) (graphDelta, error) {
	if len(analyzers) == 0 {
		return delta, nil
	}

	findings := []string{}
	for _, analyzer := range analyzers {
		if analyzer == nil {
			continue
		}
		semanticFindings, err := analyzer(base, target, delta)
		if err != nil {
			return graphDelta{}, err
		}
		findings = append(findings, semanticFindings...)
	}
	sort.Strings(findings)
	delta.findings = findings
	return delta, nil
}

func collectNodes(adj map[string][]string) map[string]struct{} {
	nodes := make(map[string]struct{}, len(adj))
	for from, deps := range adj {
		nodes[from] = struct{}{}
		for _, to := range deps {
			nodes[to] = struct{}{}
		}
	}
	return nodes
}

func setDifference(left, right map[string]struct{}) []string {
	result := []string{}
	for v := range left {
		if _, ok := right[v]; !ok {
			result = append(result, v)
		}
	}
	return result
}

func collectEdges(adj map[string][]string) map[graphEdge]struct{} {
	edges := make(map[graphEdge]struct{})
	for from, deps := range adj {
		for _, to := range deps {
			edges[graphEdge{from: from, to: to}] = struct{}{}
		}
	}
	return edges
}

func edgeDifference(left, right map[graphEdge]struct{}) []graphEdge {
	result := []graphEdge{}
	for edge := range left {
		if _, ok := right[edge]; !ok {
			result = append(result, edge)
		}
	}
	return result
}

func renderSummary(delta graphDelta) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Nodes added: %d", len(delta.nodesAdded)))
	lines = append(lines, delta.nodesAdded...)
	lines = append(lines, fmt.Sprintf("Nodes removed: %d", len(delta.nodesRemoved)))
	lines = append(lines, delta.nodesRemoved...)
	lines = append(lines, fmt.Sprintf("Edges added: %d", len(delta.edgesAdded)))
	for _, e := range delta.edgesAdded {
		lines = append(lines, fmt.Sprintf("%s -> %s", e.from, e.to))
	}
	lines = append(lines, fmt.Sprintf("Edges removed: %d", len(delta.edgesRemoved)))
	for _, e := range delta.edgesRemoved {
		lines = append(lines, fmt.Sprintf("%s -> %s", e.from, e.to))
	}
	lines = append(lines, fmt.Sprintf("Semantic findings: %d", len(delta.findings)))
	lines = append(lines, delta.findings...)
	return strings.Join(lines, "\n")
}

func renderDelta(format string, delta graphDelta) (string, error) {
	parsed, ok := formatters.ParseOutputFormat(format)
	if !ok {
		return "", fmt.Errorf("unknown format: %s (valid options: %s)", format, formatters.SupportedFormats())
	}

	switch parsed {
	case formatters.OutputFormatDOT:
		return renderDeltaDOT(delta), nil
	case formatters.OutputFormatMermaid:
		return renderDeltaMermaid(delta), nil
	default:
		return "", fmt.Errorf("unknown format: %s (valid options: %s)", format, formatters.SupportedFormats())
	}
}

func renderDeltaDOT(delta graphDelta) string {
	var b strings.Builder
	b.WriteString("digraph diff {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box];\n")

	changedNodes := sortedChangedNodes(delta.changedNodes)
	for _, n := range changedNodes {
		b.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=\"#d9f2d9\", color=\"#2e8b57\"];\n", n, filepath.Base(n)))
	}
	for _, n := range delta.nodesAdded {
		b.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=\"#d9f2d9\", color=\"#2e8b57\"];\n", n, filepath.Base(n)))
	}
	for _, n := range delta.nodesRemoved {
		b.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=\"#f8d7da\", color=\"#b22222\"];\n", n, filepath.Base(n)))
	}

	for _, e := range delta.edgesAdded {
		b.WriteString(fmt.Sprintf("  %q -> %q [color=\"#2e8b57\"];\n", e.from, e.to))
	}
	for _, e := range delta.edgesRemoved {
		b.WriteString(fmt.Sprintf("  %q -> %q [color=\"#b22222\", style=dashed];\n", e.from, e.to))
	}

	b.WriteString("}\n")
	return b.String()
}

func renderDeltaMermaid(delta graphDelta) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	nodeIDs := make(map[string]string)
	nodes := sortedChangedNodes(delta.changedNodes)
	nodes = append(nodes, delta.nodesAdded...)
	nodes = append(nodes, delta.nodesRemoved...)
	nodes = dedupeSortedStrings(nodes)
	for i, n := range nodes {
		id := fmt.Sprintf("n%d", i)
		nodeIDs[n] = id
		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, filepath.Base(n)))
	}

	for _, e := range delta.edgesAdded {
		fromID := nodeIDs[e.from]
		if fromID == "" {
			fromID = fmt.Sprintf("anon_%d", len(nodeIDs))
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", fromID, filepath.Base(e.from)))
			nodeIDs[e.from] = fromID
		}
		toID := nodeIDs[e.to]
		if toID == "" {
			toID = fmt.Sprintf("anon_%d", len(nodeIDs))
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", toID, filepath.Base(e.to)))
			nodeIDs[e.to] = toID
		}
		b.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
	}
	for _, e := range delta.edgesRemoved {
		fromID := nodeIDs[e.from]
		if fromID == "" {
			fromID = fmt.Sprintf("anon_%d", len(nodeIDs))
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", fromID, filepath.Base(e.from)))
			nodeIDs[e.from] = fromID
		}
		toID := nodeIDs[e.to]
		if toID == "" {
			toID = fmt.Sprintf("anon_%d", len(nodeIDs))
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", toID, filepath.Base(e.to)))
			nodeIDs[e.to] = toID
		}
		b.WriteString(fmt.Sprintf("    %s -.-> %s\n", fromID, toID))
	}

	if len(delta.changedNodes) > 0 {
		addedClasses := make([]string, 0, len(delta.changedNodes))
		for _, n := range sortedChangedNodes(delta.changedNodes) {
			if id := nodeIDs[n]; id != "" {
				addedClasses = append(addedClasses, id)
			}
		}
		if len(addedClasses) > 0 {
			b.WriteString("    classDef added fill:#d9f2d9,stroke:#2e8b57,color:#000000\n")
			b.WriteString(fmt.Sprintf("    class %s added\n", strings.Join(addedClasses, ",")))
		}
	}
	if len(delta.nodesRemoved) > 0 {
		removedClasses := make([]string, 0, len(delta.nodesRemoved))
		for _, n := range delta.nodesRemoved {
			if id := nodeIDs[n]; id != "" {
				removedClasses = append(removedClasses, id)
			}
		}
		if len(removedClasses) > 0 {
			b.WriteString("    classDef removed fill:#f8d7da,stroke:#b22222,color:#000000\n")
			b.WriteString(fmt.Sprintf("    class %s removed\n", strings.Join(removedClasses, ",")))
		}
	}

	unchangedClasses := make([]string, 0, len(nodeIDs))
	for path, id := range nodeIDs {
		if _, changed := delta.changedNodes[path]; changed {
			continue
		}
		unchangedClasses = append(unchangedClasses, id)
	}
	sort.Strings(unchangedClasses)
	if len(unchangedClasses) > 0 {
		b.WriteString("    classDef unchanged fill:#f5f6f8,stroke:#c3c7cf,color:#667085,stroke-dasharray: 5 3\n")
		b.WriteString(fmt.Sprintf("    class %s unchanged\n", strings.Join(unchangedClasses, ",")))
	}

	return b.String()
}

func sortedChangedNodes(changed map[string]struct{}) []string {
	if len(changed) == 0 {
		return nil
	}
	nodes := make([]string, 0, len(changed))
	for n := range changed {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	return nodes
}

func dedupeSortedStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	sort.Strings(values)
	result := make([]string, 0, len(values))
	prev := ""
	for i, value := range values {
		if i == 0 || value != prev {
			result = append(result, value)
			prev = value
		}
	}
	return result
}
