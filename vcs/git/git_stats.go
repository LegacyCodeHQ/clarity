package git

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// GetUncommittedFileStats returns statistics (additions/deletions) for uncommitted files
// Returns a map from relative file paths to their FileStats
func GetUncommittedFileStats(repoPath string) (map[string]vcs.FileStats, error) {
	// Validate the repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Verify it's a git repository
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository", repoPath)
	}

	// Get the repository root
	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	// Run git diff --numstat to get stats for uncommitted changes
	// This includes both staged and unstaged changes
	stdout, stderr, err := runGitCommand(repoPath, "diff", "--numstat", "HEAD")
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	statusMap, err := getUncommittedFileStatuses(repoPath)
	if err != nil {
		return nil, err
	}

	// Parse the numstat output
	stats := make(map[string]vcs.FileStats)
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: additions	deletions	filename
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		additions := 0
		deletions := 0

		// Parse additions (may be "-" for binary files)
		if parts[0] != "-" {
			additions, _ = strconv.Atoi(parts[0])
		}

		// Parse deletions (may be "-" for binary files)
		if parts[1] != "-" {
			deletions, _ = strconv.Atoi(parts[1])
		}

		// Handle renamed files
		filePath := strings.Join(parts[2:], " ")
		filePath = parseRenamedFilePath(filePath)
		filePath = filepath.Clean(filePath)

		// Convert to absolute path
		absPath := filepath.Join(repoRoot, filePath)

		stats[absPath] = vcs.FileStats{
			Additions: additions,
			Deletions: deletions,
			IsNew:     isNewStatus(statusMap[filePath]),
		}
	}

	// Include entries for new/untracked files that may not appear in numstat output
	for relPath, status := range statusMap {
		if !isNewStatus(status) {
			continue
		}

		absPath := filepath.Join(repoRoot, relPath)
		fileStats := stats[absPath]
		fileStats.IsNew = true
		if fileStats.Additions == 0 && fileStats.Deletions == 0 {
			lineCount, err := countLinesInFile(absPath)
			if err == nil {
				fileStats.Additions = lineCount
			}
		}
		stats[absPath] = fileStats
	}

	return stats, nil
}

// GetCommitFileStats returns statistics (additions/deletions) for files in a specific commit
// Returns a map from absolute file paths to their FileStats
func GetCommitFileStats(repoPath, commitID string) (map[string]vcs.FileStats, error) {
	// Validate the repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Verify it's a git repository
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository", repoPath)
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

	// Run git show --numstat to get stats for the commit
	// Use --root flag to handle root commits
	stdout, stderr, err := runGitCommand(repoPath, "show", "--numstat", "--format=", commitID)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	statusMap, err := getCommitFileStatuses(repoPath, commitID)
	if err != nil {
		return nil, err
	}

	// Parse the numstat output
	stats := make(map[string]vcs.FileStats)
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: additions	deletions	filename
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		additions := 0
		deletions := 0

		// Parse additions (may be "-" for binary files)
		if parts[0] != "-" {
			additions, _ = strconv.Atoi(parts[0])
		}

		// Parse deletions (may be "-" for binary files)
		if parts[1] != "-" {
			deletions, _ = strconv.Atoi(parts[1])
		}

		// Handle renamed files
		filePath := strings.Join(parts[2:], " ")
		filePath = parseRenamedFilePath(filePath)
		filePath = filepath.Clean(filePath)

		// Convert to absolute path
		absPath := filepath.Join(repoRoot, filePath)

		stats[absPath] = vcs.FileStats{
			Additions: additions,
			Deletions: deletions,
			IsNew:     isNewStatus(statusMap[filePath]),
		}
	}

	// Include entries for new files that may not appear in numstat output
	for relPath, status := range statusMap {
		if !isNewStatus(status) {
			continue
		}

		absPath := filepath.Join(repoRoot, relPath)
		fileStats := stats[absPath]
		fileStats.IsNew = true
		stats[absPath] = fileStats
	}

	return stats, nil
}

