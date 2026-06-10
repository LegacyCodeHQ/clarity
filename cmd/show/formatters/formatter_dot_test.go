package formatters

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/require"
)

func testGraph(adjacency map[string][]string) depgraph.DependencyGraph {
	return depgraph.MustDependencyGraph(adjacency)
}

func testFileGraph(t *testing.T, adjacency map[string][]string, stats map[string]vcs.FileStats) depgraph.FileDependencyGraph {
	t.Helper()
	fileGraph, err := depgraph.NewFileDependencyGraph(testGraph(adjacency), stats, nil)
	require.NoError(t, err)
	return fileGraph
}

func TestDependencyGraph_ToDOT(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_ModuleNodeShowsFileCountAndChurn(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"X":                  {"/project/util.dart"},
		"/project/util.dart": {},
	}, nil)

	moduleMeta := graph.Meta.Files["X"]
	moduleMeta.IsModule = true
	moduleMeta.ModuleFileCount = 3
	moduleMeta.Stats = &vcs.FileStats{Additions: 50, Deletions: 10}
	graph.Meta.Files["X"] = moduleMeta

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	require.Contains(t, output, `label="X\n3 files\n+50 -10"`)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_ModuleNodeUsesComponentShape(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"X":                   {"/project/util.dart"},
		"/project/util.dart":  {},
		"/project/other.dart": {"X"},
	}, nil)

	moduleMeta := graph.Meta.Files["X"]
	moduleMeta.IsModule = true
	graph.Meta.Files["X"] = moduleMeta

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	require.Contains(t, output, "\"X\" [label=\"X\", shape=component, style=filled, fillcolor=lightyellow];")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_CustomDirection(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.dart": {"/project/utils.dart"},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionTB})
	require.NoError(t, err)
	require.Contains(t, output, "rankdir=TB;")
}

