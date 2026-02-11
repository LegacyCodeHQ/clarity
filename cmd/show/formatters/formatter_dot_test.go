package formatters

import (
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

	require.Contains(t, output, "\"a.go\" [label=\"a.go\", style=filled, fillcolor=white, color=red];")
	require.Contains(t, output, "\"b.go\" [label=\"b.go\", style=filled, fillcolor=white, color=red];")
	require.Contains(t, output, "\"c.go\" [label=\"c.go\", style=filled, fillcolor=white, color=red];")
	require.Contains(t, output, "\"a.go\" -> \"b.go\" [color=red, style=dashed];")
	require.Contains(t, output, "\"a.go\" -> \"c.go\" [color=red, style=dashed];")
	require.Contains(t, output, "\"b.go\" -> \"a.go\" [color=red, style=dashed];")
	require.Contains(t, output, "\"c.go\" -> \"a.go\" [color=red, style=dashed];")
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
