package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/clarityconfig"
	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
)

const (
	watchReachDown = "down"
	watchReachUp   = "up"
	watchReachBoth = "both"
)

func buildGraph(repoPath string, opts *watchOptions, formatter formatters.Formatter) (string, error) {
	// Resolve symlinks so the render base path matches the (symlink-resolved)
	// file node paths. Otherwise relative-path shortening fails (e.g. macOS
	// /var vs /private/var) and every node renders with an absolute id.
	if resolved, err := filepath.EvalSymlinks(repoPath); err == nil {
		repoPath = resolved
	}

	if err := validateWatchGraphOptions(opts); err != nil {
		return "", err
	}

	changedFiles, err := git.GetUncommittedFiles(repoPath)
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
	defaultAnchorFiles := unionWatchPaths(changedFiles, deletedFiles)

	filePaths, anchorFiles, modules, moduleMembers, err := selectWatchGraphFiles(repoPath, opts, defaultAnchorFiles)
	if err != nil {
		return "", err
	}
	if len(filePaths) == 0 {
		return "", errNoUncommittedChanges
	}

	filePaths, err = applyWatchExtensionFilters(opts, filePaths)
	if err != nil {
		return "", err
	}

	if len(opts.excludes) > 0 {
		filePaths, err = applyWatchExcludeFilter(repoPath, opts, filePaths)
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
	pureDeleted = filesPresentInGraph(graph, pureDeleted)

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

	var prunedNodes map[string]bool
	if opts.reach != "" {
		graph, prunedNodes, err = applyWatchReachFilter(repoPath, opts, graph, anchorFiles)
		if err != nil {
			return "", err
		}
	}

	if len(opts.betweenFiles) > 0 {
		graph, err = applyWatchBetweenFilter(repoPath, opts, graph)
		if err != nil {
			return "", err
		}
	}

	var collapse depgraph.Collapse
	if opts.collapse && len(modules) > 0 {
		graph, collapse, err = depgraph.CollapseModules(graph, modules)
		if err != nil {
			return "", err
		}
	}

	var fileStats map[string]vcs.FileStats
	if !opts.noStats {
		fileStats, _ = git.GetUncommittedFileStats(repoPath)
	}

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, fileStats, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build file graph metadata: %w", err)
	}
	depgraph.MarkDeletedFiles(&fileGraph, pureDeleted)
	depgraph.MarkRenamedFiles(&fileGraph, renames, mergeContent(deletedContent, renameOldContent), contentReader)
	for node := range prunedNodes {
		if md, ok := fileGraph.Meta.Files[node]; ok {
			md.IsPruned = true
			fileGraph.Meta.Files[node] = md
		}
	}
	for moduleNode, members := range collapse.Members {
		depgraph.AnnotateModule(&fileGraph, moduleNode, members, fileStats)
	}
	fileGraph.Meta.EdgeOrigins = collapse.EdgeOrigins
	if opts.moduleSelect != "" && len(moduleMembers) > 0 {
		fileGraph.Meta.ModuleCluster = &depgraph.ModuleCluster{
			Name:    opts.moduleSelect,
			Members: membersInWatchGraph(graph, moduleMembers),
		}
	}

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
		Direction:  direction,
		BasePath:   repoPath,
		EdgeLabels: opts.edgeLabels,
	}

	return formatter.Format(fileGraph, renderOpts)
}

var errNoUncommittedChanges = fmt.Errorf("no uncommitted changes")

func validateWatchGraphOptions(opts *watchOptions) error {
	opts.reach = strings.ToLower(strings.TrimSpace(opts.reach))
	switch opts.reach {
	case "", watchReachDown, watchReachUp, watchReachBoth:
	default:
		return fmt.Errorf("unknown reach: %s (valid options: up, down, both)", opts.reach)
	}
	if opts.depthLevel < 0 {
		return fmt.Errorf("--depth must be at least 0")
	}
	if opts.collapse && opts.moduleSelect != "" {
		return fmt.Errorf("--collapse cannot be used with --module")
	}
	if opts.reach != "" && opts.collapse {
		return fmt.Errorf("--reach cannot be used with --collapse")
	}
	if len(opts.betweenFiles) > 0 && opts.reach != "" {
		return fmt.Errorf("--reach cannot be used with --between")
	}
	if len(opts.betweenFiles) > 0 && opts.collapse {
		return fmt.Errorf("--collapse cannot be used with --between")
	}
	if opts.all && len(opts.includes) > 0 {
		return fmt.Errorf("--all cannot be used with paths")
	}
	if opts.all && len(opts.betweenFiles) > 0 {
		return fmt.Errorf("--all cannot be used with --between")
	}
	if opts.all && opts.moduleSelect != "" {
		return fmt.Errorf("--all cannot be used with --module")
	}
	if opts.all && opts.reach != "" {
		return fmt.Errorf("--reach cannot be used with --all")
	}
	if opts.moduleSelect != "" && len(opts.betweenFiles) > 0 {
		return fmt.Errorf("--module cannot be used with --between")
	}
	if len(opts.pruneFiles) > 0 && opts.reach == "" {
		return fmt.Errorf("--prune requires --reach")
	}
	return nil
}

