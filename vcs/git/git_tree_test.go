package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommitTreeFiles_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit a file
	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Initial commit")

	// Get all files in the commit tree
	files, err := GetCommitTreeFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitTreeFiles_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit multiple files
	createFile(t, tmpDir, "file1.txt", "content1")
	createFile(t, tmpDir, "file2.txt", "content2")
	createFile(t, tmpDir, "file3.txt", "content3")
	gitAdd(t, tmpDir, "file1.txt")
	gitAdd(t, tmpDir, "file2.txt")
	gitAdd(t, tmpDir, "file3.txt")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add multiple files")

	files, err := GetCommitTreeFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitTreeFiles_FilesInSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create subdirectories
	libDir := filepath.Join(tmpDir, "lib")
	srcDir := filepath.Join(tmpDir, "src", "utils")
	require.NoError(t, os.MkdirAll(libDir, 0755))
	require.NoError(t, os.MkdirAll(srcDir, 0755))

	// Create files in various locations
	createFile(t, tmpDir, "main.go", "package main")
	createFile(t, libDir, "lib.go", "package lib")
	createFile(t, srcDir, "utils.go", "package utils")

	gitAdd(t, tmpDir, "main.go")
	gitAdd(t, tmpDir, "lib/lib.go")
	gitAdd(t, tmpDir, "src/utils/utils.go")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add files in subdirectories")

	files, err := GetCommitTreeFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitTreeFiles_HEAD(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	// Use HEAD reference
	files, err := GetCommitTreeFiles(tmpDir, "HEAD")

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitTreeFiles_OlderCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// First commit with one file
	createFile(t, tmpDir, "first.txt", "first content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Second commit adds another file
	createFile(t, tmpDir, "second.txt", "second content")
	gitAdd(t, tmpDir, "second.txt")
	gitCommit(t, tmpDir, "Second commit")

	// Get tree from first commit - should only have first file
	files, err := GetCommitTreeFiles(tmpDir, firstCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitTreeFiles_AfterFileDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// First commit with two files
	createFile(t, tmpDir, "keep.txt", "keep")
	file2 := createFile(t, tmpDir, "delete.txt", "delete")
	gitAdd(t, tmpDir, "keep.txt")
	gitAdd(t, tmpDir, "delete.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "Add two files")

	// Second commit deletes one file
	require.NoError(t, os.Remove(file2))
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())
	gitCommit(t, tmpDir, "Delete file")

	// First commit tree should still have both files
	files, err := GetCommitTreeFiles(tmpDir, firstCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitTreeFiles_ReturnsAllFilesAcrossCommits(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create files in multiple commits
	createFile(t, tmpDir, "first.txt", "first")
	gitAdd(t, tmpDir, "first.txt")
	gitCommit(t, tmpDir, "First commit")

	createFile(t, tmpDir, "second.txt", "second")
	gitAdd(t, tmpDir, "second.txt")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	// Should return all files that exist at this commit
	files, err := GetCommitTreeFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitTreeFiles_InvalidCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	_, err := GetCommitTreeFiles(tmpDir, "invalid-sha-that-does-not-exist")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid commit reference")
}

func TestGetCommitTreeFiles_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't initialize git

	_, err := GetCommitTreeFiles(tmpDir, "HEAD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestGetCommitTreeFiles_InvalidPath(t *testing.T) {
	_, err := GetCommitTreeFiles("/nonexistent/path", "HEAD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}
