package graph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/sanity/internal/testhelpers"
	"github.com/LegacyCodeHQ/sanity/tests/integration/internal"
)

func TestGraphCommit_VerifyGoEmbedRelationships(t *testing.T) {
	output := internal.GraphSubcommand(t, "ba7ca46722c59ede139f329a3f1ef65c47b66832")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyNewFilesAndMixedTypes(t *testing.T) {
	output := internal.GraphSubcommand(t, "daa4fedbaeefcd21568819b2224f2e9febcf02e8")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyAllNewFilesAndTestHighlight(t *testing.T) {
	output := internal.GraphSubcommand(t, "29daace91e32509eae1ce314623bcdaa1411475d")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}
