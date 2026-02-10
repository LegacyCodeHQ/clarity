package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/require"
)

// setupGitRepo initializes a git repository in a temporary directory
func setupGitRepo(t *testing.T, dir string) {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	require.NoError(t, cmd.Run(), "failed to initialize git repository")

	// Configure git user to avoid errors
	gitConfig(t, dir, "user.name", "Test User")
	gitConfig(t, dir, "user.email", "test@example.com")
}

// gitConfig sets a git config value
func gitConfig(t *testing.T, repoDir, key, value string) {
	cmd := exec.Command("git", "config", key, value)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run(), "failed to set git config %s", key)
}

// createFile creates a file with content
func createFile(t *testing.T, dir, name, content string) string {
	filePath := filepath.Join(dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "failed to create file %s", name)
	return filePath
}

// createDartFile creates a dart file with sample content
func createDartFile(t *testing.T, dir, name string) string {
	return createFile(t, dir, name, "import 'dart:io';\n\nclass Test {}")
}

// gitAdd adds a file to git staging area
func gitAdd(t *testing.T, repoDir, file string) {
	cmd := exec.Command("git", "add", file)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run(), "failed to git add %s", file)
}

// gitCommit commits files with a message
func gitCommit(t *testing.T, repoDir, message string) {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run(), "failed to git commit")
}

// gitCommitAndGetSHA commits files and returns the commit SHA
func gitCommitAndGetSHA(t *testing.T, repoDir, message string) string {
	// Commit the files
	gitCommit(t, repoDir, message)

	// Get the commit SHA
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run(), "failed to get commit SHA")

	return strings.TrimSpace(stdout.String())
}

// modifyFile overwrites a file with modified content
func modifyFile(t *testing.T, filePath string) {
	err := os.WriteFile(filePath, []byte("modified content\n"), 0644)
	require.NoError(t, err, "failed to modify file %s", filePath)
}

// normalizeFilePaths normalizes file paths for golden file comparison
// by replacing the temp directory with $REPO placeholder
func normalizeFilePaths(tmpDir string, paths []string) string {
	if len(paths) == 0 {
		return "(empty)"
	}
	resolvedTmpDir, _ := filepath.EvalSymlinks(tmpDir)
	var normalized []string
	for _, p := range paths {
		relPath := strings.TrimPrefix(p, resolvedTmpDir+"/")
		normalized = append(normalized, "$REPO/"+relPath)
	}
	sort.Strings(normalized)
	return strings.Join(normalized, "\n")
}

// normalizeFileStats normalizes file stats for golden file comparison
func normalizeFileStats(tmpDir string, stats map[string]vcs.FileStats) string {
	if len(stats) == 0 {
		return "(empty)"
	}
	resolvedTmpDir, _ := filepath.EvalSymlinks(tmpDir)
	var keys []string
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		stat := stats[k]
		relPath := strings.TrimPrefix(k, resolvedTmpDir+"/")
		lines = append(lines, fmt.Sprintf("$REPO/%s: +%d -%d new=%t", relPath, stat.Additions, stat.Deletions, stat.IsNew))
	}
	return strings.Join(lines, "\n")
}