func selectWatchGraphFiles(repoPath string, opts *watchOptions, defaultAnchorFiles []string) (filePaths, anchorFiles []string, modules []depgraph.Module, moduleMembers []string, err error) {
	loadModules := func() ([]depgraph.Module, error) {
		if !opts.collapse && opts.moduleSelect == "" {
			return nil, nil
		}
		return clarityconfig.LoadModules(repoPath)
	}

	if opts.all {
		filePaths, err = expandWatchPaths([]string{repoPath}, false)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		modules, err = loadModules()
		return filePaths, filePaths, modules, nil, err
	}

	if len(opts.includes) > 0 {
		modules, err = loadModules()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		anchorFiles, err = expandWatchInputPaths(repoPath, opts.includes, true)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if opts.reach != "" {
			filePaths, err = expandWatchPaths([]string{repoPath}, false)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			return filePaths, anchorFiles, modules, nil, nil
		}
		return anchorFiles, anchorFiles, modules, nil, nil
	}

	if len(opts.betweenFiles) > 0 {
		filePaths, err = expandWatchPaths([]string{repoPath}, false)
		return filePaths, filePaths, nil, nil, err
	}

	if opts.moduleSelect != "" {
		modules, err = clarityconfig.LoadModules(repoPath)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		selected, selErr := findWatchModule(opts.moduleSelect, modules)
		if selErr != nil {
			return nil, nil, nil, nil, selErr
		}
		moduleMembers = existingWatchFiles(selected.Files)
		if len(moduleMembers) == 0 {
			return nil, nil, nil, nil, fmt.Errorf("module %q resolves to no files", opts.moduleSelect)
		}
		anchorFiles = moduleMembers
		if opts.reach != "" {
			filePaths, err = expandWatchPaths([]string{repoPath}, false)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			return filePaths, anchorFiles, modules, moduleMembers, nil
		}
		return moduleMembers, anchorFiles, modules, moduleMembers, nil
	}

	if len(defaultAnchorFiles) == 0 {
		return nil, nil, nil, nil, nil
	}

	if opts.reach != "" {
		filePaths, err = expandWatchPaths([]string{repoPath}, false)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		filePaths = unionWatchPaths(filePaths, defaultAnchorFiles)
		return filePaths, defaultAnchorFiles, nil, nil, nil
	}

	modules, err = loadModules()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return defaultAnchorFiles, defaultAnchorFiles, modules, nil, nil
}

func expandWatchInputPaths(repoPath string, includes []string, includeUnsupportedFiles bool) ([]string, error) {
	resolved := make([]string, 0, len(includes))
	for _, include := range includes {
		resolved = append(resolved, resolveWatchPath(repoPath, include))
	}
	return expandWatchPaths(resolved, includeUnsupportedFiles)
}

func expandWatchPaths(paths []string, includeUnsupportedFiles bool) ([]string, error) {
	var result []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", path, err)
		}
		if !info.IsDir() {
			result = append(result, path)
			continue
		}
		err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if skippedDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if includeUnsupportedFiles || registry.IsSupportedLanguageExtension(filepath.Ext(filePath)) {
				result = append(result, filePath)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func resolveWatchPath(repoPath, raw string) string {
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(repoPath, raw))
}

func existingWatchFiles(paths []string) []string {
	files := make([]string, 0, len(paths))
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			files = append(files, path)
		}
	}
	return files
}

func unionWatchPaths(base, extra []string) []string {
	seen := make(map[string]bool, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, path := range append(base, extra...) {
		clean := filepath.Clean(path)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, path)
	}
	return out
}

