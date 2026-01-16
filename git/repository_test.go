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

func TestGetUncommittedDartFiles_UntrackedFilesInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "lib")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// Create untracked files in subdirectory
	dartFile1 := createDartFile(t, subDir, "test1.dart")
	dartFile2 := createDartFile(t, subDir, "test2.dart")

	// Get uncommitted files
	files, err := GetUncommittedDartFiles(tmpDir)

	require.NoError(t, err)
	assert.Len(t, files, 2, "should find individual files in subdirectory, not just directory name")

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

func TestGetUncommittedFileStats_MarksNewAndUntrackedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	committedFile := createDartFile(t, tmpDir, "committed.dart")
	gitAdd(t, tmpDir, "committed.dart")
	gitCommit(t, tmpDir, "Initial commit")

	stagedFile := createDartFile(t, tmpDir, "staged.dart")
	gitAdd(t, tmpDir, "staged.dart")

	untrackedFile := createDartFile(t, tmpDir, "untracked.dart")

	modifyFile(t, committedFile)

	stats, err := GetUncommittedFileStats(tmpDir)
	require.NoError(t, err)

	resolvedStaged, _ := filepath.EvalSymlinks(stagedFile)
	stagedStats, ok := stats[resolvedStaged]
	require.True(t, ok)
	assert.True(t, stagedStats.IsNew)

	resolvedUntracked, _ := filepath.EvalSymlinks(untrackedFile)
	untrackedStats, ok := stats[resolvedUntracked]
	require.True(t, ok)
	assert.True(t, untrackedStats.IsNew)

	resolvedCommitted, _ := filepath.EvalSymlinks(committedFile)
	committedStats, ok := stats[resolvedCommitted]
	require.True(t, ok)
	assert.False(t, committedStats.IsNew)
}

func TestGetCommitFileStats_MarksNewFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	existingFile := createDartFile(t, tmpDir, "existing.dart")
	gitAdd(t, tmpDir, "existing.dart")
	gitCommit(t, tmpDir, "Initial commit")

	addedFile := createDartFile(t, tmpDir, "added.dart")
	gitAdd(t, tmpDir, "added.dart")
	modifyFile(t, existingFile)
	gitAdd(t, tmpDir, "existing.dart")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add new file and modify existing")

	stats, err := GetCommitFileStats(tmpDir, commitID)
	require.NoError(t, err)

	resolvedAdded, _ := filepath.EvalSymlinks(addedFile)
	addedStats, ok := stats[resolvedAdded]
	require.True(t, ok)
	assert.True(t, addedStats.IsNew)

	resolvedExisting, _ := filepath.EvalSymlinks(existingFile)
	existingStats, ok := stats[resolvedExisting]
	require.True(t, ok)
	assert.False(t, existingStats.IsNew)
}

// Tests for parseRenamedFilePath

func TestParseRenamedFilePath_AbbreviatedFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty old path",
			input:    "parsers/{ => dart}/dart_parser.go",
			expected: "parsers/dart/dart_parser.go",
		},
		{
			name:     "both old and new paths",
			input:    "parsers/{old => new}/parser.go",
			expected: "parsers/new/parser.go",
		},
		{
			name:     "file rename in same directory",
			input:    "src/{old_name.go => new_name.go}",
			expected: "src/new_name.go",
		},
		{
			name:     "complex path with multiple parts",
			input:    "lib/parsers/{ => go}/go_parser_test.go",
			expected: "lib/parsers/go/go_parser_test.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRenamedFilePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRenamedFilePath_FullFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple rename",
			input:    "old/path/file.go => new/path/file.go",
			expected: "new/path/file.go",
		},
		{
			name:     "rename with spaces in path",
			input:    "old path/file.go => new path/file.go",
			expected: "new path/file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRenamedFilePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRenamedFilePath_NoRename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "regular file path",
			input:    "parsers/dependency_graph.go",
			expected: "parsers/dependency_graph.go",
		},
		{
			name:     "file with braces but no arrow",
			input:    "lib/{utils}/helper.go",
			expected: "lib/{utils}/helper.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRenamedFilePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Tests for GetRepositoryRoot

func TestGetRepositoryRoot_FromRepoRoot(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	root, err := GetRepositoryRoot(tmpDir)

	require.NoError(t, err)
	// Resolve symlinks for comparison (macOS /var -> /private/var)
	resolvedTmp, _ := filepath.EvalSymlinks(tmpDir)
	assert.Equal(t, resolvedTmp, root)
}

func TestGetRepositoryRoot_FromSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "lib", "src")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	root, err := GetRepositoryRoot(subDir)

	require.NoError(t, err)
	resolvedTmp, _ := filepath.EvalSymlinks(tmpDir)
	assert.Equal(t, resolvedTmp, root)
}

