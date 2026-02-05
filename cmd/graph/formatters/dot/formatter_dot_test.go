package dot_test

import (
	"testing"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters/dot"
	"github.com/LegacyCodeHQ/sanity/depgraph"
	"github.com/LegacyCodeHQ/sanity/internal/testhelpers"
	"github.com/LegacyCodeHQ/sanity/vcs"
	"github.com/stretchr/testify/require"
)

func testGraph(adjacency map[string][]string) depgraph.DependencyGraph {
	return depgraph.MustDependencyGraph(adjacency)
}

func TestDependencyGraph_ToDOT(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_NewFilesUseSeedlingLabel(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/new_file.dart":       {},
		"/project/new_with_stats.dart": {},
		"/project/existing.dart":       {},
	})

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

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen_Dart(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/lib/main.dart":        {"/project/lib/utils.dart"},
		"/project/lib/utils.dart":       {},
		"/project/test/main_test.dart":  {"/project/lib/main.dart"},
		"/project/test/utils_test.dart": {"/project/lib/utils.dart"},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/main.go":          {"/project/utils.go"},
		"/project/utils.go":         {},
		"/project/output_format.go": {},
		"/project/helpers.go":       {},
		"/project/config.go":        {},
		"/project/main.dart":        {},
		"/project/utils.dart":       {},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite_WithTestFiles(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/main.go":          {"/project/utils.go"},
		"/project/utils.go":         {},
		"/project/output_format.go": {},
		"/project/main_test.go":     {"/project/main.go"},
		"/project/utils_test.go":    {"/project/utils.go"},
		"/project/main.dart":        {},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_MajorityExtensionTie(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/main.go":    {},
		"/project/utils.go":   {},
		"/project/main.dart":  {},
		"/project/utils.dart": {},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_SingleExtensionAllWhite(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/main.go":          {"/project/utils.go"},
		"/project/utils.go":         {},
		"/project/output_format.go": {},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_TypeScriptTestFiles(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/src/App.tsx":                    {"/project/src/utils.tsx"},
		"/project/src/utils.tsx":                  {},
		"/project/src/App.test.tsx":               {"/project/src/App.tsx"},
		"/project/src/__tests__/utils.test.tsx":   {"/project/src/utils.tsx"},
		"/project/src/components/Button.spec.tsx": {},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestDependencyGraph_ToDOT_NodesAreDeclaredOnlyOnce(t *testing.T) {
	graph := testGraph(map[string][]string{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/standalone.go": {},
		"/project/config.go":     {"/project/standalone.go"},
	})

	formatter := &dot.Formatter{}
	output, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}
