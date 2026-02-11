package show

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RawPath is a user-provided file path from CLI flags.
type RawPath string

// AbsolutePath is a normalized absolute filesystem path.
type AbsolutePath string

func (p AbsolutePath) String() string {
	return string(p)
}

// PathResolver resolves raw user paths relative to a configured base directory.
type PathResolver struct {
	baseDir      AbsolutePath
	allowOutside bool
}

func (r PathResolver) BaseDir() string {
	return r.baseDir.String()
}

func NewPathResolver(baseDir string, allowOutside bool) (PathResolver, error) {
	if baseDir == "" {
		baseDir = "."
	}

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return PathResolver{}, fmt.Errorf("failed to resolve base path: %w", err)
	}

	absBaseDir = resolveSymlinks(absBaseDir)
	return PathResolver{
		baseDir:      AbsolutePath(filepath.Clean(absBaseDir)),
		allowOutside: allowOutside,
	}, nil
}

func (r PathResolver) Resolve(path RawPath) (AbsolutePath, error) {
	pathStr := string(path)
	if pathStr == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	if filepath.IsAbs(pathStr) {
		absPath := filepath.Clean(pathStr)
		if !r.allowOutside {
			within, err := isWithinBase(r.baseDir.String(), absPath)
			if err != nil {
				return "", err
			}
			if !within {
				return "", fmt.Errorf("path must be within repository: %q", pathStr)
			}
		}
		return AbsolutePath(absPath), nil
	}

	absPath := filepath.Clean(filepath.Join(r.baseDir.String(), pathStr))
	if !r.allowOutside {
		within, err := isWithinBase(r.baseDir.String(), absPath)
		if err != nil {
			return "", err
		}
		if !within {
			return "", fmt.Errorf("path must be within repository: %q", pathStr)
		}
	}
	return AbsolutePath(absPath), nil
}

func isWithinBase(baseDir, targetPath string) (bool, error) {
	baseDir = resolveSymlinks(filepath.Clean(baseDir))
	targetPath = resolveSymlinks(filepath.Clean(targetPath))

	rel, err := filepath.Rel(baseDir, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate path %q: %w", targetPath, err)
	}
	if rel == "." {
		return true, nil
	}
	if rel == ".." {
		return false, nil
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return !filepath.IsAbs(rel), nil
}

func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