func TestGetRepositoryRoot_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't initialize git

	_, err := GetRepositoryRoot(tmpDir)

	assert.Error(t, err)
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

// Tests for GetCurrentCommitHash

func TestGetCurrentCommitHash_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	hash, err := GetCurrentCommitHash(tmpDir)

	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	// Short hash is typically 7 characters
	assert.True(t, len(hash) >= 7 && len(hash) <= 12, "hash should be short format")
}

func TestGetCurrentCommitHash_MatchesHEAD(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	expectedHash := gitCommitAndGetSHA(t, tmpDir, "Initial commit")

	hash, err := GetCurrentCommitHash(tmpDir)

	require.NoError(t, err)
	// The current hash should be a prefix of the full commit SHA
	assert.True(t, strings.HasPrefix(expectedHash, hash), "current hash should match HEAD")
}

func TestGetCurrentCommitHash_NoCommits(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// No commits made
	_, err := GetCurrentCommitHash(tmpDir)

	assert.Error(t, err)
}

// Tests for GetShortCommitHash

func TestGetShortCommitHash_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	fullHash := gitCommitAndGetSHA(t, tmpDir, "Initial commit")

	shortHash, err := GetShortCommitHash(tmpDir, fullHash)

	require.NoError(t, err)
	assert.True(t, len(shortHash) >= 7 && len(shortHash) <= 12)
	assert.True(t, strings.HasPrefix(fullHash, shortHash))
}

func TestGetShortCommitHash_AlreadyShort(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	fullHash := gitCommitAndGetSHA(t, tmpDir, "Initial commit")

	// Get short hash first
	shortHash, err := GetShortCommitHash(tmpDir, fullHash[:7])

	require.NoError(t, err)
	assert.NotEmpty(t, shortHash)
}

func TestGetShortCommitHash_HEAD(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	shortHash, err := GetShortCommitHash(tmpDir, "HEAD")

	require.NoError(t, err)
	assert.NotEmpty(t, shortHash)
}

func TestGetShortCommitHash_InvalidCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	_, err := GetShortCommitHash(tmpDir, "invalid-sha-that-does-not-exist")

	assert.Error(t, err)
}

// Tests for HasUncommittedChanges

func TestHasUncommittedChanges_Clean(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	hasChanges, err := HasUncommittedChanges(tmpDir)

	require.NoError(t, err)
	assert.False(t, hasChanges)
}

func TestHasUncommittedChanges_UntrackedFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "committed.txt", "content")
	gitAdd(t, tmpDir, "committed.txt")
	gitCommit(t, tmpDir, "Initial commit")

	// Create untracked file
	createFile(t, tmpDir, "untracked.txt", "new content")

	hasChanges, err := HasUncommittedChanges(tmpDir)

	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestHasUncommittedChanges_ModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	filePath := createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	gitCommit(t, tmpDir, "Initial commit")

	// Modify the file
	modifyFile(t, filePath)

	hasChanges, err := HasUncommittedChanges(tmpDir)

	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestHasUncommittedChanges_StagedFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	gitCommit(t, tmpDir, "Initial commit")

	// Create and stage a new file
	createFile(t, tmpDir, "staged.txt", "new content")
	gitAdd(t, tmpDir, "staged.txt")

	hasChanges, err := HasUncommittedChanges(tmpDir)

	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestHasUncommittedChanges_EmptyRepo(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Empty repo with no commits
	hasChanges, err := HasUncommittedChanges(tmpDir)

	require.NoError(t, err)
	assert.False(t, hasChanges)
}

// Tests for ParseCommitRange

