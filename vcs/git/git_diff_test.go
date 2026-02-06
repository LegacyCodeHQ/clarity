package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/sanity/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUncommittedDartFiles_UntrackedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create untracked .dart files
	createDartFile(t, tmpDir, "test1.dart")
	createDartFile(t, tmpDir, "test2.dart")

	// Get uncommitted files
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_UntrackedFilesInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "lib")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// Create untracked files in subdirectory
	createDartFile(t, subDir, "test1.dart")
	createDartFile(t, subDir, "test2.dart")

	// Get uncommitted files
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_StagedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and stage .dart files
	createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")

	// Get uncommitted files
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_ModifiedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create, commit, then modify a .dart file
	dartFile := createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	gitCommit(t, tmpDir, "Initial commit")

	// Modify the file
	modifyFile(t, dartFile)

	// Get uncommitted files
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_MixedStates(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create a committed file
	committedFile := createDartFile(t, tmpDir, "committed.dart")
	gitAdd(t, tmpDir, "committed.dart")
	gitCommit(t, tmpDir, "Initial commit")

	// Create a staged file
	createDartFile(t, tmpDir, "staged.dart")
	gitAdd(t, tmpDir, "staged.dart")

	// Create an untracked file
	createDartFile(t, tmpDir, "untracked.dart")

	// Modify the committed file
	modifyFile(t, committedFile)

	// Get uncommitted files
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_IncludesAllFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create various file types
	createDartFile(t, tmpDir, "test.dart")
	createFile(t, tmpDir, "test.go", "package main")
	createFile(t, tmpDir, "README.md", "# Test")
	createFile(t, tmpDir, "test.txt", "text file")

	// Get uncommitted files
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_NoUncommittedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit a file
	createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	gitCommit(t, tmpDir, "Initial commit")

	// Get uncommitted files (should be empty)
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_EmptyRepo(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Get uncommitted files from empty repo
	files, err := GetUncommittedFiles(tmpDir)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetUncommittedDartFiles_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't initialize git

	// Try to get uncommitted files
	_, err := GetUncommittedFiles(tmpDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestGetUncommittedDartFiles_InvalidPath(t *testing.T) {
	_, err := GetUncommittedFiles("/nonexistent/path")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

// Tests for GetCommitDartFiles

func TestGetCommitDartFiles_SingleCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit .dart file
	createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add test.dart")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitDartFiles_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit multiple .dart files
	createDartFile(t, tmpDir, "test1.dart")
	createDartFile(t, tmpDir, "test2.dart")
	createDartFile(t, tmpDir, "test3.dart")
	gitAdd(t, tmpDir, "test1.dart")
	gitAdd(t, tmpDir, "test2.dart")
	gitAdd(t, tmpDir, "test3.dart")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add multiple dart files")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitDartFiles_IncludesAllFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit various file types
	createDartFile(t, tmpDir, "test.dart")
	createFile(t, tmpDir, "test.go", "package main")
	createFile(t, tmpDir, "README.md", "# Test")

	gitAdd(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.go")
	gitAdd(t, tmpDir, "README.md")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add mixed files")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitDartFiles_NonDartFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit non-.dart files
	createFile(t, tmpDir, "test.go", "package main")
	gitAdd(t, tmpDir, "test.go")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add go file")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitDartFiles_InvalidCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create an initial commit
	createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	gitCommit(t, tmpDir, "Initial commit")

	// Try to get files from invalid commit
	_, err := GetCommitDartFiles(tmpDir, "invalid-commit-sha")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid commit reference")
}

func TestGetCommitDartFiles_HeadReference(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createDartFile(t, tmpDir, "first.dart")
	gitAdd(t, tmpDir, "first.dart")
	gitCommit(t, tmpDir, "First commit")

	// Create second commit with HEAD test
	createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	gitCommit(t, tmpDir, "Add test.dart")

	// Get files from HEAD
	files, err := GetCommitDartFiles(tmpDir, "HEAD")

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitDartFiles_HeadTildeReference(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createDartFile(t, tmpDir, "first.dart")
	gitAdd(t, tmpDir, "first.dart")
	gitCommit(t, tmpDir, "First commit")

	// Create second commit
	createDartFile(t, tmpDir, "second.dart")
	gitAdd(t, tmpDir, "second.dart")
	gitCommit(t, tmpDir, "Second commit")

	// Get files from HEAD~1 (first commit)
	files, err := GetCommitDartFiles(tmpDir, "HEAD~1")

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitDartFiles_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't initialize git

	_, err := GetCommitDartFiles(tmpDir, "HEAD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestGetCommitDartFiles_InvalidPath(t *testing.T) {
	_, err := GetCommitDartFiles("/nonexistent/path", "HEAD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

// Tests for GetFileContentFromCommit

func TestGetFileContentFromCommit_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit a file with specific content
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	createFile(t, tmpDir, "main.go", content)
	gitAdd(t, tmpDir, "main.go")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add main.go")

	// Read the file content from commit
	result, err := GetFileContentFromCommit(tmpDir, commitID, "main.go")

	require.NoError(t, err)
	assert.Equal(t, content, string(result))
}

func TestGetFileContentFromCommit_OlderCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit a file with initial content
	initialContent := "version 1"
	createFile(t, tmpDir, "version.txt", initialContent)
	gitAdd(t, tmpDir, "version.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "Version 1")

	// Modify and commit again
	modifiedContent := "version 2"
	createFile(t, tmpDir, "version.txt", modifiedContent)
	gitAdd(t, tmpDir, "version.txt")
	gitCommit(t, tmpDir, "Version 2")

	// Read the old content from first commit
	result, err := GetFileContentFromCommit(tmpDir, firstCommit, "version.txt")

	require.NoError(t, err)
	assert.Equal(t, initialContent, string(result))
}

func TestGetFileContentFromCommit_FileInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create subdirectory and file
	subDir := filepath.Join(tmpDir, "lib")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	content := "library code"
	createFile(t, subDir, "lib.go", content)
	gitAdd(t, tmpDir, "lib/lib.go")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add lib.go")

	// Read using relative path from repo root
	result, err := GetFileContentFromCommit(tmpDir, commitID, "lib/lib.go")

	require.NoError(t, err)
	assert.Equal(t, content, string(result))
}

func TestGetFileContentFromCommit_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create a commit with one file
	createFile(t, tmpDir, "exists.txt", "content")
	gitAdd(t, tmpDir, "exists.txt")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add file")

	// Try to read a non-existent file
	_, err := GetFileContentFromCommit(tmpDir, commitID, "nonexistent.txt")

	assert.Error(t, err)
}

func TestGetFileContentFromCommit_InvalidCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	_, err := GetFileContentFromCommit(tmpDir, "invalid-sha", "test.txt")

	assert.Error(t, err)
}

// Tests for GetCommitRangeFiles

func TestGetCommitRangeFiles_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Create second commit with new file
	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	files, err := GetCommitRangeFiles(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitRangeFiles_MultipleCommits(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Create second commit
	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	gitCommit(t, tmpDir, "Second commit")

	// Create third commit
	createFile(t, tmpDir, "third.txt", "content")
	gitAdd(t, tmpDir, "third.txt")
	thirdCommit := gitCommitAndGetSHA(t, tmpDir, "Third commit")

	// Get files from first to third commit
	files, err := GetCommitRangeFiles(tmpDir, firstCommit, thirdCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitRangeFiles_ModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	file := createFile(t, tmpDir, "test.txt", "initial content")
	gitAdd(t, tmpDir, "test.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Modify and commit again
	modifyFile(t, file)
	gitAdd(t, tmpDir, "test.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	files, err := GetCommitRangeFiles(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitRangeFiles_ExcludesDeletedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit with two files
	createFile(t, tmpDir, "keep.txt", "content")
	toDelete := createFile(t, tmpDir, "delete.txt", "content")
	gitAdd(t, tmpDir, "keep.txt")
	gitAdd(t, tmpDir, "delete.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Delete one file
	err := os.Remove(toDelete)
	require.NoError(t, err)
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Delete file")

	files, err := GetCommitRangeFiles(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitRangeFiles_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	// Same commit for from and to
	files, err := GetCommitRangeFiles(tmpDir, commit, commit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFilePaths(tmpDir, files)))
}

func TestGetCommitRangeFiles_InvalidFromCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	_, err := GetCommitRangeFiles(tmpDir, "invalid-sha", commit)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid commit reference")
}

func TestGetCommitRangeFiles_InvalidToCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	_, err := GetCommitRangeFiles(tmpDir, commit, "invalid-sha")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid commit reference")
}

func TestGetCommitRangeFiles_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetCommitRangeFiles(tmpDir, "abc", "def")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestGetCommitRangeFiles_InvalidPath(t *testing.T) {
	_, err := GetCommitRangeFiles("/nonexistent/path", "abc", "def")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}
