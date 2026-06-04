package depgraph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollapseModules_MergesMembersIntoSingleNode(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/main.go":  {"/project/mod.go", "/project/util.go"},
		"/project/mod.go":   {"/project/main.go"}, // internal cycle with main.go
		"/project/util.go":  {},
		"/project/other.go": {"/project/main.go"},
	})

	collapsed, moduleMembers, err := depgraph.CollapseModules(graph, []depgraph.Module{
		{Name: "X", Files: []string{"/project/main.go", "/project/mod.go"}},
	})
	require.NoError(t, err)

	assert.Equal(t, map[string][]string{
		"X": {"/project/main.go", "/project/mod.go"},
	}, moduleMembers)

	adjacency, err := depgraph.AdjacencyList(collapsed)
	require.NoError(t, err)

	// Members are gone; the module node X takes their place.
	assert.NotContains(t, adjacency, "/project/main.go")
	assert.NotContains(t, adjacency, "/project/mod.go")
	require.Contains(t, adjacency, "X")

	// Outgoing edges from members redirect to X; internal edges are dropped
	// (no X -> X self-loop), and duplicates collapse to one.
	assert.ElementsMatch(t, []string{"/project/util.go"}, adjacency["X"])

	// Incoming edges to members redirect to X.
	assert.ElementsMatch(t, []string{"X"}, adjacency["/project/other.go"])

	assert.Empty(t, adjacency["/project/util.go"])
}

func TestCollapseModules_NoMembersInGraphIsNoop(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/main.go": {},
	})

	collapsed, moduleMembers, err := depgraph.CollapseModules(graph, []depgraph.Module{
		{Name: "X", Files: []string{"/project/absent.go"}},
	})
	require.NoError(t, err)

	assert.Empty(t, moduleMembers)

	adjacency, err := depgraph.AdjacencyList(collapsed)
	require.NoError(t, err)

	require.Contains(t, adjacency, "/project/main.go")
	assert.NotContains(t, adjacency, "X")
}

func TestAnnotateModule_AggregatesMemberChurn(t *testing.T) {
	fileGraph, err := depgraph.NewFileDependencyGraph(depgraph.MustDependencyGraph(map[string][]string{
		"X":                {"/project/util.go"},
		"/project/util.go": {},
	}), nil, nil)
	require.NoError(t, err)

	fileGraph.AnnotateModule("X", []string{"/project/a.go", "/project/b.go", "/project/c.go"}, map[string]vcs.FileStats{
		"/project/a.go": {Additions: 30, Deletions: 5},
		"/project/b.go": {Additions: 20, Deletions: 5},
		// c.go has no stats entry (unchanged file); it still counts toward the total.
	})

	md := fileGraph.Meta.Files["X"]
	assert.True(t, md.IsModule)
	assert.Equal(t, 3, md.ModuleFileCount)
	require.NotNil(t, md.Stats)
	assert.Equal(t, 50, md.Stats.Additions)
	assert.Equal(t, 10, md.Stats.Deletions)
}
