package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListIgnoredDirectoriesUsesGitStandardIgnoreSources(t *testing.T) {
	repo := t.TempDir()
	setupGitRepo(t, repo)

	require.NoError(t, os.WriteFile(
		filepath.Join(repo, ".gitignore"),
		[]byte(".build/\n*.log\n"),
		0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, ".git", "info", "exclude"),
		[]byte(".cache/\n"),
		0o644))

	modules := filepath.Join(repo, "Modules")
	require.NoError(t, os.MkdirAll(modules, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(modules, ".gitignore"),
		[]byte("Generated/\n"),
		0o644))

	globalExclude := filepath.Join(repo, "test-global-excludes")
	require.NoError(t, os.WriteFile(globalExclude, []byte("Local/\n"), 0o644))
	gitConfig(t, repo, "core.excludesFile", globalExclude)

	ignored := []string{
		filepath.Join(repo, ".build"),
		filepath.Join(repo, ".cache"),
		filepath.Join(repo, "Local"),
		filepath.Join(repo, "Modules", "Generated"),
	}
	for _, path := range ignored {
		require.NoError(t, os.MkdirAll(filepath.Join(path, "nested"), 0o755))
	}
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "debug.log"),
		[]byte("ignored file"),
		0o644))

	got, err := ListIgnoredDirectories(modules)
	require.NoError(t, err)

	resolvedIgnored := make([]string, 0, len(ignored))
	for _, path := range ignored {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		require.NoError(t, resolveErr)
		resolvedIgnored = append(resolvedIgnored, resolved)
	}
	assert.ElementsMatch(t, resolvedIgnored, got)
	assert.NotContains(t, got, filepath.Join(repo, "debug.log"))
}
