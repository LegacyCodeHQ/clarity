package git

import (
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// GitCommitContentReader returns a ContentReader that reads file content from a specific git commit.
func GitCommitContentReader(repoPath, commitID string) vcs.ContentReader {
	return func(absPath string) ([]byte, error) {
		relPath := getRelativePath(absPath, repoPath)
		return GetFileContentFromCommit(repoPath, commitID, relPath)
	}
}

// GetRelativePath converts an absolute file path to a path relative to the repository root
func getRelativePath(absPath, repoPath string) string {
	// Get absolute repository path
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		// If we can't get absolute path, try relative path as-is
		relPath, err := filepath.Rel(repoPath, absPath)
		if err != nil {
			// Fallback to using the absolute path
			return absPath
		}
		return relPath
	}

	// Normalize symlinks for temp directories like /var -> /private/var.
	resolvedRepoPath := resolveSymlinks(absRepoPath)
	resolvedAbsPath := resolveSymlinks(absPath)
	if relPath, err := filepath.Rel(resolvedRepoPath, resolvedAbsPath); err == nil {
		return relPath
	}

	// Get path relative to repository root
	relPath, err := filepath.Rel(absRepoPath, absPath)
	if err != nil {
		// Fallback to using the absolute path
		return absPath
	}

	return relPath
}

func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
