package graph_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/sanity/internal/testhelpers"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

const (
	commitAddOnboard = "d2c296558f5a5261629fd43eff6b572c636bb3e8"
	commitAddInit    = "ba7ca46722c59ede139f329a3f1ef65c47b66832"
)

func TestGraphCommit_AddOnboardCommand(t *testing.T) {
	output := runGraphCommand(t, commitAddOnboard)

	g := graphGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func TestGraphCommit_AddInitAndPrimeCommands(t *testing.T) {
	output := runGraphCommand(t, commitAddInit)

	g := graphGoldie(t)
	g.Assert(t, t.Name(), []byte(output))
}

func graphGoldie(t *testing.T) *goldie.Goldie {
	return testhelpers.GoldieWithSuffix(t, ".dot")
}

func runGraphCommand(t *testing.T, commit string) string {
	t.Helper()

	repoRoot := repoRoot(t)
	cmd := exec.Command("go", "run", "./main.go", "graph", "-c", commit, "-f", "dot")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.NoError(t, err, "stderr: %s", strings.TrimSpace(stderr.String()))

	return strings.TrimRight(stdout.String(), "\n")
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	_, err = os.Stat(filepath.Join(repoRoot, "go.mod"))
	require.NoError(t, err, "expected repo root with go.mod, got %s", repoRoot)

	return repoRoot
}
