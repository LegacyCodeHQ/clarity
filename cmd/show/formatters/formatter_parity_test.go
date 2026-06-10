package formatters

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/stretchr/testify/require"
)

// parityFileGraph builds a file graph from a raw adjacency for parity tests.
func parityFileGraph(t *testing.T, adjacency map[string][]string) depgraph.FileDependencyGraph {
	t.Helper()
	fg, err := depgraph.NewFileDependencyGraph(depgraph.MustDependencyGraph(adjacency), nil, nil)
	require.NoError(t, err)
	return fg
}

func renderBoth(t *testing.T, g depgraph.FileDependencyGraph) (dotOut, mermaidOut string) {
	t.Helper()
	d, err := (&dotFormatter{}).Format(g, RenderOptions{})
	require.NoError(t, err)
	m, err := mermaidFormatter{}.Format(g, RenderOptions{})
	require.NoError(t, err)
	return d, m
}

// TestFormatterParity_NodeStates asserts every node state renders a distinct
// marker in BOTH formatters. If a state is handled in one formatter but dropped
// in the other, the corresponding assertion here fails — the safety net for the
// presentation layer the Renderer contract does not compile-guard.
func TestFormatterParity_NodeStates(t *testing.T) {
	g := parityFileGraph(t, map[string][]string{
		"/p/app.dart":       {"/p/util_test.dart", "/p/mod", "/p/pruned.dart", "/p/old.dart", "/p/del.dart"},
		"/p/util_test.dart": {},
		"/p/mod":            {},
		"/p/pruned.dart":    {},
		"/p/old.dart":       {},
		"/p/del.dart":       {},
	})

	mutate := func(key string, fn func(*depgraph.FileMetadata)) {
		md := g.Meta.Files[key]
		fn(&md)
		g.Meta.Files[key] = md
	}
	mutate("/p/util_test.dart", func(md *depgraph.FileMetadata) { md.IsTest = true })
	mutate("/p/mod", func(md *depgraph.FileMetadata) { md.IsModule = true; md.ModuleFileCount = 2 })
	mutate("/p/pruned.dart", func(md *depgraph.FileMetadata) { md.IsPruned = true })
	mutate("/p/old.dart", func(md *depgraph.FileMetadata) { md.State = depgraph.FileStateRenamed })
	mutate("/p/del.dart", func(md *depgraph.FileMetadata) { md.State = depgraph.FileStateDeleted })

	dot, mermaid := renderBoth(t, g)

	cases := []struct {
		state   string
		dotWant string
		mmWant  string
	}{
		{"test", "lightgreen", "classDef testFile"},
		{"module", "shape=component", "classDef moduleNode"},
		{"pruned", "color=gray", "classDef prunedFile"},
		{"renamed", "(renamed)", "classDef renamedFile"},
		{"deleted", "(deleted)", "classDef deletedFile"},
	}
	for _, c := range cases {
		require.Containsf(t, dot, c.dotWant, "DOT missing %s styling", c.state)
		require.Containsf(t, mermaid, c.mmWant, "Mermaid missing %s styling", c.state)
	}
}

// TestFormatterParity_PhantomNodes asserts the phantom test-sibling capability
// renders in both formatters, so it cannot be dropped from one unnoticed.
func TestFormatterParity_PhantomNodes(t *testing.T) {
	g := parityFileGraph(t, map[string][]string{
		"/p/lib.rs": {},
		"/p/app.rs": {"/p/lib.rs"},
	})

	md := g.Meta.Files["/p/lib.rs"]
	md.Phantom = &depgraph.PhantomMetadata{Kind: "rust-test"}
	g.Meta.Files["/p/lib.rs"] = md

	dot, mermaid := renderBoth(t, g)

	require.Contains(t, dot, "darkgreen", "DOT missing phantom styling")
	require.Contains(t, mermaid, "phantomTest", "Mermaid missing phantom styling")
}

// TestFormatterParity_MajorityTypeKeying pins the unified extension keying both
// formatters now share: extensionless files key by base name (not ""), so a
// distinct `Makefile`/`Dockerfile` does not collapse into one bucket and the
// majority is computed identically for DOT and Mermaid.
func TestFormatterParity_MajorityTypeKeying(t *testing.T) {
	g := parityFileGraph(t, map[string][]string{
		"/p/Makefile":   {},
		"/p/Dockerfile": {},
		"/p/x.go":       {"/p/y.go"},
		"/p/y.go":       {},
	})

	scene, err := BuildScene(g, RenderOptions{})
	require.NoError(t, err)

	require.Equal(t, ".go", scene.MajorityType, "majority should be .go (2 files), not the extensionless bucket")
	require.Equal(t, "Makefile", scene.Nodes["/p/Makefile"].Type, "extensionless files key by base name")
	require.True(t, scene.HasMultipleTypes)
}

// TestFormatterParity_EdgeStates asserts every edge state renders distinctly in
// both formatters.
func TestFormatterParity_EdgeStates(t *testing.T) {
	g := parityFileGraph(t, map[string][]string{
		"/p/a.dart": {"/p/b.dart", "/p/c.dart"},
		"/p/b.dart": {},
		"/p/c.dart": {},
	})

	setEdge := func(from, to string, state depgraph.EdgeState) {
		edge := depgraph.FileEdge{From: from, To: to}
		em := g.Meta.Edges[edge]
		em.State = state
		g.Meta.Edges[edge] = em
	}
	setEdge("/p/a.dart", "/p/b.dart", depgraph.EdgeStateDeleted)
	setEdge("/p/a.dart", "/p/c.dart", depgraph.EdgeStateRenamed)

	dot, mermaid := renderBoth(t, g)

	require.Contains(t, dot, `color="#cc3333"`, "DOT missing deleted-edge styling")
	require.Contains(t, dot, `color="#cc8800"`, "DOT missing renamed-edge styling")
	require.Contains(t, mermaid, "stroke:#CC3333", "Mermaid missing deleted-edge styling")
	require.Contains(t, mermaid, "stroke:#CC8800", "Mermaid missing renamed-edge styling")
}
