package depgraph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
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

	collapsed, moduleNodes, err := depgraph.CollapseModules(graph, []depgraph.Module{
		{Name: "X", Files: []string{"/project/main.go", "/project/mod.go"}},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"X"}, moduleNodes)

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

	collapsed, moduleNodes, err := depgraph.CollapseModules(graph, []depgraph.Module{
		{Name: "X", Files: []string{"/project/absent.go"}},
	})
	require.NoError(t, err)

	assert.Empty(t, moduleNodes)

	adjacency, err := depgraph.AdjacencyList(collapsed)
	require.NoError(t, err)

	require.Contains(t, adjacency, "/project/main.go")
	assert.NotContains(t, adjacency, "X")
}
