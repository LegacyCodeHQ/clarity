package diff

import (
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

func TestBuildGraphDelta_ComputesAddedRemovedNodesAndEdges(t *testing.T) {
	base := depgraph.MustDependencyGraph(map[string][]string{
		"/repo/a.go": {"/repo/b.go"},
		"/repo/b.go": {},
	})
	target := depgraph.MustDependencyGraph(map[string][]string{
		"/repo/a.go": {"/repo/c.go"},
		"/repo/c.go": {},
	})

	delta, err := buildGraphDelta(base, target)
	if err != nil {
		t.Fatalf("buildGraphDelta() error = %v", err)
	}

	if len(delta.nodesAdded) != 1 || delta.nodesAdded[0] != "/repo/c.go" {
		t.Fatalf("unexpected nodesAdded: %+v", delta.nodesAdded)
	}
	if len(delta.nodesRemoved) != 1 || delta.nodesRemoved[0] != "/repo/b.go" {
		t.Fatalf("unexpected nodesRemoved: %+v", delta.nodesRemoved)
	}

	if len(delta.edgesAdded) != 1 || delta.edgesAdded[0] != (graphEdge{from: "/repo/a.go", to: "/repo/c.go"}) {
		t.Fatalf("unexpected edgesAdded: %+v", delta.edgesAdded)
	}
	if len(delta.edgesRemoved) != 1 || delta.edgesRemoved[0] != (graphEdge{from: "/repo/a.go", to: "/repo/b.go"}) {
		t.Fatalf("unexpected edgesRemoved: %+v", delta.edgesRemoved)
	}
}

func TestRenderSummary_DeterministicOrder(t *testing.T) {
	delta := graphDelta{
		nodesAdded:   []string{"/repo/z.go", "/repo/a.go"},
		nodesRemoved: []string{"/repo/c.go"},
		edgesAdded: []graphEdge{
			{from: "/repo/z.go", to: "/repo/a.go"},
		},
		edgesRemoved: []graphEdge{
			{from: "/repo/c.go", to: "/repo/a.go"},
		},
		findings: []string{"new cycle in /repo/z.go"},
	}

	out := renderSummary(delta)
	wantSections := []string{"Nodes added:", "Nodes removed:", "Edges added:", "Edges removed:", "Semantic findings:"}
	lastIndex := -1
	for _, section := range wantSections {
		idx := strings.Index(out, section)
		if idx == -1 {
			t.Fatalf("missing summary section %q in:\n%s", section, out)
		}
		if idx <= lastIndex {
			t.Fatalf("section %q out of order in:\n%s", section, out)
		}
		lastIndex = idx
	}
}

func TestApplySemanticAnalyzers_SortedAndAggregated(t *testing.T) {
	base := depgraph.MustDependencyGraph(make(map[string][]string))
	target := depgraph.MustDependencyGraph(make(map[string][]string))
	delta := graphDelta{}

	analyzers := []SemanticAnalyzer{
		func(base, target depgraph.DependencyGraph, delta graphDelta) ([]string, error) {
			return []string{"b-finding"}, nil
		},
		func(base, target depgraph.DependencyGraph, delta graphDelta) ([]string, error) {
			return []string{"a-finding"}, nil
		},
	}

	out, err := applySemanticAnalyzers(base, target, delta, analyzers)
	if err != nil {
		t.Fatalf("applySemanticAnalyzers() error = %v", err)
	}
	if len(out.findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(out.findings))
	}
	if out.findings[0] != "a-finding" || out.findings[1] != "b-finding" {
		t.Fatalf("findings are not sorted: %+v", out.findings)
	}
}

func TestRenderDelta_UnknownFormat(t *testing.T) {
	_, err := renderDelta("json", graphDelta{})
	if err == nil {
		t.Fatal("expected unknown format error")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderDeltaMermaid_UnchangedNodesUseDottedBorder(t *testing.T) {
	delta := graphDelta{
		edgesAdded: []graphEdge{
			{from: "/repo/changed.go", to: "/repo/unchanged.go"},
			{from: "/repo/changed.go", to: "/repo/changed2.go"},
			{from: "/repo/unchanged.go", to: "/repo/unchanged2.go"},
		},
		changedNodes: map[string]struct{}{
			"/repo/changed.go":  {},
			"/repo/changed2.go": {},
		},
	}

	out := renderDeltaMermaid(delta)
	if strings.Contains(out, "✨") {
		t.Fatalf("expected plain edges without sparkle labels, got:\n%s", out)
	}
	if !strings.Contains(out, "classDef unchanged") || !strings.Contains(out, "stroke-dasharray") {
		t.Fatalf("expected unchanged class with dotted border, got:\n%s", out)
	}
}
