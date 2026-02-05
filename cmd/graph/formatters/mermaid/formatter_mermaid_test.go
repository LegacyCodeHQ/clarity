package mermaid_test

import (
	"testing"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters/mermaid"
	"github.com/LegacyCodeHQ/sanity/depgraph"
	"github.com/LegacyCodeHQ/sanity/internal/testhelpers"
	"github.com/LegacyCodeHQ/sanity/vcs"
	"github.com/stretchr/testify/require"
)

func TestMermaidFormatter_BasicFlowchart(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_WithLabel(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/main.dart": {},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{Label: "My Graph"})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_WithoutLabel(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/main.dart": {},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_NewFilesUseSeedlingLabel(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/new_file.dart":       {},
		"/project/new_with_stats.dart": {},
		"/project/existing.dart":       {},
	}

	stats := map[string]vcs.FileStats{
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
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_TestFilesAreStyled(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_DartTestFiles(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/lib/main.dart":        {"/project/lib/utils.dart"},
		"/project/lib/utils.dart":       {},
		"/project/test/main_test.dart":  {"/project/lib/main.dart"},
		"/project/test/utils_test.dart": {"/project/lib/utils.dart"},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_NewFilesAreStyled(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/new_file.dart":  {},
		"/project/existing.dart":  {},
		"/project/another_new.go": {},
	}

	stats := map[string]vcs.FileStats{
		"/project/new_file.dart": {
			IsNew: true,
		},
		"/project/another_new.go": {
			IsNew: true,
		},
		"/project/existing.dart": {
			Additions: 5,
		},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_TypeScriptTestFiles(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/src/App.tsx":                    {"/project/src/utils.tsx"},
		"/project/src/utils.tsx":                  {},
		"/project/src/App.test.tsx":               {"/project/src/App.tsx"},
		"/project/src/__tests__/utils.test.tsx":   {"/project/src/utils.tsx"},
		"/project/src/components/Button.spec.tsx": {},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_EdgesBetweenNodes(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_QuoteEscaping(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/file.go": {},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_EmptyGraph(t *testing.T) {
	graph := depgraph.DependencyGraph{}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_FileStatsWithOnlyAdditions(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/modified.go": {},
	}

	stats := map[string]vcs.FileStats{
		"/project/modified.go": {
			Additions: 10,
			Deletions: 0,
		},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_FileStatsWithOnlyDeletions(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/modified.go": {},
	}

	stats := map[string]vcs.FileStats{
		"/project/modified.go": {
			Additions: 0,
			Deletions: 5,
		},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestMermaidFormatter_TestFileTakesPriorityOverNewFile(t *testing.T) {
	graph := depgraph.DependencyGraph{
		"/project/main_test.go": {},
	}

	stats := map[string]vcs.FileStats{
		"/project/main_test.go": {
			IsNew: true,
		},
	}

	formatter := &mermaid.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}
