package git

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GetUncommittedFiles finds all uncommitted files in a git repository.
// Returns absolute paths to all uncommitted files (staged, unstaged, and untracked).
func GetUncommittedFiles(repoPath string) ([]string, error) {
	// Validate the repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Verify it's a git repository
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository (use 'git init' to initialize)", repoPath)
	}

	// Get the repository root
	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	// Get all uncommitted files
	uncommittedFiles, err := getUncommittedFiles(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get uncommitted files: %w", err)
	}

	// Convert to absolute paths (no filtering - include all files)
	absolutePaths := toAbsolutePaths(repoRoot, uncommittedFiles)

	return absolutePaths, nil
}

// GetUncommittedDeletedFiles returns absolute paths for tracked files deleted
// from the current working tree or index.
func GetUncommittedDeletedFiles(repoPath string) ([]string, error) {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository (use 'git init' to initialize)", repoPath)
	}

	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	statusMap, err := getUncommittedFileStatuses(repoPath)
	if err != nil {
		return nil, err
	}

	var deleted []string
	for relPath, status := range statusMap {
		if len(status) < 2 {
			continue
		}
		if status[0] == 'D' || status[1] == 'D' {
			deleted = append(deleted, filepath.Join(repoRoot, relPath))
		}
	}
	sort.Strings(deleted)

	return deleted, nil
}

// getUncommittedFiles returns a list of all uncommitted files (relative to repo root)
func getUncommittedFiles(repoPath string) ([]string, error) {
	stdout, stderr, err := runGitCommand(repoPath, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		// Check if git is not installed
		if strings.Contains(stderr, "not found") || strings.Contains(stderr, "not recognized") {
			return nil, fmt.Errorf("git command not found - please install Git to use the --repo flag")
		}
		return nil, gitCommandError(err, stderr)
	}

	// Parse the porcelain output
	var files []string
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		// Porcelain format: XY filename
		// X = status in index, Y = status in working tree
		// Skip deleted files (D in either position) as they don't exist on the filesystem
		statusX := line[0]
		statusY := line[1]
		if statusX == 'D' || statusY == 'D' {
			continue
		}

		filePath := strings.TrimSpace(line[3:])

		// Handle renamed files (format: "old -> new")
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			filePath = parts[1] // Use the new filename
		}

		if filePath != "" {
			files = append(files, filePath)
		}
	}

	return files, nil
}

// GetCommitDartFiles finds all files that were changed in a specific commit.
// Returns absolute paths to all files added, modified, or renamed in the commit.
func GetCommitDartFiles(repoPath, commitID string) ([]string, error) {
	// Validate the repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Verify it's a git repository
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository (use 'git init' to initialize)", repoPath)
	}

	// Validate the commit exists
	if err := validateCommit(repoPath, commitID); err != nil {
		return nil, err
	}

	// Get the repository root
	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	// Get files changed in the commit
	commitFiles, err := getCommitFiles(repoPath, commitID)
	if err != nil {
		return nil, fmt.Errorf("failed to get files from commit: %w", err)
	}

	// Convert to absolute paths (no filtering - include all files)
	absolutePaths := toAbsolutePaths(repoRoot, commitFiles)

	return absolutePaths, nil
}

// getCommitFiles returns a list of all files changed in the specified commit (relative to repo root)
func getCommitFiles(repoPath, commitID string) ([]string, error) {
	// Use --root flag to handle root commits (first commit in repo)
	// Use --diff-filter=d to exclude deleted files (only include added, modified, and renamed files)
	stdout, stderr, err := runGitCommand(repoPath, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", "--diff-filter=d", commitID)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	// Parse the output - one file per line
	var files []string
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// GetFileContentFromCommit reads the content of a file at a specific commit
// using 'git show commit:path'. The filePath should be relative to the repository root.
func GetFileContentFromCommit(repoPath, commitID, filePath string) ([]byte, error) {
	if err := validateGitRef(commitID); err != nil {
		return nil, err
	}
	if err := validateGitRelPath(filePath); err != nil {
		return nil, err
	}

	// Format: commit:path
	ref := fmt.Sprintf("%s:%s", commitID, filePath)

	stdout, stderr, err := runGitCommand(repoPath, "show", ref)
	if err != nil {
		if stderr != "" {
			return nil, fmt.Errorf("git show failed: %s", stderr)
		}
		return nil, err
	}

	return stdout, nil
}

// GetCommitRangeFiles finds all files changed between two commits.
// Uses: git diff --name-only --diff-filter=d <from> <to>
// Returns absolute paths to all files added, modified, or renamed between the commits.
func GetCommitRangeFiles(repoPath, fromCommit, toCommit string) ([]string, error) {
	// Validate the repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Verify it's a git repository
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository (use 'git init' to initialize)", repoPath)
	}

	// Validate both commits exist
	if err := validateCommit(repoPath, fromCommit); err != nil {
		return nil, err
	}
	if err := validateCommit(repoPath, toCommit); err != nil {
		return nil, err
	}

	// Get the repository root
	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	// Get files changed between the two commits
	// --diff-filter=d excludes deleted files (only include added, modified, and renamed files)
	stdout, stderr, err := runGitCommand(repoPath, "diff", "--name-only", "--diff-filter=d", fromCommit, toCommit)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	// Parse the output - one file per line
	var files []string
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	// Convert to absolute paths
	absolutePaths := toAbsolutePaths(repoRoot, files)

	return absolutePaths, nil
}
