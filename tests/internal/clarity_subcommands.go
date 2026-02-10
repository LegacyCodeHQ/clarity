package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	graphcmd "github.com/LegacyCodeHQ/clarity/cmd/graph"
	"github.com/stretchr/testify/require"
)

func GraphSubcommand(t *testing.T, commit string) string {
	t.Helper()

	repoRoot := RepoRoot(t)
	cmd := graphcmd.NewCommand()
	cmd.SetArgs([]string{"-c", commit, "-f", "dot", "-r", repoRoot})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	require.NoError(t, err, "stderr: %s", strings.TrimSpace(stderr.String()))

	return strings.TrimRight(stdout.String(), "\n")
}

func GraphSubcommandInputWithRepo(t *testing.T, repoPath string, inputs ...string) string {
	t.Helper()

	cmd := graphcmd.NewCommand()
	args := []string{"-f", "dot", "-r", repoPath}
	allowOutside := false
	for _, input := range inputs {
		checkPath := input
		if !filepath.IsAbs(input) {
			checkPath = filepath.Join(repoPath, input)
		}
		rel, err := filepath.Rel(repoPath, checkPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			allowOutside = true
			break
		}
	}
	if allowOutside {
		args = append(args, "--allow-outside-repo")
	}
	if len(inputs) > 0 {
		args = append(args, "-i", strings.Join(inputs, ","))
	}
	cmd.SetArgs(args)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	require.NoError(t, err, "stderr: %s", strings.TrimSpace(stderr.String()))

	return strings.TrimRight(stdout.String(), "\n")
}

func RepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	repoRoot := wd
	for i := 0; i < 10; i++ {
		_, err = os.Stat(filepath.Join(repoRoot, "go.mod"))
		if err == nil {
			return repoRoot
		}

		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			break
		}
		repoRoot = parent
	}

	require.NoError(t, err, "expected repo root with go.mod, got %s", repoRoot)
	return repoRoot
}
