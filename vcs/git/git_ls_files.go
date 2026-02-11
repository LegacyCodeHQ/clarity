package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListTrackedFiles returns absolute paths for files tracked in the git index.
func ListTrackedFiles(repoPath string) ([]string, error) {
	repoRoot, err := ensureRepoRoot(repoPath)
	if err != nil {
		return nil, err
	}

	stdout, stderr, err := runGitCommand(repoPath, "ls-files", "-z", "--cached")
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	return toAbsolutePaths(repoRoot, parseNullSeparatedPaths(stdout)), nil
}

// ListUntrackedFiles returns absolute paths for non-ignored untracked files.
func ListUntrackedFiles(repoPath string) ([]string, error) {
	repoRoot, err := ensureRepoRoot(repoPath)
	if err != nil {
		return nil, err
	}

	stdout, stderr, err := runGitCommand(repoPath, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	return toAbsolutePaths(repoRoot, parseNullSeparatedPaths(stdout)), nil
}

// ResolveFirstParent resolves the first parent of a commit.
// Returns hasParent=false for root commits.
func ResolveFirstParent(repoPath, commitID string) (parent string, hasParent bool, err error) {
	if err := validateCommit(repoPath, commitID); err != nil {
		return "", false, err
	}

	stdout, stderr, err := runGitCommand(repoPath, "rev-list", "--parents", "-n", "1", commitID)
	if err != nil {
		return "", false, gitCommandError(err, stderr)
	}

	fields := strings.Fields(strings.TrimSpace(string(stdout)))
	if len(fields) < 1 {
		return "", false, fmt.Errorf("unexpected git rev-list output for commit %s", commitID)
	}
	if len(fields) == 1 {
		return "", false, nil
	}

	return fields[1], true, nil
}

func ensureRepoRoot(repoPath string) (string, error) {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("repository path does not exist: %s", repoPath)
	}
	if !isGitRepository(repoPath) {
		return "", fmt.Errorf("%s is not a git repository (use 'git init' to initialize)", repoPath)
	}

	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %w", err)
	}
	return filepath.Clean(repoRoot), nil
}

func parseNullSeparatedPaths(stdout []byte) []string {
	parts := strings.Split(string(stdout), "\x00")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		path := strings.TrimSpace(part)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// FileExists checks whether a file exists on disk.
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to stat %s: %w", path, err)
}
