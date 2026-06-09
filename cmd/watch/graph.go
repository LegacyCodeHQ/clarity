package watch

import (
	"crypto/sha256"
	"encoding/hex"
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

	// A rename is an old path deleted + a new path added with identical content.
	// Detect those, add the old->new edge, and render them distinctly from plain
	// deletions. Every removed file stays visible either way.
	renames := detectRenames(deletedFiles, deletedContent, filePaths, contentReader)
	graph, err = addRenameEdges(graph, renames)
	if err != nil {
		return "", err
	}
	pureDeleted := make([]string, 0, len(deletedFiles))
	for _, d := range deletedFiles {
		if _, renamed := renames[d]; !renamed {
			pureDeleted = append(pureDeleted, d)
		}
	}

	fileStats, _ := git.GetUncommittedFileStats(repoPath)

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, fileStats, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build file graph metadata: %w", err)
	}
	depgraph.MarkDeletedFiles(&fileGraph, pureDeleted)
	depgraph.MarkRenamedFiles(&fileGraph, renames)

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

// detectRenames identifies rename sources among the deleted files: a deleted
// path whose pre-deletion (HEAD) content reappears verbatim in a currently
// existing file is treated as having been renamed to that file. Returns a map of
// old path -> new path.
func detectRenames(
	deletedFiles []string,
	deletedContent map[string][]byte,
	filePaths []string,
	contentReader vcs.ContentReader,
) map[string]string {
	if len(deletedFiles) == 0 {
		return nil
	}

	deleted := make(map[string]bool, len(deletedFiles))
	for _, d := range deletedFiles {
		deleted[d] = true
	}

	// Index currently-existing (non-deleted) files by content hash.
	byHash := make(map[string]string)
	for _, f := range filePaths {
		if deleted[f] {
			continue
		}
		content, err := contentReader(f)
		if err != nil || len(content) == 0 {
			continue
		}
		byHash[contentHash(content)] = f
	}

	renames := make(map[string]string)
	for _, d := range deletedFiles {
		content, ok := deletedContent[d]
		if !ok || len(content) == 0 {
			continue
		}
		if newPath, ok := byHash[contentHash(content)]; ok {
			renames[d] = newPath
		}
	}
	if len(renames) == 0 {
		return nil
	}
	return renames
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// addRenameEdges adds an old->new edge for each detected rename so the move is
// drawn in the graph. The new path is already a node (it's a supplied file).
func addRenameEdges(graph depgraph.DependencyGraph, renames map[string]string) (depgraph.DependencyGraph, error) {
	if len(renames) == 0 {
		return graph, nil
	}

	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to build adjacency list: %w", err)
	}
	for oldPath, newPath := range renames {
		adjacency[oldPath] = append(adjacency[oldPath], newPath)
	}

	newGraph, err := depgraph.NewDependencyGraphFromAdjacency(adjacency)
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild graph with rename edges: %w", err)
	}
	return newGraph, nil
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