func TestParseCommitRange_ThreeDotSyntax(t *testing.T) {
	from, to, isRange := ParseCommitRange("abc123...def456")

	assert.Equal(t, "abc123", from)
	assert.Equal(t, "def456", to)
	assert.True(t, isRange)
}

func TestParseCommitRange_TwoDotSyntax(t *testing.T) {
	from, to, isRange := ParseCommitRange("abc123..def456")

	assert.Equal(t, "abc123", from)
	assert.Equal(t, "def456", to)
	assert.True(t, isRange)
}

func TestParseCommitRange_SingleCommit(t *testing.T) {
	from, to, isRange := ParseCommitRange("abc123")

	assert.Equal(t, "", from)
	assert.Equal(t, "abc123", to)
	assert.False(t, isRange)
}

func TestParseCommitRange_HEAD(t *testing.T) {
	from, to, isRange := ParseCommitRange("HEAD")

	assert.Equal(t, "", from)
	assert.Equal(t, "HEAD", to)
	assert.False(t, isRange)
}

func TestParseCommitRange_HEADTilde(t *testing.T) {
	from, to, isRange := ParseCommitRange("HEAD~5...HEAD")

	assert.Equal(t, "HEAD~5", from)
	assert.Equal(t, "HEAD", to)
	assert.True(t, isRange)
}

func TestParseCommitRange_ThreeDotPreferredOverTwoDot(t *testing.T) {
	// Edge case: input contains both ... and ..
	// Should split on ... first
	from, to, isRange := ParseCommitRange("a..b...c")

	assert.Equal(t, "a..b", from)
	assert.Equal(t, "c", to)
	assert.True(t, isRange)
}

// Tests for isAncestor

func TestIsAncestor_True(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Create second commit
	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	// First should be ancestor of second
	result, err := isAncestor(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	assert.True(t, result)
}

func TestIsAncestor_False(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Create second commit
	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	// Second should NOT be ancestor of first
	result, err := isAncestor(tmpDir, secondCommit, firstCommit)

	require.NoError(t, err)
	assert.False(t, result)
}

func TestIsAncestor_SameCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	// A commit is its own ancestor
	result, err := isAncestor(tmpDir, commit, commit)

	require.NoError(t, err)
	assert.True(t, result)
}

// Tests for NormalizeCommitRange

func TestNormalizeCommitRange_AlreadyCorrectOrder(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	older := gitCommitAndGetSHA(t, tmpDir, "First commit")

	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	newer := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	from, to, swapped, err := NormalizeCommitRange(tmpDir, older, newer)

	require.NoError(t, err)
	assert.Equal(t, older, from)
	assert.Equal(t, newer, to)
	assert.False(t, swapped)
}

func TestNormalizeCommitRange_ReversedOrder(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	older := gitCommitAndGetSHA(t, tmpDir, "First commit")

	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	newer := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	// Pass in reversed order (newer...older)
	from, to, swapped, err := NormalizeCommitRange(tmpDir, newer, older)

	require.NoError(t, err)
	assert.Equal(t, older, from)
	assert.Equal(t, newer, to)
	assert.True(t, swapped)
}

func TestNormalizeCommitRange_SameCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	from, to, swapped, err := NormalizeCommitRange(tmpDir, commit, commit)

	require.NoError(t, err)
	assert.Equal(t, commit, from)
	assert.Equal(t, commit, to)
	assert.False(t, swapped)
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
	newFile := createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	files, err := GetCommitRangeFiles(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	resolved, _ := filepath.EvalSymlinks(newFile)
	assert.Contains(t, files, resolved)
}

