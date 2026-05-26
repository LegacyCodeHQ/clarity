package formatters

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildMermaidRustGraph(t *testing.T, phantom *depgraph.PhantomMetadata, prodStats *vcs.FileStats) depgraph.FileDependencyGraph {
	t.Helper()
	fg, err := depgraph.NewFileDependencyGraph(testGraphMermaid(map[string][]string{
		"/project/src/lib.rs": {},
	}), nil, nil)
	require.NoError(t, err)
	meta := fg.Meta.Files["/project/src/lib.rs"]
	meta.Phantom = phantom
	meta.Stats = prodStats
	fg.Meta.Files["/project/src/lib.rs"] = meta
	return fg
}

func TestMermaid_PhantomNode_ShowMode(t *testing.T) {
	fg := buildMermaidRustGraph(t, &depgraph.PhantomMetadata{Kind: "rust-test"}, nil)

	formatter := mermaidFormatter{}
	out, err := formatter.Format(fg, RenderOptions{})
	require.NoError(t, err)

	assert.Contains(t, out, "n0p[", "phantom node definition missing")
	assert.Contains(t, out, "n0p --- n0", "phantom→prod edge missing")
	assert.Contains(t, out, "classDef phantomTest")
	assert.Contains(t, out, "class n0p phantomTest")
	assert.NotContains(t, out, "phantomProdContext", "prod should not be context-styled in show mode")
}

func TestMermaid_PhantomNode_Watch_TestOnly(t *testing.T) {
	phantom := &depgraph.PhantomMetadata{
		Kind:        "rust-test",
		Stats:       &vcs.FileStats{Additions: 2},
		ProdChanged: false,
	}
	fg := buildMermaidRustGraph(t, phantom, &vcs.FileStats{})

	formatter := mermaidFormatter{}
	out, err := formatter.Format(fg, RenderOptions{})
	require.NoError(t, err)

	assert.Contains(t, out, "+2", "phantom node should carry test-side count")
	assert.Contains(t, out, "classDef phantomProdContext stroke-dasharray: 5 5")
	assert.Contains(t, out, "class n0 phantomProdContext")
}

func TestMermaid_PhantomNode_AbsentWhenProdOnly(t *testing.T) {
	fg := buildMermaidRustGraph(t, nil, &vcs.FileStats{Additions: 3})

	formatter := mermaidFormatter{}
	out, err := formatter.Format(fg, RenderOptions{})
	require.NoError(t, err)

	assert.NotContains(t, out, "n0p")
	assert.NotContains(t, out, "phantomTest")
	assert.NotContains(t, out, "phantomProdContext")
}
