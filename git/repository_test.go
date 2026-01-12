package git

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to setup a git repository in a temporary directory
func setupGitRepo(t *testing.T, dir string) {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	require.NoError(t, cmd.Run(), "failed to initialize git repository")

	// Configure git user to avoid errors
	gitConfig(t, dir, "user.name", "Test User")
	gitConfig(t, dir, "user.email", "test@example.com")
}

// Helper function to set git config
func gitConfig(t *testing.T, repoDir, key, value string) {
	cmd := exec.Command("git", "config", key, value)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run(), "failed to set git config %s", key)
}

// Helper function to create a file with content
func createFile(t *testing.T, dir, name, content string) string {
	filePath := filepath.Join(dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "failed to create file %s", name)
	return filePath
}

// Helper function to create a dart file
func createDartFile(t *testing.T, dir, name string) string {
	return createFile(t, dir, name, "import 'dart:io';\n\nclass Test {}")
}

// Helper function to add a file to git staging area
func gitAdd(t *testing.T, repoDir, file string) {
	cmd := exec.Command("git", "add", file)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run(), "failed to git add %s", file)
}

// Helper function to commit files
func gitCommit(t *testing.T, repoDir, message string) {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run(), "failed to git commit")
}

// Helper function to commit files and return the commit SHA
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

// Helper function to modify a file
func modifyFile(t *testing.T, filePath string) {
	err := os.WriteFile(filePath, []byte("modified content\n"), 0644)
	require.NoError(t, err, "failed to modify file %s", filePath)
}

func TestGetUncommittedDartFiles_UntrackedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create untracked .dart files
	dartFile1 := createDartFile(t, tmpDir, "test1.dart")
	dartFile2 := createDartFile(t, tmpDir, "test2.dart")

	// Get uncommitted files
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Len(t, files, 2)

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	resolved1, _ := filepath.EvalSymlinks(dartFile1)
	resolved2, _ := filepath.EvalSymlinks(dartFile2)
	assert.Contains(t, files, resolved1)
	assert.Contains(t, files, resolved2)
}

func TestGetUncommittedDartFiles_StagedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and stage .dart files
	dartFile := createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")

	// Get uncommitted files
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	resolved, _ := filepath.EvalSymlinks(dartFile)
	assert.Contains(t, files, resolved)
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
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	resolved, _ := filepath.EvalSymlinks(dartFile)
	assert.Contains(t, files, resolved)
}

func TestGetUncommittedDartFiles_MixedStates(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create a committed file
	committedFile := createDartFile(t, tmpDir, "committed.dart")
	gitAdd(t, tmpDir, "committed.dart")
	gitCommit(t, tmpDir, "Initial commit")

	// Create a staged file
	stagedFile := createDartFile(t, tmpDir, "staged.dart")
	gitAdd(t, tmpDir, "staged.dart")

	// Create an untracked file
	untrackedFile := createDartFile(t, tmpDir, "untracked.dart")

	// Modify the committed file
	modifyFile(t, committedFile)

	// Get uncommitted files
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Len(t, files, 3)
	resolved1, _ := filepath.EvalSymlinks(committedFile)
	resolved2, _ := filepath.EvalSymlinks(stagedFile)
	resolved3, _ := filepath.EvalSymlinks(untrackedFile)
	assert.Contains(t, files, resolved1) // modified
	assert.Contains(t, files, resolved2) // staged
	assert.Contains(t, files, resolved3) // untracked
}

func TestGetUncommittedDartFiles_IncludesAllFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create various file types
	dartFile := createDartFile(t, tmpDir, "test.dart")
	goFile := createFile(t, tmpDir, "test.go", "package main")
	mdFile := createFile(t, tmpDir, "README.md", "# Test")
	txtFile := createFile(t, tmpDir, "test.txt", "text file")

	// Get uncommitted files
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Len(t, files, 4, "should include all file types")
	resolvedDart, _ := filepath.EvalSymlinks(dartFile)
	resolvedGo, _ := filepath.EvalSymlinks(goFile)
	resolvedMd, _ := filepath.EvalSymlinks(mdFile)
	resolvedTxt, _ := filepath.EvalSymlinks(txtFile)
	assert.Contains(t, files, resolvedDart)
	assert.Contains(t, files, resolvedGo)
	assert.Contains(t, files, resolvedMd)
	assert.Contains(t, files, resolvedTxt)
}

