package show

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
)

// existingFiles keeps only paths that exist as regular files, dropping declared
// module members whose paths no longer resolve (matching how a mistyped pattern
// surfaces as zero files).
func existingFiles(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			out = append(out, p)
		}
	}
	return out
}

func existingSnapshotFiles(paths []string, snapshotFiles []string) []string {
	if snapshotFiles == nil {
		return existingFiles(paths)
	}

	present := make(map[string]bool, len(snapshotFiles))
	for _, file := range snapshotFiles {
		present[filepath.Clean(file)] = true
	}

	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if present[filepath.Clean(p)] {
			out = append(out, p)
		}
	}
	return out
}

// expandPaths expands file paths and directories into individual file paths.
// Directories are recursively walked and regular files are included based on includeUnsupportedFiles.
// For directories inside a git repository, git ls-files is used to respect .gitignore rules.
func expandPaths(paths []string, includeUnsupportedFiles bool) ([]string, error) {
	var result []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", path, err)
		}

		if info.IsDir() {
			files, err := listGitFiles(path)
			if err != nil {
				// Not a git repo or git not available; fall back to walk
				files, err = walkDirectoryFiles(path)
				if err != nil {
					return nil, fmt.Errorf("failed to walk directory %s: %w", path, err)
				}
			}

			for _, f := range files {
				if includeUnsupportedFiles {
					result = append(result, f)
					continue
				}
				ext := filepath.Ext(f)
				if registry.IsSupportedLanguageExtension(ext) {
					result = append(result, f)
				}
			}
		} else {
			// Regular file - include it directly
			result = append(result, path)
		}
	}

	return result, nil
}

// listGitFiles returns absolute paths for all non-ignored files in a git repository,
// including files inside submodules. It combines tracked files (--recurse-submodules)
// with untracked but non-ignored files (--others --exclude-standard).
func listGitFiles(dir string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	tracked, err := gitLsFiles(absDir, "--cached", "--recurse-submodules")
	if err != nil {
		return nil, err
	}

	untracked, err := gitLsFiles(absDir, "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(tracked)+len(untracked))
	for _, rel := range append(tracked, untracked...) {
		result = append(result, filepath.Join(absDir, rel))
	}
	return result, nil
}

func gitLsFiles(dir string, args ...string) ([]string, error) {
	cmdArgs := append([]string{"ls-files", "-z"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, part := range strings.Split(string(out), "\x00") {
		p := strings.TrimSpace(part)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

var walkSkippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	".dart_tool":   true,
	"build":        true,
	"__pycache__":  true,
	".gradle":      true,
	".idea":        true,
	".vscode":      true,
}

// walkDirectoryFiles is a fallback for non-git directories.
func walkDirectoryFiles(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fileInfo.IsDir() {
			if walkSkippedDirs[fileInfo.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		result = append(result, filePath)
		return nil
	})
	return result, err
}
