package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
)

func buildDOTGraph(repoPath string, opts *watchOptions, formatter formatters.Formatter) (string, error) {
	filePaths, err := git.GetUncommittedFiles(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get uncommitted files: %w", err)
	}
	deletedFiles, err := git.GetUncommittedDeletedFiles(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get deleted uncommitted files: %w", err)
	}
	deletedContent, err := loadDeletedFileContent(repoPath, deletedFiles)
	if err != nil {
		return "", err
	}
	filePaths = append(filePaths, deletedFiles...)

	if len(filePaths) == 0 {
		return "", errNoUncommittedChanges
	}

	filePaths, err = applyWatchExtensionFilters(opts, filePaths)
	if err != nil {
		return "", err
	}

	if len(opts.excludes) > 0 {
		filePaths, err = applyWatchExcludeFilter(opts, filePaths)
		if err != nil {
			return "", err
		}
	}

	contentReader := deletedAwareContentReader(deletedContent)

	graph, err := depgraph.BuildDependencyGraph(filePaths, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build dependency graph: %w", err)
	}

	fileStats, _ := git.GetUncommittedFileStats(repoPath)

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, fileStats, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build file graph metadata: %w", err)
	}
	depgraph.MarkDeletedFiles(&fileGraph, deletedFiles)

	if !opts.noPhantom {
		if diffs, diffErr := git.GetUncommittedFileDiffs(repoPath); diffErr == nil {
			depgraph.AnnotateRustPhantomsWatch(&fileGraph, diffs, git.GitCommitContentReader(repoPath, "HEAD"), contentReader)
		}
	}

	direction, ok := formatters.ParseDirection(opts.direction)
	if !ok {
		direction = formatters.DefaultDirection
	}
	renderOpts := formatters.RenderOptions{
		Direction: direction,
		BasePath:  repoPath,
	}

	return formatter.Format(fileGraph, renderOpts)
}

var errNoUncommittedChanges = fmt.Errorf("no uncommitted changes")

func loadDeletedFileContent(repoPath string, deletedFiles []string) (map[string][]byte, error) {
	if len(deletedFiles) == 0 {
		return nil, nil
	}

	repoRoot, err := git.GetRepositoryRoot(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository root: %w", err)
	}

	content := make(map[string][]byte, len(deletedFiles))
	for _, absPath := range deletedFiles {
		relPath, err := filepath.Rel(repoRoot, absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve deleted file %s relative to repository root: %w", absPath, err)
		}
		bytes, err := git.GetFileContentFromCommit(repoPath, "HEAD", relPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read deleted file %s from HEAD: %w", relPath, err)
		}
		content[absPath] = bytes
	}

	return content, nil
}

func deletedAwareContentReader(deletedContent map[string][]byte) vcs.ContentReader {
	filesystemReader := vcs.FilesystemContentReader()
	return func(absPath string) ([]byte, error) {
		if content, ok := deletedContent[absPath]; ok {
			return content, nil
		}
		content, err := filesystemReader(absPath)
		if err == nil || !os.IsNotExist(err) {
			return content, err
		}
		if resolved, resolveErr := filepath.EvalSymlinks(absPath); resolveErr == nil {
			if content, ok := deletedContent[resolved]; ok {
				return content, nil
			}
		}
		return content, err
	}
}

func applyWatchExtensionFilters(opts *watchOptions, filePaths []string) ([]string, error) {
	if opts.includeExt != "" {
		exts := parseExtensions(opts.includeExt)
		filtered := make([]string, 0, len(filePaths))
		for _, fp := range filePaths {
			if exts[strings.ToLower(filepath.Ext(fp))] {
				filtered = append(filtered, fp)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("no files remain after applying --include-ext %q", opts.includeExt)
		}
		filePaths = filtered
	}

	if opts.excludeExt != "" {
		exts := parseExtensions(opts.excludeExt)
		filtered := make([]string, 0, len(filePaths))
		for _, fp := range filePaths {
			if !exts[strings.ToLower(filepath.Ext(fp))] {
				filtered = append(filtered, fp)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("no files remain after applying --exclude-ext %q", opts.excludeExt)
		}
		filePaths = filtered
	}

	return filePaths, nil
}

func applyWatchExcludeFilter(opts *watchOptions, filePaths []string) ([]string, error) {
	excludePaths := make([]string, 0, len(opts.excludes))
	for _, exclude := range opts.excludes {
		absExclude, err := filepath.Abs(exclude)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve exclude path %q: %w", exclude, err)
		}
		excludePaths = append(excludePaths, absExclude)
	}

	filtered := make([]string, 0, len(filePaths))
	for _, fp := range filePaths {
		excluded := false
		for _, ep := range excludePaths {
			if fp == ep || strings.HasPrefix(fp, ep+string(filepath.Separator)) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, fp)
		}
	}

	return filtered, nil
}

func parseExtensions(raw string) map[string]bool {
	exts := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		ext := strings.TrimSpace(part)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts[strings.ToLower(ext)] = true
	}
	return exts
}