func findWatchModule(name string, modules []depgraph.Module) (depgraph.Module, error) {
	available := make([]string, 0, len(modules))
	for _, module := range modules {
		if module.Name == name {
			return module, nil
		}
		available = append(available, module.Name)
	}
	if len(available) == 0 {
		return depgraph.Module{}, fmt.Errorf("unknown module %q: no modules declared in .clarity/modules.json", name)
	}
	sort.Strings(available)
	return depgraph.Module{}, fmt.Errorf("unknown module %q (available: %s)", name, strings.Join(available, ", "))
}

func applyWatchReachFilter(repoPath string, opts *watchOptions, graph depgraph.DependencyGraph, anchorFiles []string) (depgraph.DependencyGraph, map[string]bool, error) {
	pruneSet := make(map[string]bool, len(opts.pruneFiles))
	for _, prune := range opts.pruneFiles {
		pruneSet[resolveWatchPath(repoPath, prune)] = true
	}
	return filterWatchGraphByReach(graph, anchorFiles, opts.depthLevel, opts.reach, pruneSet)
}

func filterWatchGraphByReach(graph depgraph.DependencyGraph, targetFiles []string, depth int, reach string, pruneSet map[string]bool) (depgraph.DependencyGraph, map[string]bool, error) {
	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return nil, nil, err
	}
	reverseAdjacency := make(map[string][]string, len(adjacency))
	for source, deps := range adjacency {
		if _, ok := reverseAdjacency[source]; !ok {
			reverseAdjacency[source] = nil
		}
		for _, dep := range deps {
			reverseAdjacency[dep] = append(reverseAdjacency[dep], source)
		}
	}

	visited := make(map[string]bool)
	currentLevel := make([]string, 0, len(targetFiles))
	for _, target := range targetFiles {
		if _, ok := adjacency[target]; !ok {
			continue
		}
		if !visited[target] {
			visited[target] = true
			currentLevel = append(currentLevel, target)
		}
	}
	if len(currentLevel) == 0 {
		return depgraph.NewDependencyGraph(), nil, nil
	}

	for level := 0; (depth == 0 || level < depth) && len(currentLevel) > 0; level++ {
		var nextLevel []string
		for _, file := range currentLevel {
			if pruneSet[file] {
				continue
			}
			if reach == watchReachDown || reach == watchReachBoth {
				for _, dep := range adjacency[file] {
					if !visited[dep] {
						visited[dep] = true
						nextLevel = append(nextLevel, dep)
					}
				}
			}
			if reach == watchReachUp || reach == watchReachBoth {
				for _, dependent := range reverseAdjacency[file] {
					if !visited[dependent] {
						visited[dependent] = true
						nextLevel = append(nextLevel, dependent)
					}
				}
			}
		}
		currentLevel = nextLevel
	}

	filtered := make(map[string][]string, len(visited))
	for file := range visited {
		for _, dep := range adjacency[file] {
			if visited[dep] {
				filtered[file] = append(filtered[file], dep)
			}
		}
		if _, ok := filtered[file]; !ok {
			filtered[file] = nil
		}
	}

	pruned := make(map[string]bool)
	for file := range pruneSet {
		if visited[file] {
			pruned[file] = true
		}
	}

	return depgraph.MustDependencyGraph(filtered), pruned, nil
}

func applyWatchBetweenFilter(repoPath string, opts *watchOptions, graph depgraph.DependencyGraph) (depgraph.DependencyGraph, error) {
	resolved := make([]string, 0, len(opts.betweenFiles))
	var missing []string
	for _, path := range opts.betweenFiles {
		absPath := resolveWatchPath(repoPath, path)
		if depgraph.ContainsNode(graph, absPath) {
			resolved = append(resolved, absPath)
		} else {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("files not found in graph: %v", missing)
	}
	if len(resolved) < 2 {
		return nil, fmt.Errorf("at least 2 files required for --between, found %d in graph", len(resolved))
	}
	return depgraph.FindPathNodes(graph, resolved), nil
}

func filesPresentInGraph(graph depgraph.DependencyGraph, files []string) []string {
	present := make([]string, 0, len(files))
	for _, file := range files {
		if depgraph.ContainsNode(graph, file) {
			present = append(present, file)
		}
	}
	return present
}

func membersInWatchGraph(graph depgraph.DependencyGraph, members []string) []string {
	present := filesPresentInGraph(graph, members)
	sort.Strings(present)
	return present
}

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

func applyWatchExcludeFilter(repoPath string, opts *watchOptions, filePaths []string) ([]string, error) {
	excludePaths := make([]string, 0, len(opts.excludes))
	for _, exclude := range opts.excludes {
		excludePaths = append(excludePaths, resolveWatchPath(repoPath, exclude))
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
