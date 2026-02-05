package graph

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/depgraph"
	"github.com/LegacyCodeHQ/sanity/vcs"
	"github.com/LegacyCodeHQ/sanity/vcs/git"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

type graphOptions struct {
	outputFormat    string
	repoPath        string
	commitID        string
	generateURL     bool
	copyToClipboard bool
	includes        []string
	betweenFiles    []string
	targetFile      string
	depthLevel      int
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
		Short: "Generate a dependency graph for project files.",
		Long: `Generate a dependency graph for project files.

By default, graphs uncommitted changes. Use -c for commits or -i for specific files.

Examples:
  sanity graph                                # uncommitted changes
  sanity graph -c HEAD~3                      # single commit
  sanity graph -c f0459ec...be3d11a           # commit range
  sanity graph -i ./main.go,./lib             # specific files/directories
  sanity graph -c HEAD -i ./lib               # files in directory at commit
  sanity graph -w ./main.go,./utils.go        # paths between files
  sanity graph -p ./main.go                   # dependencies of a specific file (level 1)
  sanity graph -p ./main.go -l 2              # dependencies up to 2 levels deep
  sanity graph -u                             # generate visualization URL`,
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
		fmt.Sprintf("Output format (%s)", formatters.SupportedFormats()),
	)
	// Add repo flag
	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	// Add commit flag
	cmd.Flags().StringVarP(&opts.commitID, "commit", "c", "", "Git commit or range to analyze (e.g., f0459ec, HEAD~3, f0459ec...be3d11a)")
	// Add URL flag
	cmd.Flags().BoolVarP(&opts.generateURL, "url", "u", false, "Generate visualization URL (supported formats: dot, mermaid)")
	// Add input flag for explicit files/directories
	cmd.Flags().StringSliceVarP(&opts.includes, "input", "i", nil, "Build graph from specific files and/or directories (comma-separated)")
	// Add between flag for finding paths between files
	cmd.Flags().StringSliceVarP(&opts.betweenFiles, "between", "w", nil, "Find all paths between specified files (comma-separated)")
	// Add clipboard flag for copying output to clipboard
	cmd.Flags().BoolVarP(&opts.copyToClipboard, "clipboard", "b", false, "Automatically copy output to clipboard")
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

	fromCommit, toCommit, isCommitRange, err := parseCommitRange(opts)
	if err != nil {
		return err
	}

	filePaths, done, err := determineFilePaths(cmd, opts, fromCommit, toCommit, isCommitRange)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	contentReader := selectContentReader(opts, toCommit)

	graph, err := depgraph.BuildDependencyGraph(filePaths, contentReader)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	graph, filePaths, err = applyTargetFileFilter(opts, graph, filePaths)
	if err != nil {
		return err
	}

	graph, filePaths, err = applyBetweenFilter(opts, graph, filePaths)
	if err != nil {
		return err
	}

	format, _ := formatters.ParseOutputFormat(opts.outputFormat)
	fileStats := collectFileStats(cmd, opts, format, fromCommit, toCommit, isCommitRange)
	label := buildGraphLabel(opts, format, fromCommit, toCommit, isCommitRange, filePaths)

	// Create formatter and generate output
	formatter, err := NewFormatter(opts.outputFormat)
	if err != nil {
		return err
	}

	formatOpts := formatters.FormatOptions{
		Label:     label,
		FileStats: fileStats,
	}

	output, err := formatter.Format(graph, formatOpts)
	if err != nil {
		return fmt.Errorf("failed to format graph: %w", err)
	}

	return emitOutput(cmd, opts, format, formatter, output)
}

func validateGraphOptions(opts *graphOptions) error {
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

func determineFilePaths(cmd *cobra.Command, opts *graphOptions, fromCommit, toCommit string, isCommitRange bool) ([]string, bool, error) {
	if len(opts.includes) > 0 {
		filePaths, err := expandPaths(opts.includes)
		if err != nil {
			return nil, false, fmt.Errorf("failed to expand paths: %w", err)
		}
		if len(filePaths) == 0 {
			return nil, false, fmt.Errorf("no supported files found in specified paths")
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
		filePaths, err := expandPaths([]string{opts.repoPath})
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

	filePaths, err := expandPaths([]string{opts.repoPath})
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

func applyTargetFileFilter(opts *graphOptions, graph depgraph.DependencyGraph, filePaths []string) (depgraph.DependencyGraph, []string, error) {
	if opts.targetFile == "" {
		return graph, filePaths, nil
	}

	absTargetFile, err := filepath.Abs(opts.targetFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve file path: %w", err)
	}

	if !depgraph.ContainsNode(graph, absTargetFile) {
		return nil, nil, fmt.Errorf("file not found in graph: %s", opts.targetFile)
	}

	graph = filterGraphByLevel(graph, absTargetFile, opts.depthLevel)
	filePaths = graphFiles(graph)

	return graph, filePaths, nil
}

func applyBetweenFilter(opts *graphOptions, graph depgraph.DependencyGraph, filePaths []string) (depgraph.DependencyGraph, []string, error) {
	if len(opts.betweenFiles) == 0 {
		return graph, filePaths, nil
	}

	resolvedPaths, missingPaths := resolveAndValidatePaths(opts.betweenFiles, graph)
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

	if opts.copyToClipboard {
		if err := clipboard.WriteAll(output); err != nil {
			return fmt.Errorf("failed to copy to clipboard: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\n✅ Content copied to your clipboard.")
	}

	return nil
}

// supportedExtensions contains file extensions that the graph command can analyze
var supportedExtensions = map[string]bool{
	".dart": true,
	".go":   true,
	".kt":   true,
	".ts":   true,
	".tsx":  true,
}

// expandPaths expands file paths and directories into individual file paths.
// Directories are recursively walked and only files with supported extensions are included.
func expandPaths(paths []string) ([]string, error) {
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

				// Check if file has a supported extension
				ext := filepath.Ext(filePath)
				if supportedExtensions[ext] {
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

// resolveAndValidatePaths resolves file paths to absolute paths and validates they exist in the graph.
// Returns the list of resolved paths that exist in the graph and the list of paths that were not found.
func resolveAndValidatePaths(paths []string, graph depgraph.DependencyGraph) (resolved []string, missing []string) {
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			missing = append(missing, p)
			continue
		}

		if depgraph.ContainsNode(graph, absPath) {
			resolved = append(resolved, absPath)
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
