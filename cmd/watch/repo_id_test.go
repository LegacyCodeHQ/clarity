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

func TestPrimaryRepoLabel(t *testing.T) {
	cases := []struct {
		branch string
		want   string
	}{
		{"", "clarity-cli"},
		{"refs/heads/main", "main"},
		{"refs/heads/feat/foo", "feat/foo"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, primaryRepoLabel("/Users/ragu/clarity-cli", c.branch), "branch=%q", c.branch)
	}
}

func TestLinkedRepoLabel(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/Users/ragu/clarity-cli", "clarity-cli"},
		{"/tmp/foo-feat", "foo-feat"},
		{"/", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, linkedRepoLabel(c.path), "path=%q", c.path)
	}
}
