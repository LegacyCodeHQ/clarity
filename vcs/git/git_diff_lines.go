package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// GetUncommittedFileDiffs returns per-line additions and deletions for files
// with uncommitted changes (staged + unstaged). Line numbers in Additions
// reference the working-tree (post-image); line numbers in Deletions
// reference HEAD (pre-image). Both are 1-indexed.
func GetUncommittedFileDiffs(repoPath string) (map[string]vcs.FileDiff, error) {
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
	}
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository", repoPath)
	}
	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	stdout, stderr, err := runGitCommand(repoPath, "diff", "--unified=0", "--no-color", "HEAD")
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}

	diffs := parseUnifiedDiff(string(stdout))

	// Resolve relative paths to absolute, and account for untracked files which
	// `git diff HEAD` does not surface.
	out := make(map[string]vcs.FileDiff, len(diffs))
	for relPath, fd := range diffs {
		out[filepath.Join(repoRoot, relPath)] = fd
	}

	statusMap, err := getUncommittedFileStatuses(repoPath)
	if err != nil {
		return nil, err
	}
	for relPath, status := range statusMap {
		if !isNewStatus(status) {
			continue
		}
		absPath := filepath.Join(repoRoot, relPath)
		if _, ok := out[absPath]; ok {
			continue
		}
		lineCount, err := countLinesInFile(absPath)
		if err != nil {
			continue
		}
		adds := make([]int, lineCount)
		for i := 0; i < lineCount; i++ {
			adds[i] = i + 1
		}
		out[absPath] = vcs.FileDiff{Additions: adds, IsNew: true}
	}

	return out, nil
}

// GetCommitFileDiffs returns per-line additions and deletions for files
// modified in a single commit, computed against that commit's first parent.
// Line numbers in Additions reference the commit's post-image; line numbers
// in Deletions reference the parent's content.
func GetCommitFileDiffs(repoPath, commitID string) (map[string]vcs.FileDiff, error) {
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository", repoPath)
	}
	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	// `git diff <commit>~ <commit>` is undefined for the root commit; fall back
	// to `--root <commit>` which diffs against the empty tree.
	stdout, _, err := runGitCommand(repoPath, "diff", "--unified=0", "--no-color", commitID+"~", commitID)
	if err != nil {
		// Root commit has no parent; fall back to diff-tree --root which emits
		// a synthetic diff against the empty tree.
		var stderr string
		stdout, stderr, err = runGitCommand(repoPath, "diff-tree", "--root", "--unified=0", "--no-color", "-p", commitID)
		if err != nil {
			return nil, gitCommandError(err, stderr)
		}
	}
	return absolutize(repoRoot, parseUnifiedDiff(string(stdout))), nil
}

// GetCommitRangeFileDiffs returns per-line additions and deletions for files
// changed between two commits. Inputs are the same as `git diff from..to`.
func GetCommitRangeFileDiffs(repoPath, fromCommit, toCommit string) (map[string]vcs.FileDiff, error) {
	if !isGitRepository(repoPath) {
		return nil, fmt.Errorf("%s is not a git repository", repoPath)
	}
	repoRoot, err := GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	stdout, stderr, err := runGitCommand(repoPath, "diff", "--unified=0", "--no-color", fromCommit, toCommit)
	if err != nil {
		return nil, gitCommandError(err, stderr)
	}
	return absolutize(repoRoot, parseUnifiedDiff(string(stdout))), nil
}

func absolutize(repoRoot string, diffs map[string]vcs.FileDiff) map[string]vcs.FileDiff {
	out := make(map[string]vcs.FileDiff, len(diffs))
	for relPath, fd := range diffs {
		out[filepath.Join(repoRoot, relPath)] = fd
	}
	return out
}

// parseUnifiedDiff walks `git diff --unified=0` output and emits per-file
// FileDiff structures with 1-indexed line numbers.
func parseUnifiedDiff(diff string) map[string]vcs.FileDiff {
	out := make(map[string]vcs.FileDiff)
	var currentPath string
	var current vcs.FileDiff
	var oldLine, newLine int

	flush := func() {
		if currentPath == "" {
			return
		}
		out[currentPath] = current
	}

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			currentPath = ""
			current = vcs.FileDiff{}
			// Path is the `b/...` side; pick it from the trailing token.
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				currentPath = strings.TrimPrefix(fields[3], "b/")
			}
		case strings.HasPrefix(line, "new file mode"):
			current.IsNew = true
		case strings.HasPrefix(line, "deleted file mode"):
			current.IsDeleted = true
		case strings.HasPrefix(line, "rename from"), strings.HasPrefix(line, "rename to"):
			current.IsRenamed = true
		case strings.HasPrefix(line, "@@"):
			oldStart, _, newStart, _, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			oldLine = oldStart
			newLine = newStart
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			// File headers; ignore.
		case strings.HasPrefix(line, "+"):
			current.Additions = append(current.Additions, newLine)
			newLine++
		case strings.HasPrefix(line, "-"):
			current.Deletions = append(current.Deletions, oldLine)
			oldLine++
		}
	}
	flush()
	return out
}

// parseHunkHeader parses a unified-diff hunk header of the form
//
//	@@ -<oldStart>[,<oldCount>] +<newStart>[,<newCount>] @@
//
// returning the four line numbers. When count is omitted it defaults to 1.
func parseHunkHeader(header string) (oldStart, oldCount, newStart, newCount int, ok bool) {
	if !strings.HasPrefix(header, "@@") {
		return 0, 0, 0, 0, false
	}
	body := strings.TrimPrefix(header, "@@")
	body = strings.TrimSpace(body)
	end := strings.Index(body, "@@")
	if end == -1 {
		return 0, 0, 0, 0, false
	}
	parts := strings.Fields(strings.TrimSpace(body[:end]))
	if len(parts) < 2 {
		return 0, 0, 0, 0, false
	}
	oldStart, oldCount, ok = parseHunkSide(parts[0], '-')
	if !ok {
		return 0, 0, 0, 0, false
	}
	newStart, newCount, ok = parseHunkSide(parts[1], '+')
	if !ok {
		return 0, 0, 0, 0, false
	}
	return oldStart, oldCount, newStart, newCount, true
}

func parseHunkSide(s string, sign byte) (start, count int, ok bool) {
	if len(s) == 0 || s[0] != sign {
		return 0, 0, false
	}
	body := s[1:]
	commaIdx := strings.IndexByte(body, ',')
	if commaIdx == -1 {
		n, err := strconv.Atoi(body)
		if err != nil {
			return 0, 0, false
		}
		return n, 1, true
	}
	start, err := strconv.Atoi(body[:commaIdx])
	if err != nil {
		return 0, 0, false
	}
	count, err = strconv.Atoi(body[commaIdx+1:])
	if err != nil {
		return 0, 0, false
	}
	return start, count, true
}
