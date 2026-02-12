package watch

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/require"
)

func testJSONFileGraph(t *testing.T, adjacency map[string][]string, stats map[string]vcs.FileStats) depgraph.FileDependencyGraph {
	t.Helper()
	fileGraph, err := depgraph.NewFileDependencyGraph(depgraph.MustDependencyGraph(adjacency), stats, nil)
	require.NoError(t, err)
	return fileGraph
}

func TestJSONGraphFormatter_Format(t *testing.T) {
	graph := testJSONFileGraph(t, map[string][]string{
		"/project/main.go":  {"/project/utils.go"},
		"/project/utils.go": {},
	}, map[string]vcs.FileStats{
		"/project/main.go": {
			IsNew:     true,
			Additions: 3,
			Deletions: 1,
		},
	})

	formatter := jsonGraphFormatter{}
	output, err := formatter.Format(graph, "test-label")
	require.NoError(t, err)

	g := testhelpers.JSONGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestJSONGraphFormatter_Format_TestFileAttribute(t *testing.T) {
	graph := testJSONFileGraph(t, map[string][]string{
		"/project/main.go":      {},
		"/project/main_test.go": {"/project/main.go"},
	}, nil)

	formatter := jsonGraphFormatter{}
	output, err := formatter.Format(graph, "test-label")
	require.NoError(t, err)

	g := testhelpers.JSONGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestJSONGraphFormatter_Format_Cycles(t *testing.T) {
	graph := testJSONFileGraph(t, map[string][]string{
		"/project/a.go": {"/project/b.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {"/project/a.go"},
		"/project/d.go": {"/project/e.go"},
		"/project/e.go": {},
	}, nil)

	formatter := jsonGraphFormatter{}
	output, err := formatter.Format(graph, "cycle-label")
	require.NoError(t, err)

	g := testhelpers.JSONGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}
