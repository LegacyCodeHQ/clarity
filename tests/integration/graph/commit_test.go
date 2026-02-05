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

func TestGraphCommit_VerifyCrossTypeRelationships(t *testing.T) {
	output := internal.GraphSubcommand(t, "db68272e70a2733c65f58c5a36e438acd9134db8")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyMultipleTestFiles(t *testing.T) {
	output := internal.GraphSubcommand(t, "296c9a6b26bab88aaaff57e4cfca40efe10754c1")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifySingleFile(t *testing.T) {
	output := internal.GraphSubcommand(t, "67d0be00f743d4afdd26387d62d4ba84085e393b")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyDifferentExtensions(t *testing.T) {
	output := internal.GraphSubcommand(t, "0deca10204f93f78ad5d25fabdf0046bdea127dd")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyComplexTestProductionRelationships(t *testing.T) {
	output := internal.GraphSubcommand(t, "051a1b8c4ef7704259383edfe9c7595335c4f358")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyDocsAndGeneratedArtifacts(t *testing.T) {
	output := internal.GraphSubcommand(t, "e1a0ef91b6e431ed9ad6c8c3c24cb00d414f2272")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyComplexRelationships(t *testing.T) {
	output := internal.GraphSubcommand(t, "ee16b4ee8563a37b578f52a2e9569fe6dfd1e2df")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyMajorityColorWhite(t *testing.T) {
	output := internal.GraphSubcommand(t, "5aa97693902e9bacaad15158068105c0ca18107a")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_VerifyFormatterAndPathRefactor(t *testing.T) {
	output := internal.GraphSubcommand(t, "b7bb3fcfaad2260e2aa14d3dc9554a83f02084fe")

	g := testhelpers.DotGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}
