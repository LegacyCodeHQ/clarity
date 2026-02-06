package git

import (
	"testing"

	"github.com/LegacyCodeHQ/sanity/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUncommittedFileStats_MarksNewAndUntrackedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	committedFile := createDartFile(t, tmpDir, "committed.dart")
	gitAdd(t, tmpDir, "committed.dart")
	gitCommit(t, tmpDir, "Initial commit")

	createDartFile(t, tmpDir, "staged.dart")
	gitAdd(t, tmpDir, "staged.dart")

	createDartFile(t, tmpDir, "untracked.dart")

	modifyFile(t, committedFile)

	stats, err := GetUncommittedFileStats(tmpDir)
	require.NoError(t, err)

	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFileStats(tmpDir, stats)))
}

func TestGetCommitFileStats_MarksNewFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	existingFile := createDartFile(t, tmpDir, "existing.dart")
	gitAdd(t, tmpDir, "existing.dart")
	gitCommit(t, tmpDir, "Initial commit")

	createDartFile(t, tmpDir, "added.dart")
	gitAdd(t, tmpDir, "added.dart")
	modifyFile(t, existingFile)
	gitAdd(t, tmpDir, "existing.dart")
	commitID := gitCommitAndGetSHA(t, tmpDir, "Add new file and modify existing")

	stats, err := GetCommitFileStats(tmpDir, commitID)
	require.NoError(t, err)

	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFileStats(tmpDir, stats)))
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
			input:    "depgraph/{ => dart}/parser_dart.go",
			expected: "depgraph/dart/parser_dart.go",
		},
		{
			name:     "both old and new paths",
			input:    "depgraph/{old => new}/parser.go",
			expected: "depgraph/new/parser.go",
		},
		{
			name:     "file rename in same directory",
			input:    "src/{old_name.go => new_name.go}",
			expected: "src/new_name.go",
		},
		{
			name:     "complex path with multiple parts",
			input:    "lib/parsers/{ => golang}/golang_parser_test.go",
			expected: "lib/parsers/golang/golang_parser_test.go",
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
			input:    "depgraph/dependency_graph.go",
			expected: "depgraph/dependency_graph.go",
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

// Tests for GetCommitRangeFileStats

func TestGetCommitRangeFileStats_AdditionsAndDeletions(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "test.txt", "line1\nline2\nline3\n")
	gitAdd(t, tmpDir, "test.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Modify file: add 2 lines, remove 1
	createFile(t, tmpDir, "test.txt", "line1\nline3\nnew1\nnew2\n")
	gitAdd(t, tmpDir, "test.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	stats, err := GetCommitRangeFileStats(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFileStats(tmpDir, stats)))
}

func TestGetCommitRangeFileStats_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "first.txt", "content")
	gitAdd(t, tmpDir, "first.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Add new file
	createFile(t, tmpDir, "new.txt", "line1\nline2\n")
	gitAdd(t, tmpDir, "new.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	stats, err := GetCommitRangeFileStats(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFileStats(tmpDir, stats)))
}

func TestGetCommitRangeFileStats_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create first commit
	createFile(t, tmpDir, "file1.txt", "content")
	gitAdd(t, tmpDir, "file1.txt")
	firstCommit := gitCommitAndGetSHA(t, tmpDir, "First commit")

	// Modify and add new file
	createFile(t, tmpDir, "file1.txt", "content\nnew line\n")
	createFile(t, tmpDir, "file2.txt", "new file\n")
	gitAdd(t, tmpDir, "file1.txt")
	gitAdd(t, tmpDir, "file2.txt")
	secondCommit := gitCommitAndGetSHA(t, tmpDir, "Second commit")

	stats, err := GetCommitRangeFileStats(tmpDir, firstCommit, secondCommit)

	require.NoError(t, err)
	g := testhelpers.TextGoldie(t)
	g.Assert(t, t.Name(), []byte(normalizeFileStats(tmpDir, stats)))
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
