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
