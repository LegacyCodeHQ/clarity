package explain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/explain"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachEvidence_SwiftTypeReference(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "Sources", "App")
	require.NoError(t, os.MkdirAll(module, 0o755))
	a := filepath.Join(module, "A.swift")
	b := filepath.Join(module, "B.swift")
	require.NoError(t, os.WriteFile(a, []byte(`
struct A {
    let b: B
}
`), 0o644))
	require.NoError(t, os.WriteFile(b, []byte(`
struct B {
    let a: A
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph, err := depgraph.BuildDependencyGraph([]string{a, b}, reader)
	require.NoError(t, err)
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	aToB := fileGraph.Meta.Edges[depgraph.FileEdge{From: a, To: b}].Evidence
	require.NotEmpty(t, aToB)
	assert.Equal(t, "B", aToB[0].Symbol)
	assert.Equal(t, "swift-symbol", aToB[0].Kind)
	assert.Equal(t, depgraph.RelationshipTypeReference, aToB[0].Relationship)
	assert.Equal(t, 3, aToB[0].ReferenceLine)
	assert.Equal(t, 2, aToB[0].DeclarationLine)
	assert.Equal(t, depgraph.EvidenceConfidenceHigh, aToB[0].Confidence)
}

func TestAttachEvidence_MarkdownLink(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	guide := filepath.Join(dir, "guide.md")
	require.NoError(t, os.WriteFile(readme, []byte("[guide](guide.md)\n"), 0o644))
	require.NoError(t, os.WriteFile(guide, []byte("[readme](README.md)\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	graph, err := depgraph.BuildDependencyGraph([]string{readme, guide}, reader)
	require.NoError(t, err)
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: readme, To: guide}].Evidence
	require.NotEmpty(t, evidence)
	assert.Equal(t, "guide.md", evidence[0].Symbol)
	assert.Equal(t, "markdown-link", evidence[0].Kind)
	assert.Equal(t, depgraph.RelationshipNavigation, evidence[0].Relationship)
	assert.Equal(t, 1, evidence[0].ReferenceLine)
	assert.Equal(t, depgraph.EvidenceConfidenceHigh, evidence[0].Confidence)
}