func TestGetUncommittedDartFiles_NoUncommittedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit a file
	createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	gitCommit(t, tmpDir, "Initial commit")

	// Get uncommitted files (should be empty)
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestGetUncommittedDartFiles_EmptyRepo(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Get uncommitted files from empty repo
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestGetUncommittedDartFiles_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't initialize git

	// Try to get uncommitted files
	_, err := GetUncommittedDartFiles(tmpDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestGetUncommittedDartFiles_InvalidPath(t *testing.T) {
	_, err := GetUncommittedDartFiles("/nonexistent/path")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestIsGitRepository_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	isRepo := isGitRepository(tmpDir)

	assert.True(t, isRepo)
}

func TestIsGitRepository_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't initialize git

	isRepo := isGitRepository(tmpDir)

	assert.False(t, isRepo)
}

func TestFilterDartFiles(t *testing.T) {
	files := []string{
		"test1.dart",
		"test2.go",
		"README.md",
		"test3.dart",
		"script.sh",
	}

	dartFiles := filterDartFiles(files)

	assert.Len(t, dartFiles, 2)
	assert.Contains(t, dartFiles, "test1.dart")
	assert.Contains(t, dartFiles, "test3.dart")
}

func TestToAbsolutePaths(t *testing.T) {
	repoRoot := "/Users/test/repo"
	relativePaths := []string{
		"lib/main.dart",
		"test/widget_test.dart",
		"models/user.dart",
	}

	absolutePaths := toAbsolutePaths(repoRoot, relativePaths)

	assert.Len(t, absolutePaths, 3)
	assert.Equal(t, "/Users/test/repo/lib/main.dart", absolutePaths[0])
	assert.Equal(t, "/Users/test/repo/test/widget_test.dart", absolutePaths[1])
	assert.Equal(t, "/Users/test/repo/models/user.dart", absolutePaths[2])
}

// Tests for GetCommitDartFiles

func TestGetCommitDartFiles_SingleCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit .dart file
	dartFile := createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add test.dart")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	resolved, _ := filepath.EvalSymlinks(dartFile)
	assert.Contains(t, files, resolved)
}

func TestGetCommitDartFiles_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit multiple .dart files
	dartFile1 := createDartFile(t, tmpDir, "test1.dart")
	dartFile2 := createDartFile(t, tmpDir, "test2.dart")
	dartFile3 := createDartFile(t, tmpDir, "test3.dart")
	gitAdd(t, tmpDir, "test1.dart")
	gitAdd(t, tmpDir, "test2.dart")
	gitAdd(t, tmpDir, "test3.dart")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add multiple dart files")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	assert.Len(t, files, 3)
	resolved1, _ := filepath.EvalSymlinks(dartFile1)
	resolved2, _ := filepath.EvalSymlinks(dartFile2)
	resolved3, _ := filepath.EvalSymlinks(dartFile3)
	assert.Contains(t, files, resolved1)
	assert.Contains(t, files, resolved2)
	assert.Contains(t, files, resolved3)
}

func TestGetCommitDartFiles_IncludesAllFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit various file types
	dartFile := createDartFile(t, tmpDir, "test.dart")
	goFile := createFile(t, tmpDir, "test.go", "package main")
	mdFile := createFile(t, tmpDir, "README.md", "# Test")

	gitAdd(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.go")
	gitAdd(t, tmpDir, "README.md")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add mixed files")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	assert.Len(t, files, 3, "should include all file types")
	resolvedDart, _ := filepath.EvalSymlinks(dartFile)
	resolvedGo, _ := filepath.EvalSymlinks(goFile)
	resolvedMd, _ := filepath.EvalSymlinks(mdFile)
	assert.Contains(t, files, resolvedDart)
	assert.Contains(t, files, resolvedGo)
	assert.Contains(t, files, resolvedMd)
}

func TestGetCommitDartFiles_NonDartFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit non-.dart files
	goFile := createFile(t, tmpDir, "test.go", "package main")
	gitAdd(t, tmpDir, "test.go")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add go file")

	// Get files from commit
	files, err := GetCommitDartFiles(tmpDir, commitID)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	resolvedGo, _ := filepath.EvalSymlinks(goFile)
	assert.Contains(t, files, resolvedGo)
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
	dartFile := createDartFile(t, tmpDir, "test.dart")
	gitAdd(t, tmpDir, "test.dart")
	gitCommit(t, tmpDir, "Add test.dart")

	// Get files from HEAD
	files, err := GetCommitDartFiles(tmpDir, "HEAD")

	require.NoError(t, err)
	assert.Len(t, files, 1)
	resolved, _ := filepath.EvalSymlinks(dartFile)
	assert.Contains(t, files, resolved)
}

func TestGetCommitDartFiles_HeadTildeReference(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	dartFile1 := createDartFile(t, tmpDir, "first.dart")
	gitAdd(t, tmpDir, "first.dart")
	gitCommit(t, tmpDir, "First commit")

	// Create second commit
	createDartFile(t, tmpDir, "second.dart")
	gitAdd(t, tmpDir, "second.dart")
	gitCommit(t, tmpDir, "Second commit")

	// Get files from HEAD~1 (first commit)
	files, err := GetCommitDartFiles(tmpDir, "HEAD~1")

	require.NoError(t, err)
	assert.Len(t, files, 1)
	resolved, _ := filepath.EvalSymlinks(dartFile1)
	assert.Contains(t, files, resolved)
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
