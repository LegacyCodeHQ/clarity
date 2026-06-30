package formatters

import (
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildRustGraph constructs a single-file Rust dependency graph and applies
// the supplied phantom + stats metadata before returning it.
func buildRustGraph(t *testing.T, phantom *depgraph.PhantomMetadata, prodStats *vcs.FileStats) depgraph.FileDependencyGraph {
	t.Helper()
	fg, err := depgraph.NewFileDependencyGraph(testGraph(map[string][]string{
		"/project/src/lib.rs": {},
	}), nil, nil)
	require.NoError(t, err)
	meta := fg.Meta.Files["/project/src/lib.rs"]
	meta.Phantom = phantom
	meta.Stats = prodStats
	fg.Meta.Files["/project/src/lib.rs"] = meta
	return fg
}

func TestDOT_PhantomNode_ShowMode(t *testing.T) {
	fg := buildRustGraph(t, &depgraph.PhantomMetadata{Kind: "rust-test"}, nil)

	formatter := dotFormatter{}
	out, err := formatter.Format(fg, RenderOptions{})
	require.NoError(t, err)

	assert.Contains(t, out, `"/project/src/lib.rs::tests"`, "phantom node id missing")
	assert.Contains(t, out, `fillcolor=lightgreen`, "phantom node should be green")
	assert.Contains(t, out, `"/project/src/lib.rs::tests" -> "/project/src/lib.rs"`, "phantom→prod edge missing")
	assert.Contains(t, out, `style=dotted, color=darkgreen`, "phantom edge should be dotted green")

	// Prod node still solid in show mode (no Stats on the phantom).
	prodLine := lineWithSourceKey(out, `"/project/src/lib.rs"`, "[")
	require.NotEmpty(t, prodLine)
	assert.Contains(t, prodLine, "style=filled")
	assert.NotContains(t, prodLine, "dashed", "prod border should be solid in show mode")
}

func TestDOT_PhantomNode_Watch_BothChanged(t *testing.T) {
	phantom := &depgraph.PhantomMetadata{
		Kind:        "rust-test",
		Stats:       &vcs.FileStats{Additions: 1},
		ProdChanged: true,
	}
	fg := buildRustGraph(t, phantom, &vcs.FileStats{Additions: 2, Deletions: 2})

	formatter := dotFormatter{}
	out, err := formatter.Format(fg, RenderOptions{})
	require.NoError(t, err)

	// Phantom carries its own +1 count.
	assert.Contains(t, out, `"/project/src/lib.rs::tests" [label="lib.rs\n+1"`)

	// Prod node solid (both changed).
	prodLine := lineWithSourceKey(out, `"/project/src/lib.rs"`, "[")
	require.NotEmpty(t, prodLine)
	assert.Contains(t, prodLine, "style=filled,")
	assert.NotContains(t, prodLine, "dashed")
	assert.Contains(t, prodLine, `label="lib.rs\n+2 -2"`, "prod node should show prod-side counts")
}

func TestDOT_PhantomNode_Watch_TestOnly(t *testing.T) {
	phantom := &depgraph.PhantomMetadata{
		Kind:        "rust-test",
		Stats:       &vcs.FileStats{Additions: 2},
		ProdChanged: false,
	}
	fg := buildRustGraph(t, phantom, &vcs.FileStats{})

	formatter := dotFormatter{}
	out, err := formatter.Format(fg, RenderOptions{})
	require.NoError(t, err)

	assert.Contains(t, out, `"/project/src/lib.rs::tests" [label="lib.rs\n+2"`)

	// The phantom node's count already communicates that only the in-file test region changed;
	// keep the actual file node visually solid.
	prodLine := lineWithSourceKey(out, `"/project/src/lib.rs"`, "[")
	require.NotEmpty(t, prodLine)
	assert.Contains(t, prodLine, "style=filled")
	assert.NotContains(t, prodLine, "dashed")
}

func TestDOT_PhantomNode_AbsentWhenProdOnlyChange(t *testing.T) {
	fg := buildRustGraph(t, nil, &vcs.FileStats{Additions: 3})

	formatter := dotFormatter{}
	out, err := formatter.Format(fg, RenderOptions{})
	require.NoError(t, err)

	assert.NotContains(t, out, `::tests`, "no phantom should be emitted when prod-only changed")
	prodLine := lineWithSourceKey(out, `"/project/src/lib.rs"`, "[")
	require.NotEmpty(t, prodLine)
	assert.NotContains(t, prodLine, "dashed")
}

// lineWithSourceKey returns the first line containing both `key` and a
// following substring (anchored after the key) to disambiguate prod vs phantom
// rows that share a key prefix.
func lineWithSourceKey(out, key, after string) string {
	for _, line := range strings.Split(out, "\n") {
		idx := strings.Index(line, key)
		if idx == -1 {
			continue
		}
		if !strings.Contains(line[idx+len(key):], after) {
			continue
		}
		// Skip the phantom line (which starts with `"...::tests"`).
		if strings.Contains(line, "::tests") {
			continue
		}
		return line
	}
	return ""
}
