package depgraph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileDependencyGraph(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/main.go":        {"/project/utils.go"},
		"/project/main_test.go":   {"/project/main.go"},
		"/project/utils.go":       {},
		"/project/helper_test.go": {},
	})

	stats := map[string]vcs.FileStats{
		"/project/main.go": {
			Additions: 3,
			Deletions: 1,
		},
	}

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, stats, nil)
	require.NoError(t, err)

	mainMeta, ok := fileGraph.Meta.Files["/project/main.go"]
	require.True(t, ok)
	require.NotNil(t, mainMeta.Stats)
	assert.Equal(t, 3, mainMeta.Stats.Additions)
	assert.Equal(t, ".go", mainMeta.Extension)
	assert.False(t, mainMeta.IsTest)

	testMeta, ok := fileGraph.Meta.Files["/project/main_test.go"]
	require.True(t, ok)
	assert.True(t, testMeta.IsTest)
	assert.Equal(t, ".go", testMeta.Extension)
	assert.Nil(t, testMeta.Stats)

	_, hasEdge := fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/main.go", To: "/project/utils.go"}]
	assert.True(t, hasEdge)
}

func TestNewFileDependencyGraph_DetectsCycles(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/a.go": {"/project/b.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {"/project/a.go"},
		"/project/d.go": {},
	})

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	require.Len(t, fileGraph.Meta.Cycles, 1)
	assert.Empty(t, fileGraph.Meta.Cycles[0].BreakSets)
	assert.Equal(t, []string{"/project/a.go", "/project/b.go", "/project/c.go"}, fileGraph.Meta.Cycles[0].Path)
	assert.Equal(t, []string{"/project/a.go", "/project/b.go", "/project/c.go"}, fileGraph.Meta.Cycles[0].Nodes)

	assert.True(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/a.go", To: "/project/b.go"}].InCycle)
	assert.True(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/b.go", To: "/project/c.go"}].InCycle)
	assert.True(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/c.go", To: "/project/a.go"}].InCycle)
	assert.False(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/d.go", To: "/project/d.go"}].InCycle)
}

func TestMarkDeletedFiles_RemovesCyclesBrokenByDeletedEdges(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/a.go": {"/project/b.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {"/project/a.go"},
	})

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)
	require.Len(t, fileGraph.Meta.Cycles, 1)

	depgraph.MarkDeletedFiles(&fileGraph, []string{"/project/a.go"})

	assert.Empty(t, fileGraph.Meta.Cycles)
	assert.False(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/b.go", To: "/project/c.go"}].InCycle)
	assert.False(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/a.go", To: "/project/b.go"}].InCycle)
	assert.False(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/c.go", To: "/project/a.go"}].InCycle)
}

func TestNewFileDependencyGraph_MarksAllEdgesInCyclicSCC(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/a.go"},
		"/project/c.go": {"/project/a.go"},
		"/project/d.go": {},
	})

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	assert.True(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/a.go", To: "/project/b.go"}].InCycle)
	assert.True(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/b.go", To: "/project/a.go"}].InCycle)
	assert.True(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/a.go", To: "/project/c.go"}].InCycle)
	assert.True(t, fileGraph.Meta.Edges[depgraph.FileEdge{From: "/project/c.go", To: "/project/a.go"}].InCycle)
}

func TestNewFileDependencyGraph_CompleteComponent(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/a.go"},
		"/project/c.go": {"/project/a.go"},
	})

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	require.Len(t, fileGraph.Meta.Cycles, 1)
	component := fileGraph.Meta.Cycles[0]
	assert.Equal(t, []string{"/project/a.go", "/project/b.go", "/project/c.go"}, component.Nodes)
	assert.Len(t, component.Edges, 4)
	assert.Equal(t, []string{"/project/a.go", "/project/b.go"}, component.Path)
}
