package git

import (
	"path/filepath"
	"strings"
)

// Worktree describes a single working tree registered with a repository.
type Worktree struct {
	// Path is the absolute path to the working tree on disk.
	Path string
	// Head is the SHA at HEAD, or empty for an unborn branch.
	Head string
	// Branch is the full ref name (e.g. "refs/heads/main"), empty for detached.
	Branch string
	// IsPrimary is true for the primary worktree of the repository.
	// Per git's `worktree list --porcelain` contract, the primary is always first.
	IsPrimary bool
}

// GetGitDir returns the absolute path to the git directory for `path`.
// For the primary worktree this equals GetCommonDir; for a linked worktree
// this is the per-worktree gitdir under `<common>/worktrees/<name>`.
func GetGitDir(path string) (string, error) {
	stdout, stderr, err := runGitCommand(path, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", gitCommandError(err, stderr)
	}
	return strings.TrimSpace(string(stdout)), nil
}

// GetCommonDir returns the absolute path to the common git directory — the
// primary repository's `.git`. Identical for the primary and every linked
// worktree of the same repository.
func GetCommonDir(path string) (string, error) {
	stdout, stderr, err := runGitCommand(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", gitCommandError(err, stderr)
	}
	return strings.TrimSpace(string(stdout)), nil
}

// IsPrimaryWorktree reports whether `path` is inside the primary worktree of
// its repository. True iff the git dir equals the common dir.
func IsPrimaryWorktree(path string) (bool, error) {
	gitDir, err := GetGitDir(path)
	if err != nil {
		return false, err
	}
	commonDir, err := GetCommonDir(path)
	if err != nil {
		return false, err
	}
	return resolveSymlinks(gitDir) == resolveSymlinks(commonDir), nil
}

// ListWorktrees enumerates all worktrees registered with the repository
// containing `path`. The primary worktree is always the first entry.
func ListWorktrees(path string) ([]Worktree, error) {
	stdout, stderr, err := runGitCommand(path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}
	return parseWorktreePorcelain(string(stdout)), nil
}

// parseWorktreePorcelain parses the output of `git worktree list --porcelain`.
// Stanzas are separated by blank lines. Each stanza begins with `worktree <path>`,
// optionally followed by `HEAD <sha>` and either `branch <ref>`, `detached`, or
// `bare`. The first stanza describes the primary worktree.
func parseWorktreePorcelain(out string) []Worktree {
	var (
		result   []Worktree
		current  Worktree
		hasEntry bool
	)
	flush := func() {
		if hasEntry {
			current.IsPrimary = len(result) == 0
			result = append(result, current)
		}
		current = Worktree{}
		hasEntry = false
	}
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(trimmed, " ")
		switch key {
		case "worktree":
			flush()
			current.Path = filepath.Clean(value)
			hasEntry = true
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = value
		case "detached", "bare", "locked":
			// no-op for our purposes
		}
	}
	flush()
	return result
}
