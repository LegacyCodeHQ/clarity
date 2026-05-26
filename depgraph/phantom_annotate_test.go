package depgraph_test

import (
	"errors"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rustProd = `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}
`
	// Lines 1..3 prod, 5..10 test region.
	rustWithTests = `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn it_adds() { assert_eq!(add(1, 2), 3); }
}
`
)

func TestAnnotateRustPhantomsShow(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/src/lib.rs":  {},
		"/project/src/util.rs": {},
		"/project/main.go":     {},
	})

	contentReader := func(path string) ([]byte, error) {
		switch path {
		case "/project/src/lib.rs":
			return []byte(rustWithTests), nil
		case "/project/src/util.rs":
			return []byte(rustProd), nil
		case "/project/main.go":
			return []byte("package main\n"), nil
		}
		return nil, errors.New("unknown path")
	}

	fg, err := depgraph.NewFileDependencyGraph(graph, nil, contentReader)
	require.NoError(t, err)

	fg.AnnotateRustPhantomsShow(contentReader)

	libMeta, ok := fg.Meta.Files["/project/src/lib.rs"]
	require.True(t, ok)
	require.NotNil(t, libMeta.Phantom)
	assert.Equal(t, "rust-test", libMeta.Phantom.Kind)
	assert.Nil(t, libMeta.Phantom.Stats, "show mode populates no stats")

	utilMeta, ok := fg.Meta.Files["/project/src/util.rs"]
	require.True(t, ok)
	assert.Nil(t, utilMeta.Phantom, ".rs file without test region gets no phantom")

	goMeta, ok := fg.Meta.Files["/project/main.go"]
	require.True(t, ok)
	assert.Nil(t, goMeta.Phantom, "non-rust files get no phantom")
}

func TestAnnotateRustPhantomsShow_NilContentReader(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/src/lib.rs": {},
	})
	fg, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	fg.AnnotateRustPhantomsShow(nil)

	libMeta := fg.Meta.Files["/project/src/lib.rs"]
	assert.Nil(t, libMeta.Phantom)
}

func TestAnnotateRustPhantomsWatch_TestOnly(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/src/lib.rs": {},
	})
	fg, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	diffs := map[string]vcs.FileDiff{
		"/project/src/lib.rs": {Additions: []int{8}},
	}
	reader := func(path string) ([]byte, error) {
		return []byte(rustWithTests), nil
	}

	fg.AnnotateRustPhantomsWatch(diffs, reader, reader)

	meta := fg.Meta.Files["/project/src/lib.rs"]
	require.NotNil(t, meta.Stats, "prod stats are reset even when prod has no changes")
	assert.Equal(t, 0, meta.Stats.Additions)
	assert.Equal(t, 0, meta.Stats.Deletions)

	require.NotNil(t, meta.Phantom)
	require.NotNil(t, meta.Phantom.Stats)
	assert.Equal(t, 1, meta.Phantom.Stats.Additions)
	assert.False(t, meta.Phantom.ProdChanged, "test-only change leaves prod marked as context")
}

func TestAnnotateRustPhantomsWatch_ProdOnly(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/src/lib.rs": {},
	})
	fg, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	diffs := map[string]vcs.FileDiff{
		"/project/src/lib.rs": {Additions: []int{2}},
	}
	reader := func(path string) ([]byte, error) {
		return []byte(rustWithTests), nil
	}

	fg.AnnotateRustPhantomsWatch(diffs, reader, reader)

	meta := fg.Meta.Files["/project/src/lib.rs"]
	require.NotNil(t, meta.Stats)
	assert.Equal(t, 1, meta.Stats.Additions)
	assert.Nil(t, meta.Phantom, "prod-only change emits no phantom — the warning case")
}

func TestAnnotateRustPhantomsWatch_BothChanged(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/src/lib.rs": {},
	})
	fg, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	diffs := map[string]vcs.FileDiff{
		"/project/src/lib.rs": {Additions: []int{2, 8}},
	}
	reader := func(path string) ([]byte, error) {
		return []byte(rustWithTests), nil
	}

	fg.AnnotateRustPhantomsWatch(diffs, reader, reader)

	meta := fg.Meta.Files["/project/src/lib.rs"]
	require.NotNil(t, meta.Stats)
	assert.Equal(t, 1, meta.Stats.Additions)
	require.NotNil(t, meta.Phantom)
	require.NotNil(t, meta.Phantom.Stats)
	assert.Equal(t, 1, meta.Phantom.Stats.Additions)
	assert.True(t, meta.Phantom.ProdChanged)
}

func TestAnnotateRustPhantomsWatch_NewFile(t *testing.T) {
	graph := depgraph.MustDependencyGraph(map[string][]string{
		"/project/src/lib.rs": {},
	})
	fg, err := depgraph.NewFileDependencyGraph(graph, nil, nil)
	require.NoError(t, err)

	diffs := map[string]vcs.FileDiff{
		"/project/src/lib.rs": {
			Additions: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			IsNew:     true,
		},
	}
	emptyOld := func(path string) ([]byte, error) { return nil, errors.New("does not exist") }
	newReader := func(path string) ([]byte, error) { return []byte(rustWithTests), nil }

	fg.AnnotateRustPhantomsWatch(diffs, emptyOld, newReader)

	meta := fg.Meta.Files["/project/src/lib.rs"]
	require.NotNil(t, meta.Stats)
	assert.True(t, meta.Stats.IsNew)
	assert.Equal(t, 4, meta.Stats.Additions, "prod side: lines 1..4")
	require.NotNil(t, meta.Phantom)
	require.NotNil(t, meta.Phantom.Stats)
	assert.True(t, meta.Phantom.Stats.IsNew)
	assert.Equal(t, 6, meta.Phantom.Stats.Additions, "test side: lines 5..10")
}
