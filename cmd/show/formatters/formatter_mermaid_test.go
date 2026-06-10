package formatters

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/require"
)

func testGraphMermaid(adjacency map[string][]string) depgraph.DependencyGraph {
	return depgraph.MustDependencyGraph(adjacency)
}

func testFileGraphMermaid(t *testing.T, adjacency map[string][]string, stats map[string]vcs.FileStats) depgraph.FileDependencyGraph {
	t.Helper()
	fileGraph, err := depgraph.NewFileDependencyGraph(testGraphMermaid(adjacency), stats, nil)
	require.NoError(t, err)
	return fileGraph
}

func TestMermaidFormatter_BasicFlowchart(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_RenamedNodeShowsMarkerAndStyle(t *testing.T) {
	// Parity with the DOT formatter: a renamed node carries the "(renamed)"
	// marker and a dashed style. The Mermaid formatter previously dropped both.
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/old.dart": {},
		"/project/app.dart": {"/project/old.dart"},
	}, nil)

	md := graph.Meta.Files["/project/old.dart"]
	md.State = depgraph.FileStateRenamed
	graph.Meta.Files["/project/old.dart"] = md

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	require.Contains(t, output, "(renamed)")
	require.Contains(t, output, "classDef renamedFile")
}

func TestMermaidFormatter_RenamedEdgeIsStyled(t *testing.T) {
	// Parity with the DOT formatter: a renamed edge is drawn dashed amber. The
	// Mermaid formatter previously left it as a plain edge.
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/old.dart": {"/project/new.dart"},
		"/project/new.dart": {},
	}, nil)

	edge := depgraph.FileEdge{From: "/project/old.dart", To: "/project/new.dart"}
	em := graph.Meta.Edges[edge]
	em.State = depgraph.EdgeStateRenamed
	graph.Meta.Edges[edge] = em

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	require.Contains(t, output, "linkStyle 0 stroke:#CC8800")
}

func TestMermaidFormatter_ModuleNodeUsesSubroutineShapeWithFileCountAndChurn(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"X":                   {"/project/util.dart"},
		"/project/util.dart":  {},
		"/project/other.dart": {"X"},
	}, nil)

	moduleMeta := graph.Meta.Files["X"]
	moduleMeta.IsModule = true
	moduleMeta.ModuleFileCount = 3
	moduleMeta.Stats = &vcs.FileStats{Additions: 50, Deletions: 10}
	graph.Meta.Files["X"] = moduleMeta

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	require.Contains(t, output, "[[\"X<br/>3 files<br/>+50 -10\"]]")
	require.Contains(t, output, "classDef moduleNode")

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_CustomDirection(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart": {"/project/utils.dart"},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionBT})
	require.NoError(t, err)
	require.Contains(t, output, "flowchart BT")
}