func TestDependencyGraph_ToDOT_DirectionLR(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionLR})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_DirectionRL(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionRL})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_DirectionTB(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionTB})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_DirectionBT(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{Direction: DirectionBT})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_NewFilesUseSeedlingLabel(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
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

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen_Dart(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/lib/main.dart":        {"/project/lib/utils.dart"},
		"/project/lib/utils.dart":       {},
		"/project/test/main_test.dart":  {"/project/lib/main.dart"},
		"/project/test/utils_test.dart": {"/project/lib/utils.dart"},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.go":          {"/project/utils.go"},
		"/project/utils.go":         {},
		"/project/output_format.go": {},
		"/project/helpers.go":       {},
		"/project/config.go":        {},
		"/project/main.dart":        {},
		"/project/utils.dart":       {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite_WithTestFiles(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.go":          {"/project/utils.go"},
		"/project/utils.go":         {},
		"/project/output_format.go": {},
		"/project/main_test.go":     {"/project/main.go"},
		"/project/utils_test.go":    {"/project/utils.go"},
		"/project/main.dart":        {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_MajorityExtensionTie(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.go":    {},
		"/project/utils.go":   {},
		"/project/main.dart":  {},
		"/project/utils.dart": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_SingleExtensionAllWhite(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.go":          {"/project/utils.go"},
		"/project/utils.go":         {},
		"/project/output_format.go": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_TypeScriptTestFiles(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/src/App.tsx":                    {"/project/src/utils.tsx"},
		"/project/src/utils.tsx":                  {},
		"/project/src/App.test.tsx":               {"/project/src/App.tsx"},
		"/project/src/__tests__/utils.test.tsx":   {"/project/src/utils.tsx"},
		"/project/src/components/Button.spec.tsx": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_NodesAreDeclaredOnlyOnce(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/standalone.go": {},
		"/project/config.go":     {"/project/standalone.go"},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_HighlightsCycles(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/a.go": {"/project/b.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {"/project/a.go"},
		"/project/d.go": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_HighlightsAllCycleEdgesInSCC(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/a.go"},
		"/project/c.go": {"/project/a.go"},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_PrunedNodesHaveDashedBorder(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/a.go": {"/project/b.go"},
		"/project/b.go": {},
	}, nil)

	// Mark b.go as pruned
	md := graph.Meta.Files["/project/b.go"]
	md.IsPruned = true
	graph.Meta.Files["/project/b.go"] = md

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_EdgeLabels(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{EdgeLabels: true})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_ModuleEdgesKeepOriginalDependencyLabels(t *testing.T) {
	// a.go depended on b.go and c.go (both collapsed into module M) and on the
	// untouched d.go. With labels on, each original dependency must survive as
	// its own arrow into M, labeled by its original endpoints; the untouched
	// edge keeps its own label.
	const base = "/p"
	graph := testFileGraph(t, map[string][]string{
		"/p/a.go": {"M", "/p/d.go"},
		"/p/d.go": {},
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

	output, err := (&dotFormatter{}).Format(graph, RenderOptions{BasePath: base, EdgeLabels: true})
	require.NoError(t, err)

	require.Contains(t, output, fmt.Sprintf("\"a.go\" -> \"M\" [label=%q]", EdgeLabel("a.go", "b.go")))
	require.Contains(t, output, fmt.Sprintf("\"a.go\" -> \"M\" [label=%q]", EdgeLabel("a.go", "c.go")))
	require.Contains(t, output, fmt.Sprintf("\"a.go\" -> \"d.go\" [label=%q]", EdgeLabel("a.go", "d.go")))
}

func TestDependencyGraph_ToDOT_EdgeLabelsStableWhenSiblingCollapsed(t *testing.T) {
	// main.go -> app/util.go is the edge under test. A second util.go
	// (lib/util.go) forces display-name disambiguation in the full graph but
	// vanishes once collapsed into a module. The edge label for the unchanged
	// main.go -> app/util.go dependency must not change as a result.
	const base = "/project"
	adjacency := map[string][]string{
		"/project/main.go":     {"/project/app/util.go"},
		"/project/app/util.go": {},
		"/project/lib/util.go": {"/project/main.go"},
	}

	full := testFileGraph(t, adjacency, nil)
	fullOut, err := (&dotFormatter{}).Format(full, RenderOptions{BasePath: base, EdgeLabels: true})
	require.NoError(t, err)

	collapsedGraph, _, err := depgraph.CollapseModules(testGraph(adjacency), []depgraph.Module{
		{Name: "M", Files: []string{"/project/lib/util.go"}},
	})
	require.NoError(t, err)
	collapsed, err := depgraph.NewFileDependencyGraph(collapsedGraph, nil, nil)
	require.NoError(t, err)
	collapsedOut, err := (&dotFormatter{}).Format(collapsed, RenderOptions{BasePath: base, EdgeLabels: true})
	require.NoError(t, err)

	labelRE := regexp.MustCompile(`"main\.go" -> "app/util\.go" \[label="([a-z]{3})"\]`)
	fullMatch := labelRE.FindStringSubmatch(fullOut)
	collapsedMatch := labelRE.FindStringSubmatch(collapsedOut)
	require.NotNil(t, fullMatch, "edge not found in full graph output:\n%s", fullOut)
	require.NotNil(t, collapsedMatch, "edge not found in collapsed graph output:\n%s", collapsedOut)

	require.Equal(t, fullMatch[1], collapsedMatch[1],
		"edge label for main.go -> app/util.go changed after collapsing an unrelated sibling")
}

func TestDependencyGraph_ToDOT_DuplicateBaseNamesStayDistinct(t *testing.T) {
	graph := testFileGraph(t, map[string][]string{
		"/project/test/res.send.js":      {"/project/test/support/utils.js"},
		"/project/test/support/utils.js": {},
		"/project/lib/utils.js":          {},
	}, nil)

	formatter := dotFormatter{}
	output, err := formatter.Format(graph, RenderOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_ExtensionColorsRemainStableAcrossSequentialRenders(t *testing.T) {
	formatter := dotFormatter{}

	firstGraph := testFileGraph(t, map[string][]string{
		"/project/main.go":  {},
		"/project/app.py":   {},
		"/project/util.py":  {},
		"/project/extra.py": {},
	}, nil)

	firstOutput, err := formatter.Format(firstGraph, RenderOptions{})
	require.NoError(t, err)
	firstGoColor, ok := findFillColorForLabel(firstOutput, "main.go")
	require.True(t, ok, "expected main.go to have a fill color in first render")
	require.NotEqual(t, "white", firstGoColor, "setup error: main.go should not be majority extension in first render")

	secondGraph := testFileGraph(t, map[string][]string{
		"/project/main.go":  {},
		"/project/app.py":   {},
		"/project/util.py":  {},
		"/project/extra.py": {},
		"/project/engine.c": {},
	}, nil)

	secondOutput, err := formatter.Format(secondGraph, RenderOptions{})
	require.NoError(t, err)
	secondGoColor, ok := findFillColorForLabel(secondOutput, "main.go")
	require.True(t, ok, "expected main.go to have a fill color in second render")

	require.Equal(
		t,
		firstGoColor,
		secondGoColor,
		"expected .go extension color to remain stable across sequential renders when extension set changes")
}

func findFillColorForLabel(dotOutput, label string) (string, bool) {
	needle := fmt.Sprintf("label=%q", label)
	for _, line := range strings.Split(dotOutput, "\n") {
		if !strings.Contains(line, needle) {
			continue
		}

		idx := strings.Index(line, "fillcolor=")
		if idx == -1 {
			return "", false
		}

		colorPart := line[idx+len("fillcolor="):]
		if comma := strings.IndexByte(colorPart, ','); comma >= 0 {
			colorPart = colorPart[:comma]
		}
		if bracket := strings.IndexByte(colorPart, ']'); bracket >= 0 {
			colorPart = colorPart[:bracket]
		}
		return strings.TrimSpace(colorPart), true
	}
	return "", false
}
