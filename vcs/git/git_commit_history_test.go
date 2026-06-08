package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommitHistory_ReturnsCommitsBetweenHeads(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	createFile(t, dir, "main.go", "package main\n")
	gitAdd(t, dir, "main.go")
	first := gitCommitAndGetSHA(t, dir, "initial commit")

	createFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	gitAdd(t, dir, "main.go")
	second := gitCommitAndGetSHA(t, dir, "add main")

	createFile(t, dir, "helper.go", "package main\n")
	gitAdd(t, dir, "helper.go")
	third := gitCommitAndGetSHA(t, dir, "add helper")

	history, err := GetCommitHistory(dir, first, third)

	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, second, history[0].Hash)
	assert.Equal(t, "add main", history[0].Subject)
	assert.Equal(t, "Test User", history[0].Author)
	assert.Equal(t, "test@example.com", history[0].Email)
	assert.NotEmpty(t, history[0].ShortHash)
	assert.False(t, history[0].Timestamp.IsZero())
	assert.Equal(t, third, history[1].Hash)
	assert.Equal(t, "add helper", history[1].Subject)
}

func TestGetCommitHistory_UnbornHeadReturnsTargetCommit(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	createFile(t, dir, "main.go", "package main\n")
	gitAdd(t, dir, "main.go")
	first := gitCommitAndGetSHA(t, dir, "initial commit")

	history, err := GetCommitHistory(dir, unbornHeadSignature, first)

	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, first, history[0].Hash)
	assert.Equal(t, "initial commit", history[0].Subject)
}