// GetCommitRangeFileStats returns statistics (additions/deletions) for files changed between two commits.
// Returns a map from absolute file paths to their FileStats.
func GetCommitRangeFileStats(repoPath, fromCommit, toCommit string) (map[string]vcs.FileStats, error) {
	// Validate the repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Verify it's a git repository
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository", repoPath)
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

	// Run git diff --numstat to get stats for the range
	stdout, stderr, err := runGitCommand(repoPath, "diff", "--numstat", fromCommit, toCommit)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	// Get file statuses to determine if files are new
	statusMap, err := getCommitRangeFileStatuses(repoPath, fromCommit, toCommit)
	if err != nil {
		return nil, err
	}

	// Parse the numstat output
	stats := make(map[string]vcs.FileStats)
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: additions	deletions	filename
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		additions := 0
		deletions := 0

		// Parse additions (may be "-" for binary files)
		if parts[0] != "-" {
			additions, _ = strconv.Atoi(parts[0])
		}

		// Parse deletions (may be "-" for binary files)
		if parts[1] != "-" {
			deletions, _ = strconv.Atoi(parts[1])
		}

		// Handle renamed files
		filePath := strings.Join(parts[2:], " ")
		filePath = parseRenamedFilePath(filePath)
		filePath = filepath.Clean(filePath)

		// Convert to absolute path
		absPath := filepath.Join(repoRoot, filePath)

		stats[absPath] = vcs.FileStats{
			Additions: additions,
			Deletions: deletions,
			IsNew:     statusMap[filePath] == "A",
		}
	}

	return stats, nil
}

// getUncommittedFileStatuses returns a map of relative file paths to their git status codes
func getUncommittedFileStatuses(repoPath string) (map[string]string, error) {
	stdout, stderr, err := runGitCommand(repoPath, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	statuses := make(map[string]string)
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}

		status := line[:2]
		filePath := strings.TrimSpace(line[3:])

		// Handle renamed files (format: "old -> new")
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			filePath = parts[1]
		}

		if filePath == "" {
			continue
		}

		normalized := filepath.Clean(filePath)
		statuses[normalized] = status
	}

	return statuses, nil
}

// getCommitFileStatuses returns a map of file paths to their status codes for a commit
func getCommitFileStatuses(repoPath, commitID string) (map[string]string, error) {
	stdout, stderr, err := runGitCommand(repoPath, "show", "--name-status", "--format=", commitID)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	statuses := make(map[string]string)
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		var filePath string
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if len(parts) < 3 {
				continue
			}
			filePath = parts[2]
		} else {
			filePath = parts[1]
		}

		if filePath == "" {
			continue
		}

		normalized := filepath.Clean(filePath)
		statuses[normalized] = status
	}

	return statuses, nil
}

// getCommitRangeFileStatuses returns a map of file paths to their status codes for a commit range
func getCommitRangeFileStatuses(repoPath, fromCommit, toCommit string) (map[string]string, error) {
	stdout, stderr, err := runGitCommand(repoPath, "diff", "--name-status", fromCommit, toCommit)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	statuses := make(map[string]string)
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		status := parts[0]
		var filePath string
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if len(parts) < 3 {
				continue
			}
			filePath = parts[2]
		} else {
			filePath = parts[1]
		}

		if filePath == "" {
			continue
		}

		normalized := filepath.Clean(filePath)
		statuses[normalized] = status
	}

	return statuses, nil
}

// parseRenamedFilePath parses a renamed file path from git numstat output
// and returns the new (destination) file path.
// Handles two formats:
// 1. Full format: "old_path => new_path" (returns new_path)
// 2. Abbreviated format: "prefix/{old => new}/suffix" (returns prefix/new/suffix)
func parseRenamedFilePath(filePath string) string {
	// Check for abbreviated rename format: "prefix/{ => new}/suffix" or "prefix/{old => new}/suffix"
	if strings.Contains(filePath, "{") && strings.Contains(filePath, "}") {
		// Find the positions of { and }
		openBrace := strings.Index(filePath, "{")
		closeBrace := strings.Index(filePath, "}")

		if openBrace < closeBrace {
			// Extract parts
			prefix := filePath[:openBrace]
			middle := filePath[openBrace+1 : closeBrace]
			suffix := filePath[closeBrace+1:]

			// Split the middle part on " => "
			if strings.Contains(middle, " => ") {
				parts := strings.Split(middle, " => ")
				if len(parts) == 2 {
					// Use the new (right) part
					newMiddle := strings.TrimSpace(parts[1])
					return prefix + newMiddle + suffix
				}
			}
		}
	}

	// Check for full rename format: "old => new"
	if strings.Contains(filePath, " => ") {
		renameParts := strings.Split(filePath, " => ")
		if len(renameParts) == 2 {
			return strings.TrimSpace(renameParts[1])
		}
	}

	// Not a rename, return as-is
	return filePath
}

// isNewStatus determines if a git status code represents a new or untracked file
func isNewStatus(status string) bool {
	status = strings.TrimSpace(status)
	if status == "" {
		return false
	}

	if status == "??" {
		return true
	}

	return status[0] == 'A'
}

func countLinesInFile(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(content) == 0 {
		return 0, nil
	}

	lines := bytes.Count(content, []byte{'\n'})
	if content[len(content)-1] != '\n' {
		lines++
	}
	return lines, nil
}