func TestGetCommitRangeFiles_MultipleCommits(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Create second commit
	file2 := createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	gitCommit(t, tmpDir, "Second commit")

	// Create third commit
	file3 := createFile(t, tmpDir, "third.txt", "content")
	gitAdd(t, tmpDir, "third.txt")
	thirdCommit := gitCommitAndGetSHA(t, tmpDir, "Third commit")

	// Get files from first to third commit
	files, err := GetCommitRangeFiles(tmpDir, firstCommit, thirdCommit)

	require.NoError(t, err)
	assert.Len(t, files, 2)
	resolved2, _ := filepath.EvalSymlinks(file2)
	resolved3, _ := filepath.EvalSymlinks(file3)
	assert.Contains(t, files, resolved2)
	assert.Contains(t, files, resolved3)
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
	assert.Len(t, files, 1)
	resolved, _ := filepath.EvalSymlinks(file)
	assert.Contains(t, files, resolved)
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
	// Should not contain the deleted file
	assert.Empty(t, files)
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
	assert.Empty(t, files)
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

// Tests for GetCommitRangeFileStats

func TestGetCommitRangeFileStats_AdditionsAndDeletions(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	file := createFile(t, tmpDir, "test.txt", "line1\nline2\nline3\n")
	gitAdd(t, tmpDir, "test.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Modify file: add 2 lines, remove 1
	createFile(t, tmpDir, "test.txt", "line1\nline3\nnew1\nnew2\n")
	gitAdd(t, tmpDir, "test.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	stats, err := GetCommitRangeFileStats(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	resolved, _ := filepath.EvalSymlinks(file)
	fileStats, ok := stats[resolved]
	require.True(t, ok)
	assert.Equal(t, 2, fileStats.Additions)
	assert.Equal(t, 1, fileStats.Deletions)
}

func TestGetCommitRangeFileStats_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Add new file
	newFile := createFile(t, tmpDir, "new.txt", "line1\nline2\n")
	gitAdd(t, tmpDir, "new.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	stats, err := GetCommitRangeFileStats(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	resolved, _ := filepath.EvalSymlinks(newFile)
	fileStats, ok := stats[resolved]
	require.True(t, ok)
	assert.Equal(t, 2, fileStats.Additions)
	assert.Equal(t, 0, fileStats.Deletions)
	assert.True(t, fileStats.IsNew)
}

func TestGetCommitRangeFileStats_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	file1 := createFile(t, tmpDir, "file1.txt", "content")
	gitAdd(t, tmpDir, "file1.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Modify and add new file
	createFile(t, tmpDir, "file1.txt", "content\nnew line\n")
	file2 := createFile(t, tmpDir, "file2.txt", "new file\n")
	gitAdd(t, tmpDir, "file1.txt")
	gitAdd(t, tmpDir, "file2.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	stats, err := GetCommitRangeFileStats(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	assert.Len(t, stats, 2)

	resolved1, _ := filepath.EvalSymlinks(file1)
	resolved2, _ := filepath.EvalSymlinks(file2)
	_, ok1 := stats[resolved1]
	_, ok2 := stats[resolved2]
	assert.True(t, ok1)
	assert.True(t, ok2)
}

func TestGetCommitRangeFileStats_InvalidCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	_, err := GetCommitRangeFileStats(tmpDir, "invalid-sha", commit)

	assert.Error(t, err)
}

func TestGetCommitRangeFileStats_NotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetCommitRangeFileStats(tmpDir, "abc", "def")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

// Tests for GetCommitRangeLabel

func TestGetCommitRangeLabel_Success(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	label, err := GetCommitRangeLabel(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	assert.Contains(t, label, "...")
	// Should be in format "abc123...def456"
	parts := strings.Split(label, "...")
	assert.Len(t, parts, 2)
	assert.True(t, len(parts[0]) >= 7)
	assert.True(t, len(parts[1]) >= 7)
}

func TestGetCommitRangeLabel_WithHEAD(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	createFile(t, tmpDir, "second.txt", "content")
	gitAdd(t, tmpDir, "second.txt")
	gitCommit(t, tmpDir, "Second commit")

	label, err := GetCommitRangeLabel(tmpDir, firstCommit, "HEAD")

	require.NoError(t, err)
	assert.Contains(t, label, "...")
}

func TestGetCommitRangeLabel_InvalidFromCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	_, err := GetCommitRangeLabel(tmpDir, "invalid-sha", commit)

	assert.Error(t, err)
}

func TestGetCommitRangeLabel_InvalidToCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "test.txt", "content")
	gitAdd(t, tmpDir, "test.txt")
	commit := gitCommitAndGetSHA(t, tmpDir, "Commit")

	_, err := GetCommitRangeLabel(tmpDir, commit, "invalid-sha")

	assert.Error(t, err)
}
