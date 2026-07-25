package depgraph_test

import (
	"fmt"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCycleBreakSets_ExactAlternatives(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/a.go"},
		"/project/c.go": {"/project/a.go"},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)
	require.Len(t, fileGraph.Meta.Cycles, 1)

	depgraph.AnalyzeCycleBreakSets(fileGraph.Meta.Cycles)

	component := fileGraph.Meta.Cycles[0]
	assert.True(t, component.BreakSetExact)
	require.Len(t, component.BreakSets, 4)
	for _, breakSet := range component.BreakSets {
		assert.Len(t, breakSet.Edges, 2)
	}
}

func TestAnalyzeCycleBreakSets_SelfLoop(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/a.go": {"/project/a.go"},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)
	require.Len(t, fileGraph.Meta.Cycles, 1)

	depgraph.AnalyzeCycleBreakSets(fileGraph.Meta.Cycles)

	component := fileGraph.Meta.Cycles[0]
	require.Len(t, component.BreakSets, 1)
	assert.Equal(t, []depgraph.FileEdge{{
		From: "/project/a.go",
		To:   "/project/a.go",
	}}, component.BreakSets[0].Edges)
}

func TestAnalyzeCycleBreakSets_HeuristicFindsHighLeverageEdge(t *testing.T) {
	dependencies := map[string][]string{
		"/project/database.go": {"/project/migrations.go"},
	}
	for i := 0; i < 9; i++ {
		migration := fmt.Sprintf("/project/v%02d.go", i)
		dependencies["/project/migrations.go"] = append(
			dependencies["/project/migrations.go"],
			migration)
		dependencies[migration] = []string{"/project/database.go"}
	}
	graph := depgraph.MustDependencyGraph(dependencies)
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)
	require.Len(t, fileGraph.Meta.Cycles, 1)

	depgraph.AnalyzeCycleBreakSets(fileGraph.Meta.Cycles)

	component := fileGraph.Meta.Cycles[0]
	assert.False(t, component.BreakSetExact)
	require.Len(t, component.BreakSets, 1)
	assert.Equal(t, []depgraph.FileEdge{{
		From: "/project/database.go",
		To:   "/project/migrations.go",
	}}, component.BreakSets[0].Edges)
}
