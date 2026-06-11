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

func buildGraph(repoPath string, opts *watchOptions, formatter formatters.Formatter) (string, error) {
	// Resolve symlinks so the render base path matches the (symlink-resolved)
	// file node paths. Otherwise relative-path shortening fails (e.g. macOS
	// /var vs /private/var) and every node renders with an absolute id.
	if resolved, err := filepath.EvalSymlinks(repoPath); err == nil {
		repoPath = resolved
	}

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
	// Reflect git's own rename detection. A staged rename is reported as status R
	// (git has matched old to new, edits included), so we collapse it onto the new
	// path. An unstaged move is a deletion plus an untracked add — git has not
	// called it a rename, so neither do we; it stays delete + create. This keeps
	// the graph honest to `git status` instead of guessing via content hashes.
	gitRenames, err := git.GetUncommittedRenames(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get renamed uncommitted files: %w", err)
	}
	renameOldContent, err := loadDeletedFileContent(repoPath, renameSources(gitRenames))
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

	// Collapse git's staged renames onto a single new-path node, dropping the old
	// node. Unstaged moves are not in this map, so they stay delete + create.
	renames := gitRenames
	graph, err = depgraph.CollapseRenames(graph, renames)
	if err != nil {
		return "", err
	}
	pureDeleted := make([]string, 0, len(deletedFiles))
	for _, d := range deletedFiles {
		if _, renamed := renames[d]; !renamed {
			pureDeleted = append(pureDeleted, d)
		}
	}

	// Reconstruct each deleted file's pre-deletion edges from HEAD so removed
	// nodes show their old links instead of floating; MarkDeletedFiles styles
	// them as removed edges below.
	if len(pureDeleted) > 0 {
		parentReader := git.GitCommitContentReader(repoPath, "HEAD")
		graph, err = depgraph.MergeDeletedNeighborhood(graph, filePaths, pureDeleted, parentReader)
		if err != nil {
			return "", err
		}
	}

	fileStats, _ := git.GetUncommittedFileStats(repoPath)

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, fileStats, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build file graph metadata: %w", err)
	}
	depgraph.MarkDeletedFiles(&fileGraph, pureDeleted)
	depgraph.MarkRenamedFiles(&fileGraph, renames, mergeContent(deletedContent, renameOldContent), contentReader)

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

// renameSources returns the old paths (rename sources) of a renames map.
func renameSources(renames map[string]string) []string {
	if len(renames) == 0 {
		return nil
	}
	sources := make([]string, 0, len(renames))
	for oldPath := range renames {
		sources = append(sources, oldPath)
	}
	return sources
}

// mergeContent unions several path->content maps into one.
func mergeContent(maps ...map[string][]byte) map[string][]byte {
	merged := make(map[string][]byte)
	for _, m := range maps {
		for path, content := range m {
			merged[path] = content
		}
	}
	return merged
}

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
