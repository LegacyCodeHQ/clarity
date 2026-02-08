package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/depgraph"
	"github.com/LegacyCodeHQ/sanity/vcs"
	"github.com/LegacyCodeHQ/sanity/vcs/git"

	"github.com/spf13/cobra"
)

type graphOptions struct {
	outputFormat string
	repoPath     string
	commitID     string
	generateURL  bool
	allowOutside bool
	excludeExt   string
	includes     []string
	betweenFiles []string
	targetFile   string
	depthLevel   int
}

// Cmd represents the graph command
var Cmd = NewCommand()

// NewCommand returns a new graph command instance.
func NewCommand() *cobra.Command {
	opts := &graphOptions{
		outputFormat: formatters.OutputFormatDOT.String(),
		depthLevel:   1,
	}

	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Generate a scoped file-based dependency graph.",
		Long:  `Generate a scoped file-based dependency graph.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraph(cmd, opts)
		},
	}

	// Add format flag
	cmd.Flags().StringVarP(
		&opts.outputFormat,
		"format",
		"f",
		opts.outputFormat,
		fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()))
	// Add repo flag
	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	// Add allow outside repo flag
	cmd.Flags().BoolVar(&opts.allowOutside, "allow-outside-repo", false, "Allow input paths outside the repo root")
	// Add commit flag
	cmd.Flags().StringVarP(&opts.commitID, "commit", "c", "", "Git commit or range to analyze (e.g., f0459ec, HEAD~3, f0459ec...be3d11a)")
	// Add URL flag
	cmd.Flags().BoolVarP(&opts.generateURL, "url", "u", false, "Generate visualization URL (supported formats: dot, mermaid)")
	// Add input flag for explicit files/directories
	cmd.Flags().StringSliceVarP(&opts.includes, "input", "i", nil, "Build graph from specific files and/or directories (comma-separated)")
	// Add extension exclusion flag
	cmd.Flags().StringVar(&opts.excludeExt, "exclude-ext", "", "Exclude files with this extension (single extension, e.g. .go)")
	// Add between flag for finding paths between files
	cmd.Flags().StringSliceVarP(&opts.betweenFiles, "between", "w", nil, "Find all paths between specified files (comma-separated)")
	// Add file flag for showing dependencies of a specific file
	cmd.Flags().StringVarP(&opts.targetFile, "file", "p", "", "Show dependencies for a specific file")
	// Add level flag for limiting dependency depth
	cmd.Flags().IntVarP(&opts.depthLevel, "level", "l", opts.depthLevel, "Depth level for dependencies (used with --file)")

	return cmd
}

func runGraph(cmd *cobra.Command, opts *graphOptions) error {
	if err := validateGraphOptions(opts); err != nil {
		return err
	}

	ensureRepoPath(opts)
	pathResolver, err := NewPathResolver(opts.repoPath, opts.allowOutside)
	if err != nil {
		return fmt.Errorf("failed to create path resolver: %w", err)
	}
	opts.repoPath = pathResolver.BaseDir()

	fromCommit, toCommit, isCommitRange, err := parseCommitRange(opts)
	if err != nil {
		return err
	}

	filePaths, done, err := determineFilePaths(cmd, opts, pathResolver, fromCommit, toCommit, isCommitRange)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	filePaths, err = applyExcludeExtensionFilter(opts, filePaths)
	if err != nil {
		return err
	}

	verbose, err := cmd.Flags().GetBool("verbose")
	if err == nil && verbose {
		emitUnsupportedFileWarning(cmd, filePaths)
	}

	contentReader := selectContentReader(opts, toCommit)

	graph, err := depgraph.BuildDependencyGraph(filePaths, contentReader)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	graph, filePaths, err = applyTargetFileFilter(opts, pathResolver, graph, filePaths)
	if err != nil {
		return err
	}

	graph, filePaths, err = applyBetweenFilter(opts, pathResolver, graph, filePaths)
	if err != nil {
		return err
	}

	format, _ := formatters.ParseOutputFormat(opts.outputFormat)
	fileStats := collectFileStats(cmd, opts, format, fromCommit, toCommit, isCommitRange)
	label := buildGraphLabel(opts, format, fromCommit, toCommit, isCommitRange, filePaths)
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, fileStats, contentReader)
	if err != nil {
		return fmt.Errorf("failed to build file graph metadata: %w", err)
	}

	// Create formatter and generate output
	formatter, err := NewFormatter(opts.outputFormat)
	if err != nil {
		return err
	}

	renderOpts := formatters.RenderOptions{
		Label: label,
	}

	output, err := formatter.Format(fileGraph, renderOpts)
	if err != nil {
		return fmt.Errorf("failed to format graph: %w", err)
	}

	return emitOutput(cmd, opts, format, formatter, output)
}

func validateGraphOptions(opts *graphOptions) error {
	if opts.excludeExt != "" {
		excludeExt, err := normalizeExtension(opts.excludeExt)
		if err != nil {
			return err
		}
		opts.excludeExt = excludeExt
	}

	if len(opts.betweenFiles) > 0 && len(opts.includes) > 0 {
		return fmt.Errorf("--between cannot be used with --input flag")
	}

	if opts.targetFile != "" {
		if len(opts.betweenFiles) > 0 {
			return fmt.Errorf("--file cannot be used with --between flag")
		}
		if len(opts.includes) > 0 {
			return fmt.Errorf("--file cannot be used with --input flag")
		}
		if opts.depthLevel < 1 {
			return fmt.Errorf("--level must be at least 1")
		}
	}

	return nil
}

func normalizeExtension(rawExt string) (string, error) {
	ext := strings.TrimSpace(rawExt)
	if ext == "" {
		return "", fmt.Errorf("--exclude-ext cannot be empty")
	}
	if strings.Contains(ext, ",") {
		return "", fmt.Errorf("--exclude-ext supports only one extension")
	}
	if strings.Contains(ext, string(filepath.Separator)) {
		return "", fmt.Errorf("--exclude-ext must be a file extension, got %q", rawExt)
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if ext == "." {
		return "", fmt.Errorf("--exclude-ext must include extension characters")
	}
	return strings.ToLower(ext), nil
}

func ensureRepoPath(opts *graphOptions) {
	if opts.repoPath == "" {
		opts.repoPath = "."
	}
}

func parseCommitRange(opts *graphOptions) (string, string, bool, error) {
	var fromCommit, toCommit string
	var isCommitRange bool

	if opts.commitID == "" {
		return "", "", false, nil
	}

	fromCommit, toCommit, isCommitRange = git.ParseCommitRange(opts.commitID)
	if !isCommitRange {
		return fromCommit, toCommit, isCommitRange, nil
	}

	fromCommit, toCommit, _, err := git.NormalizeCommitRange(opts.repoPath, fromCommit, toCommit)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to normalize commit range: %w", err)
	}

	return fromCommit, toCommit, isCommitRange, nil
}

func determineFilePaths(cmd *cobra.Command, opts *graphOptions, pathResolver PathResolver, fromCommit, toCommit string, isCommitRange bool) ([]string, bool, error) {
	if len(opts.includes) > 0 {
		resolvedIncludes := make([]string, 0, len(opts.includes))
		for _, include := range opts.includes {
			resolvedInclude, err := pathResolver.Resolve(RawPath(include))
			if err != nil {
				return nil, false, fmt.Errorf("failed to resolve input path %q: %w", include, err)
			}
			resolvedIncludes = append(resolvedIncludes, resolvedInclude.String())
		}

		filePaths, err := expandPaths(resolvedIncludes, true)
		if err != nil {
			return nil, false, fmt.Errorf("failed to expand paths: %w", err)
		}
		if len(filePaths) == 0 {
			return nil, false, fmt.Errorf("no files found in specified paths")
		}
		return filePaths, false, nil
	}

	if len(opts.betweenFiles) > 0 {
		filePaths, err := collectBetweenFilePaths(opts, toCommit)
		if err != nil {
			return nil, false, err
		}
		return filePaths, false, nil
	}

	if opts.commitID != "" {
		filePaths, err := collectCommitFilePaths(opts, fromCommit, toCommit, isCommitRange)
		if err != nil {
			return nil, false, err
		}
		return filePaths, false, nil
	}

	if opts.targetFile != "" {
		filePaths, err := expandPaths([]string{opts.repoPath}, false)
		if err != nil {
			return nil, false, fmt.Errorf("failed to expand working directory: %w", err)
		}
		if len(filePaths) == 0 {
			return nil, false, fmt.Errorf("no supported files found in working directory")
		}
		return filePaths, false, nil
	}

	filePaths, err := git.GetUncommittedFiles(opts.repoPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get uncommitted files: %w", err)
	}

	if len(filePaths) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Working directory is clean (no uncommitted changes).")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "To visualize the most recent commit:")
		fmt.Fprintln(cmd.OutOrStdout(), "  sanity graph -c HEAD")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "To visualize a specific commit:")
		fmt.Fprintln(cmd.OutOrStdout(), "  sanity graph -c <commit-hash>")
		return nil, true, nil
	}

	return filePaths, false, nil
}

func collectBetweenFilePaths(opts *graphOptions, toCommit string) ([]string, error) {
	if opts.commitID != "" {
		filePaths, err := git.GetCommitTreeFiles(opts.repoPath, toCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to get files from commit tree: %w", err)
		}
		if len(filePaths) == 0 {
			return nil, fmt.Errorf("no files found in commit %s", toCommit)
		}
		return filePaths, nil
	}

	filePaths, err := expandPaths([]string{opts.repoPath}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to expand working directory: %w", err)
	}
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no supported files found in working directory")
	}
	return filePaths, nil
}

func collectCommitFilePaths(opts *graphOptions, fromCommit, toCommit string, isCommitRange bool) ([]string, error) {
	if isCommitRange {
		filePaths, err := git.GetCommitRangeFiles(opts.repoPath, fromCommit, toCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to get files from commit range: %w", err)
		}
		if len(filePaths) == 0 {
			return nil, fmt.Errorf("no files changed in commit range %s", opts.commitID)
		}
		return filePaths, nil
	}

	filePaths, err := git.GetCommitDartFiles(opts.repoPath, toCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get files from commit: %w", err)
	}
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no files changed in commit %s", toCommit)
	}
	return filePaths, nil
}

func selectContentReader(opts *graphOptions, toCommit string) vcs.ContentReader {
	if toCommit != "" && opts.targetFile == "" {
		return git.GitCommitContentReader(opts.repoPath, toCommit)
	}
	return vcs.FilesystemContentReader()
}

func applyTargetFileFilter(opts *graphOptions, pathResolver PathResolver, graph depgraph.DependencyGraph, filePaths []string) (depgraph.DependencyGraph, []string, error) {
	if opts.targetFile == "" {
		return graph, filePaths, nil
	}

	absTargetFile, err := pathResolver.Resolve(RawPath(opts.targetFile))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve file path: %w", err)
	}

	if !depgraph.ContainsNode(graph, absTargetFile.String()) {
		return nil, nil, fmt.Errorf("file not found in graph: %s", opts.targetFile)
	}

	graph = filterGraphByLevel(graph, absTargetFile.String(), opts.depthLevel)
	filePaths = graphFiles(graph)

	return graph, filePaths, nil
}

func applyBetweenFilter(opts *graphOptions, pathResolver PathResolver, graph depgraph.DependencyGraph, filePaths []string) (depgraph.DependencyGraph, []string, error) {
	if len(opts.betweenFiles) == 0 {
		return graph, filePaths, nil
	}

	resolvedPaths, missingPaths := resolveAndValidatePaths(opts.betweenFiles, pathResolver, graph)
	if len(missingPaths) > 0 {
		return nil, nil, fmt.Errorf("files not found in graph: %v", missingPaths)
	}
	if len(resolvedPaths) < 2 {
		return nil, nil, fmt.Errorf("at least 2 files required for --between, found %d in graph", len(resolvedPaths))
	}

	graph = depgraph.FindPathNodes(graph, resolvedPaths)
	filePaths = graphFiles(graph)

	return graph, filePaths, nil
}

func graphFiles(graph depgraph.DependencyGraph) []string {
	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return nil
	}
	filePaths := make([]string, 0, len(adjacency))
	for f := range adjacency {
		filePaths = append(filePaths, f)
	}
	return filePaths
}

func collectFileStats(cmd *cobra.Command, opts *graphOptions, format formatters.OutputFormat, fromCommit, toCommit string, isCommitRange bool) map[string]vcs.FileStats {
	if format != formatters.OutputFormatDOT && format != formatters.OutputFormatMermaid {
		return nil
	}

	var (
		fileStats map[string]vcs.FileStats
		err       error
	)

	if opts.commitID != "" {
		if isCommitRange {
			fileStats, err = git.GetCommitRangeFileStats(opts.repoPath, fromCommit, toCommit)
		} else {
			fileStats, err = git.GetCommitFileStats(opts.repoPath, toCommit)
		}
	} else {
		fileStats, err = git.GetUncommittedFileStats(opts.repoPath)
	}

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get file statistics: %v\n", err)
		return nil
	}

	return fileStats
}

func buildGraphLabel(opts *graphOptions, format formatters.OutputFormat, fromCommit, toCommit string, isCommitRange bool, filePaths []string) string {
	if format != formatters.OutputFormatDOT && format != formatters.OutputFormatMermaid {
		return ""
	}

	labelRepoPath := opts.repoPath
	if labelRepoPath == "" {
		labelRepoPath = "."
	}

	var label string
	repoRoot, err := git.GetRepositoryRoot(labelRepoPath)
	if err == nil {
		projectName := filepath.Base(repoRoot)
		label = fmt.Sprintf("%s • ", projectName)
	}

	var commitLabel string
	if opts.commitID != "" {
		if isCommitRange {
			commitLabel, err = git.GetCommitRangeLabel(labelRepoPath, fromCommit, toCommit)
		} else {
			commitLabel, err = git.GetShortCommitHash(labelRepoPath, toCommit)
		}
	} else {
		commitLabel, err = git.GetCurrentCommitHash(labelRepoPath)
	}

	if err != nil {
		return label
	}

	label += commitLabel
	if opts.commitID == "" {
		isDirty, err := git.HasUncommittedChanges(labelRepoPath)
		if err == nil && isDirty {
			label += "-dirty"
		}
	}

	fileCount := len(filePaths)
	if fileCount == 1 {
		label += fmt.Sprintf(" • %d file", fileCount)
	} else {
		label += fmt.Sprintf(" • %d files", fileCount)
	}

	return label
}

func emitOutput(cmd *cobra.Command, opts *graphOptions, format formatters.OutputFormat, formatter formatters.Formatter, output string) error {
	if opts.generateURL {
		if urlStr, ok := formatter.GenerateURL(output); ok {
			fmt.Fprintln(cmd.OutOrStdout(), urlStr)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: URL generation is not supported for %s format\n\n", format)
			fmt.Fprintln(cmd.OutOrStdout(), output)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	return nil
}

func applyExcludeExtensionFilter(opts *graphOptions, filePaths []string) ([]string, error) {
	if opts.excludeExt == "" {
		return filePaths, nil
	}

	filtered := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		if strings.EqualFold(filepath.Ext(filePath), opts.excludeExt) {
			continue
		}
		filtered = append(filtered, filePath)
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no files remain after applying --exclude-ext %q", opts.excludeExt)
	}

	return filtered, nil
}

// expandPaths expands file paths and directories into individual file paths.
// Directories are recursively walked and regular files are included based on includeUnsupportedFiles.
func expandPaths(paths []string, includeUnsupportedFiles bool) ([]string, error) {
	var result []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", path, err)
		}

		if info.IsDir() {
			// Recursively walk directory and collect supported files
			err := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}

				// Skip directories themselves
				if fileInfo.IsDir() {
					return nil
				}

				if includeUnsupportedFiles {
					result = append(result, filePath)
					return nil
				}

				ext := filepath.Ext(filePath)
				if depgraph.IsSupportedLanguageExtension(ext) {
					result = append(result, filePath)
				}

				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to walk directory %s: %w", path, err)
			}
		} else {
			// Regular file - include it directly
			result = append(result, path)
		}
	}

	return result, nil
}

func emitUnsupportedFileWarning(cmd *cobra.Command, filePaths []string) {
	unsupportedCount := 0
	unsupportedByExt := make(map[string]bool)

	for _, filePath := range filePaths {
		ext := filepath.Ext(filePath)
		if depgraph.IsSupportedLanguageExtension(ext) {
			continue
		}

		unsupportedCount++
		if ext == "" {
			unsupportedByExt["<no extension>"] = true
			continue
		}
		unsupportedByExt[ext] = true
	}

	if unsupportedCount == 0 {
		return
	}

	unsupportedExts := make([]string, 0, len(unsupportedByExt))
	for ext := range unsupportedByExt {
		unsupportedExts = append(unsupportedExts, ext)
	}
	sort.Strings(unsupportedExts)

	fmt.Fprintf(cmd.ErrOrStderr(),
		"Warning: dependency extraction is unsupported for %d file(s) (%v); rendering standalone nodes without dependency edges for those files.\n",
		unsupportedCount,
		unsupportedExts)
}

// resolveAndValidatePaths resolves file paths to absolute paths and validates they exist in the graph.
// Returns the list of resolved paths that exist in the graph and the list of paths that were not found.
func resolveAndValidatePaths(paths []string, pathResolver PathResolver, graph depgraph.DependencyGraph) (resolved []string, missing []string) {
	for _, p := range paths {
		absPath, err := pathResolver.Resolve(RawPath(p))
		if err != nil {
			missing = append(missing, p)
			continue
		}

		if depgraph.ContainsNode(graph, absPath.String()) {
			resolved = append(resolved, absPath.String())
		} else {
			missing = append(missing, p)
		}
	}
	return
}

// filterGraphByLevel filters the dependency graph to include only nodes within
// the specified number of levels from the target file. It includes both direct
// dependencies (files the target imports) and reverse dependencies (files that
// import the target).
func filterGraphByLevel(graph depgraph.DependencyGraph, targetFile string, level int) depgraph.DependencyGraph {
	adjacency, err := depgraph.AdjacencyList(graph)
	if err != nil {
		return depgraph.NewDependencyGraph()
	}

	// Build reverse adjacency map (who depends on this file)
	reverseDeps := make(map[string][]string)
	for file, deps := range adjacency {
		for _, dep := range deps {
			reverseDeps[dep] = append(reverseDeps[dep], file)
		}
	}

	// BFS to find all nodes within the specified level
	visited := make(map[string]bool)
	visited[targetFile] = true

	currentLevel := []string{targetFile}
	for l := 0; l < level && len(currentLevel) > 0; l++ {
		nextLevel := []string{}
		for _, file := range currentLevel {
			// Add direct dependencies (files this file imports)
			for _, dep := range adjacency[file] {
				if !visited[dep] {
					visited[dep] = true
					nextLevel = append(nextLevel, dep)
				}
			}
			// Add reverse dependencies (files that import this file)
			for _, revDep := range reverseDeps[file] {
				if !visited[revDep] {
					visited[revDep] = true
					nextLevel = append(nextLevel, revDep)
				}
			}
		}
		currentLevel = nextLevel
	}

	// Build filtered graph with only visited nodes
	filtered := make(map[string][]string)
	for file := range visited {
		// Only include edges where both source and target are in the filtered set
		var filteredDeps []string
		for _, dep := range adjacency[file] {
			if visited[dep] {
				filteredDeps = append(filteredDeps, dep)
			}
		}
		filtered[file] = filteredDeps
	}

	return depgraph.MustDependencyGraph(filtered)
}