func TestMermaidFormatter_DirectionLR(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionLR})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_DirectionRL(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionRL})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_DirectionTB(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionTB})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_DirectionBT(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionBT})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_WithLabel(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Label: "My Graph"})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_WithoutLabel(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.dart": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_NewFilesUseSeedlingLabel(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/new_file.dart":       {},
		"/project/new_with_stats.dart": {},
		"/project/existing.dart":       {},
	}, map[string]vcs.FileStats{
		"/project/new_file.dart": {
			IsNew: true,
		},
		"/project/new_with_stats.dart": {
			IsNew:     true,
			Additions: 12,
			Deletions: 1,
		},
		"/project/existing.dart": {
			Additions: 3,
		},
	})

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_TestFilesAreStyled(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_DartTestFiles(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/lib/main.dart":        {"/project/lib/utils.dart"},
		"/project/lib/utils.dart":       {},
		"/project/test/main_test.dart":  {"/project/lib/main.dart"},
		"/project/test/utils_test.dart": {"/project/lib/utils.dart"},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_NewFilesAreStyled(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/new_file.dart":  {},
		"/project/existing.dart":  {},
		"/project/another_new.go": {},
	}, map[string]vcs.FileStats{
		"/project/new_file.dart": {
			IsNew: true,
		},
		"/project/another_new.go": {
			IsNew: true,
		},
		"/project/existing.dart": {
			Additions: 5,
		},
	})

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_TypeScriptTestFiles(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/src/App.tsx":                    {"/project/src/utils.tsx"},
		"/project/src/utils.tsx":                  {},
		"/project/src/App.test.tsx":               {"/project/src/App.tsx"},
		"/project/src/__tests__/utils.test.tsx":   {"/project/src/utils.tsx"},
		"/project/src/components/Button.spec.tsx": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_EdgesBetweenNodes(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_QuoteEscaping(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/file.go": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_EmptyGraph(t *testing.T) {
	graph := testFileGraphMermaid(t, make(map[string][]string), nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_FileStatsWithOnlyAdditions(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/modified.go": {},
	}, map[string]vcs.FileStats{
		"/project/modified.go": {
			Additions: 10,
			Deletions: 0,
		},
	})

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_FileStatsWithOnlyDeletions(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/modified.go": {},
	}, map[string]vcs.FileStats{
		"/project/modified.go": {
			Additions: 0,
			Deletions: 5,
		},
	})

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_TestFileTakesPriorityOverNewFile(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/main_test.go": {},
	}, map[string]vcs.FileStats{
		"/project/main_test.go": {
			IsNew: true,
		},
	})

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_HighlightsCycles(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/a.go": {"/project/b.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {"/project/a.go"},
		"/project/d.go": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_HighlightsAllCycleEdgesInSCC(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/a.go"},
		"/project/c.go": {"/project/a.go"},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	require.Contains(t, output, "style n0 stroke:#d62728")
	require.Contains(t, output, "style n1 stroke:#d62728")
	require.Contains(t, output, "style n2 stroke:#d62728")
	require.Contains(t, output, "linkStyle 0 stroke:#d62728")
	require.Contains(t, output, "linkStyle 1 stroke:#d62728")
	require.Contains(t, output, "linkStyle 2 stroke:#d62728")
	require.Contains(t, output, "linkStyle 3 stroke:#d62728")
}

func TestMermaidFormatter_PrunedNodesHaveDashedBorder(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/a.go": {"/project/b.go"},
		"/project/b.go": {},
	}, nil)

	// Mark b.go as pruned
	md := graph.Meta.Files["/project/b.go"]
	md.IsPruned = true
	graph.Meta.Files["/project/b.go"] = md

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_EdgeLabels(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{EdgeLabels: true})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_ModuleEdgesKeepOriginalDependencyLabels(t *testing.T) {
	// a.go depended on b.go and c.go (collapsed into module M). With labels on,
	// both dependencies survive as their own arrow into M, labeled by the
	// original endpoints.
	const base = "/p"
	graph := testFileGraphMermaid(t, map[string][]string{
		"/p/a.go": {"M"},
		"M":       {},
	}, nil)

	md := graph.Meta.Files["M"]
	md.IsModule = true
	graph.Meta.Files["M"] = md
	graph.Meta.EdgeOrigins = map[depgraph.FileEdge][]depgraph.FileEdge{
		{From: "/p/a.go", To: "M"}: {
			{From: "/p/a.go", To: "/p/b.go"},
			{From: "/p/a.go", To: "/p/c.go"},
		},
	}

	output, err := mermaidFormatter{}.Format(graph, RenderOptions{BasePath: base, EdgeLabels: true})
	require.NoError(t, err)

	require.Contains(t, output, "|"+EdgeLabel("a.go", "b.go")+"|")
	require.Contains(t, output, "|"+EdgeLabel("a.go", "c.go")+"|")
}

func TestMermaidFormatter_EdgeLabelsStableWhenSiblingCollapsed(t *testing.T) {
	// The main.go -> app/util.go edge label must not change when an unrelated
	// sibling (lib/util.go) is collapsed into a module, even though doing so
	// changes how util.go is disambiguated.
	const base = "/project"
	adjacency := map[string][]string{
		"/project/main.go":     {"/project/app/util.go"},
		"/project/app/util.go": {},
		"/project/lib/util.go": {"/project/main.go"},
	}

	full := testFileGraphMermaid(t, adjacency, nil)
	fullOut, err := mermaidFormatter{}.Format(full, RenderOptions{BasePath: base, EdgeLabels: true})
	require.NoError(t, err)

	collapsedGraph, _, err := depgraph.CollapseModules(testGraphMermaid(adjacency), []depgraph.Module{
		{Name: "M", Files: []string{"/project/lib/util.go"}},
	})
	require.NoError(t, err)
	collapsed, err := depgraph.NewFileDependencyGraph(collapsedGraph, nil, nil)
	require.NoError(t, err)
	collapsedOut, err := mermaidFormatter{}.Format(collapsed, RenderOptions{BasePath: base, EdgeLabels: true})
	require.NoError(t, err)

	want := EdgeLabel("main.go", "app/util.go")
	require.Contains(t, fullOut, "|"+want+"|")
	require.Contains(t, collapsedOut, "|"+want+"|")
}

func TestMermaidFormatter_DuplicateBaseNamesStayDistinct(t *testing.T) {
	graph := testFileGraphMermaid(t, map[string][]string{
		"/project/test/res.send.js":      {"/project/test/support/utils.js"},
		"/project/test/support/utils.js": {},
		"/project/lib/utils.js":          {},
	}, nil)

	formatter := mermaidFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}
