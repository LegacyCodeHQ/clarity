package watch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoIDForPrimary(t *testing.T) {
	assert.Equal(t, "primary", repoIDFor("/any/path", true))
}

func TestRepoIDForLinkedIsStable(t *testing.T) {
	a := repoIDFor("/tmp/foo-feat", false)
	b := repoIDFor("/tmp/foo-feat", false)
	assert.Equal(t, a, b, "repo id must be stable for the same path")
}

func TestRepoIDForLinkedHasPrefixAndLength(t *testing.T) {
	id := repoIDFor("/tmp/foo-feat", false)
	assert.Contains(t, id, "wt-", "linked worktree id should be prefixed")
	assert.Equal(t, len("wt-")+8, len(id), "linked worktree id should be wt- + 8 hex chars")
}

func TestRepoIDForLinkedDistinguishesPaths(t *testing.T) {
	a := repoIDFor("/tmp/foo-feat-a", false)
	b := repoIDFor("/tmp/foo-feat-b", false)
	assert.NotEqual(t, a, b)
}

func TestRepoLabel(t *testing.T) {
	cases := []struct {
		path   string
		branch string
		want   string
	}{
		{"/Users/ragu/clarity-cli", "", "clarity-cli"},
		{"/Users/ragu/clarity-cli", "refs/heads/main", "clarity-cli"},
		{"/tmp/foo-feat", "refs/heads/feat/foo", "foo-feat"},
		{"/", "", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, repoLabel(c.path, c.branch), "path=%q branch=%q", c.path, c.branch)
	}
}
