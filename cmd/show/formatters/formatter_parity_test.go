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
		{"renamed", "✏️ old.dart", "classDef renamedFile"},
		{"deleted", "🗑️ del.dart", "classDef deletedFile"},
	}
	for _, c := range cases {
		require.Containsf(t, dot, c.dotWant, "DOT missing %s styling", c.state)
		require.Containsf(t, mermaid, c.mmWant, "Mermaid missing %s styling", c.state)
	}
}

// TestFormatterParity_RenameAnnotation asserts a collapsed rename annotates the
// new node with where it came from, distinguishing a rename (basename change), a
// move (directory change), and both — and that the annotation renders in BOTH
// formatters, through the full Format pipeline rather than the helper alone.
func TestFormatterParity_RenameAnnotation(t *testing.T) {
	g := parityFileGraph(t, map[string][]string{
		"/p/app.go":              {"/p/cmd/show_cmd.go", "/p/internal/binding.go", "/p/parsers/bar.go"},
		"/p/cmd/show_cmd.go":     {},
		"/p/internal/binding.go": {},
		"/p/parsers/bar.go":      {},
	})
	mutate := func(key, from string) {
		md := g.Meta.Files[key]
		md.RenamedFrom = from
		g.Meta.Files[key] = md
	}
	mutate("/p/cmd/show_cmd.go", "/p/cmd/graph_cmd.go")        // rename: same directory
	mutate("/p/internal/binding.go", "/p/external/binding.go") // move: same basename
	mutate("/p/parsers/bar.go", "/p/parser/foo.go")            // move + rename: both change

	opts := RenderOptions{BasePath: "/p"}
	dot, err := (&dotFormatter{}).Format(g, opts)
	require.NoError(t, err)
	mermaid, err := mermaidFormatter{}.Format(g, opts)
	require.NoError(t, err)

	for _, want := range []string{
		"✏️ show_cmd.go",
		"(from graph_cmd.go)",
		"🚚 binding.go",
		"(from external/)",
		"🚚 ✏️ bar.go",
		"(from parser/foo.go)",
	} {
		require.Containsf(t, dot, want, "DOT missing %q", want)
		require.Containsf(t, mermaid, want, "Mermaid missing %q", want)
	}
}

func TestFormatterParity_DeletedIconPrefixesNodeName(t *testing.T) {
	g := parityFileGraph(t, map[string][]string{
		"/p/app.go":  {"/p/dead.go"},
		"/p/dead.go": {},
	})
	md := g.Meta.Files["/p/dead.go"]
	md.State = depgraph.FileStateDeleted
	g.Meta.Files["/p/dead.go"] = md

	dot, mermaid := renderBoth(t, g)

	require.Contains(t, dot, "🗑️ dead.go", "DOT deleted node should prefix the file name")
	require.Contains(t, mermaid, "🗑️ dead.go", "Mermaid deleted node should prefix the file name")
	require.NotContains(t, dot, "(deleted)", "DOT deleted state should not live in a description row")
	require.NotContains(t, mermaid, "(deleted)", "Mermaid deleted state should not live in a description row")
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

// TestFormatterParity_DeletedEdgeTakesPrecedenceOverCycle asserts that a deleted
// cycle edge renders as historical rather than implying that the cycle still
// exists.
func TestFormatterParity_DeletedEdgeTakesPrecedenceOverCycle(t *testing.T) {
	// a <-> b is a 2-cycle, so both edges are InCycle; mark a->b deleted too.
	g := parityFileGraph(t, map[string][]string{
		"/p/a.dart": {"/p/b.dart"},
		"/p/b.dart": {"/p/a.dart"},
	})
	edge := depgraph.FileEdge{From: "/p/a.dart", To: "/p/b.dart"}
	em := g.Meta.Edges[edge]
	em.State = depgraph.EdgeStateDeleted
	g.Meta.Edges[edge] = em

	dot, mermaid := renderBoth(t, g)

	// DOT: the deleted edge has one unambiguous dashed style.
	require.Contains(t, dot, `"/p/a.dart" -> "/p/b.dart" [color="#cc3333", style=dashed, fontcolor="#7a0000"];`,
		"DOT deleted cycle edge should render dashed")
	require.NotContains(t, dot, `"/p/a.dart" -> "/p/b.dart" [color="#cc3333", style=dashed, fontcolor="#7a0000", color=red`,
		"DOT deleted cycle edge should not receive conflicting cycle styling")
	require.Contains(t, dot, `"/p/b.dart" -> "/p/a.dart" [color=red, penwidth=2.0, style=solid];`,
		"DOT plain cycle edge should be solid red")

	// Mermaid: the deleted edge is dashed red; only the present cycle edge is
	// solid and heavier.
	require.Contains(t, mermaid, "linkStyle 0 stroke:#CC3333,stroke-width:2px,stroke-dasharray: 5 5",
		"Mermaid deleted cycle edge should render dashed")
	require.Contains(t, mermaid, "stroke:#d62728,stroke-width:3px\n",
		"Mermaid present cycle edge should be solid")
	require.NotContains(t, mermaid, "linkStyle 0 stroke:#d62728",
		"Mermaid deleted cycle edge should not receive cycle styling")
}

func TestFormatterParity_DeletedFileBreaksCycle(t *testing.T) {
	g := parityFileGraph(t, map[string][]string{
		"/p/a.dart": {"/p/b.dart"},
		"/p/b.dart": {"/p/c.dart"},
		"/p/c.dart": {"/p/a.dart"},
	})
	depgraph.MarkDeletedFiles(&g, []string{"/p/a.dart"})

	dot, mermaid := renderBoth(t, g)

	require.Contains(t, dot, `style=dashed`, "DOT should retain deleted-edge context")
	require.NotContains(t, dot, "Cyclic paths", "DOT should not report a historical cycle as current")
	require.NotContains(t, dot, "color=red", "DOT should not highlight surviving edges or nodes as cyclic")

	require.Contains(t, mermaid, "stroke-dasharray: 5 5", "Mermaid should retain deleted-edge context")
	require.NotContains(t, mermaid, "%% C1:", "Mermaid should not report a historical cycle as current")
	require.NotContains(t, mermaid, "stroke:#d62728", "Mermaid should not highlight surviving edges or nodes as cyclic")
}
