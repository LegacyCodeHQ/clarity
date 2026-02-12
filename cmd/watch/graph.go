package watch

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
)

func buildDOTGraph(repoPath string, opts *watchOptions) (string, error) {
	filePaths, err := git.GetUncommittedFiles(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get uncommitted files: %w", err)
	}

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

	contentReader := vcs.FilesystemContentReader()

	graph, err := depgraph.BuildDependencyGraph(filePaths, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build dependency graph: %w", err)
	}

	fileStats, _ := git.GetUncommittedFileStats(repoPath)

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, fileStats, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build file graph metadata: %w", err)
	}

	formatter, err := formatters.NewFormatter("dot")
	if err != nil {
		return "", err
	}

	renderOpts := formatters.RenderOptions{
		Label: "clarity watch",
	}

	return formatter.Format(fileGraph, renderOpts)
}

var errNoUncommittedChanges = fmt.Errorf("no uncommitted changes")

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
