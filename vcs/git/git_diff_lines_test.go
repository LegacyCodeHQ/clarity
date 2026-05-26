package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHunkHeader(t *testing.T) {
	tests := []struct {
		header     string
		oldS, oldC int
		newS, newC int
		ok         bool
	}{
		{"@@ -10,3 +14,2 @@", 10, 3, 14, 2, true},
		{"@@ -1 +1 @@", 1, 1, 1, 1, true},
		{"@@ -1,0 +5,3 @@ ctx", 1, 0, 5, 3, true},
		{"@@ malformed", 0, 0, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.header, func(t *testing.T) {
			oS, oC, nS, nC, ok := parseHunkHeader(tc.header)
			assert.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.oldS, oS)
				assert.Equal(t, tc.oldC, oC)
				assert.Equal(t, tc.newS, nS)
				assert.Equal(t, tc.newC, nC)
			}
		})
	}
}

func TestParseUnifiedDiff_AdditionsAndDeletions(t *testing.T) {
	raw := strings.Join([]string{
		"diff --git a/src/lib.rs b/src/lib.rs",
		"--- a/src/lib.rs",
		"+++ b/src/lib.rs",
		"@@ -4,0 +5,2 @@",
		"+added one",
		"+added two",
		"@@ -12 +14,0 @@",
		"-removed one",
		"",
	}, "\n")

	got := parseUnifiedDiff(raw)
	require.Contains(t, got, "src/lib.rs")
	fd := got["src/lib.rs"]
	assert.Equal(t, []int{5, 6}, fd.Additions)
	assert.Equal(t, []int{12}, fd.Deletions)
	assert.False(t, fd.IsNew)
}

func TestParseUnifiedDiff_NoCountHunk(t *testing.T) {
	// `@@ -5 +7 @@` (count omitted on both sides → 1 line each)
	raw := strings.Join([]string{
		"diff --git a/f b/f",
		"--- a/f",
		"+++ b/f",
		"@@ -5 +7 @@",
		"-old line",
		"+new line",
		"",
	}, "\n")
	got := parseUnifiedDiff(raw)
	fd := got["f"]
	assert.Equal(t, []int{5}, fd.Deletions)
	assert.Equal(t, []int{7}, fd.Additions)
}

func TestParseUnifiedDiff_NewFile(t *testing.T) {
	raw := strings.Join([]string{
		"diff --git a/src/new.rs b/src/new.rs",
		"new file mode 100644",
		"index 0000000..abcdef",
		"--- /dev/null",
		"+++ b/src/new.rs",
		"@@ -0,0 +1,3 @@",
		"+pub fn a() {}",
		"+",
		"+pub fn b() {}",
		"",
	}, "\n")

	got := parseUnifiedDiff(raw)
	fd, ok := got["src/new.rs"]
	require.True(t, ok)
	assert.True(t, fd.IsNew)
	assert.Equal(t, []int{1, 2, 3}, fd.Additions)
}

func TestGetCommitFileDiffs(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "lib.rs", "line a\nline b\nline c\n")
	gitAdd(t, tmpDir, "lib.rs")
	gitCommit(t, tmpDir, "initial")

	createFile(t, tmpDir, "lib.rs", "line a\nline b2\nline c\nline d\n")
	gitAdd(t, tmpDir, "lib.rs")
	sha := gitCommitAndGetSHA(t, tmpDir, "second")

	diffs, err := GetCommitFileDiffs(tmpDir, sha)
	require.NoError(t, err)

	resolvedDir, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	fd, ok := diffs[filepath.Join(resolvedDir, "lib.rs")]
	require.True(t, ok)
	assert.Equal(t, []int{2}, fd.Deletions, "original line 2 (line b) removed")
	assert.Equal(t, []int{2, 4}, fd.Additions, "new line 2 (line b2) and new line 4 (line d)")
}

func TestGetCommitFileDiffs_RootCommit(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "lib.rs", "line a\nline b\n")
	gitAdd(t, tmpDir, "lib.rs")
	sha := gitCommitAndGetSHA(t, tmpDir, "root")

	diffs, err := GetCommitFileDiffs(tmpDir, sha)
	require.NoError(t, err, "root commit should fall back to --root")
	resolvedDir, _ := filepath.EvalSymlinks(tmpDir)
	fd, ok := diffs[filepath.Join(resolvedDir, "lib.rs")]
	require.True(t, ok)
	assert.True(t, fd.IsNew)
	assert.Equal(t, []int{1, 2}, fd.Additions)
}

func TestGetUncommittedFileDiffs(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	createFile(t, tmpDir, "lib.rs", "line a\nline b\nline c\nline d\n")
	gitAdd(t, tmpDir, "lib.rs")
	gitCommit(t, tmpDir, "initial")

	// Delete "line b" (original line 2); insert "line c2" after "line c"
	// (new file line 3).
	createFile(t, tmpDir, "lib.rs", "line a\nline c\nline c2\nline d\n")

	diffs, err := GetUncommittedFileDiffs(tmpDir)
	require.NoError(t, err)

	resolvedDir, err := filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	absPath := filepath.Join(resolvedDir, "lib.rs")
	fd, ok := diffs[absPath]
	require.True(t, ok)
	assert.Equal(t, []int{2}, fd.Deletions)
	assert.Equal(t, []int{3}, fd.Additions)
}
