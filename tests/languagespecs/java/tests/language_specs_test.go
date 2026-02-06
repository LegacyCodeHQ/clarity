package graph_test

import (
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/sanity/internal/testhelpers"
	"github.com/LegacyCodeHQ/sanity/tests/internal"
)

func TestGraphLanguageSpec_JavaBasic(t *testing.T) {
	repoRoot := internal.RepoRoot(t)
	inputPath := filepath.Join(repoRoot, "tests/languagespecs/java/examples/basic")

	output := internal.GraphSubcommandInputWithRepo(t, t.TempDir(), inputPath)

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}
