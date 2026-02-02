package graph

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/parsers"
	"github.com/LegacyCodeHQ/sanity/vcs"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

var outputFormat string
var repoPath string
var commitID string
var generateURL bool
var copyToClipboard bool
var includes []string
var betweenFiles []string
var targetFile string
var depthLevel int

// GraphCmd represents the graph command
var GraphCmd = &cobra.Command{
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
		var filePaths []string
		var err error

		// Track commit range info for use throughout the function
		var fromCommit, toCommit string
		var isCommitRange bool

		// Validate --between cannot be used with --input
		if len(betweenFiles) > 0 && len(includes) > 0 {
			return fmt.Errorf("--between cannot be used with --input flag")
		}

		// Validate --file cannot be used with --between or --input
		if targetFile != "" {
			if len(betweenFiles) > 0 {
				return fmt.Errorf("--file cannot be used with --between flag")
			}
			if len(includes) > 0 {
				return fmt.Errorf("--file cannot be used with --input flag")
			}
			if depthLevel < 1 {
				return fmt.Errorf("--level must be at least 1")
			}
		}

		// Default to current directory for git operations if not specified
		if repoPath == "" {
			repoPath = "."
		}

		// Parse commit range if --commit is specified
		if commitID != "" {
			fromCommit, toCommit, isCommitRange = vcs.ParseCommitRange(commitID)

			if isCommitRange {
				// Normalize commit range to chronological order (older...newer)
				fromCommit, toCommit, _, err = vcs.NormalizeCommitRange(repoPath, fromCommit, toCommit)
				if err != nil {
					return fmt.Errorf("failed to normalize commit range: %w", err)
				}
			}
		}

		// Determine file paths based on flags
		if len(includes) > 0 {
			// Explicit file/directory mode - expand directories recursively
			filePaths, err = expandPaths(includes)
			if err != nil {
				return fmt.Errorf("failed to expand paths: %w", err)
			}

			if len(filePaths) == 0 {
				return fmt.Errorf("no supported files found in specified paths")
			}
		} else if commitID != "" {
			// Commit mode without explicit files - get files changed in commit
			if isCommitRange {
				filePaths, err = vcs.GetCommitRangeFiles(repoPath, fromCommit, toCommit)
				if err != nil {
					return fmt.Errorf("failed to get files from commit range: %w", err)
				}

				if len(filePaths) == 0 {
					return fmt.Errorf("no files changed in commit range %s", commitID)
				}
			} else {
				filePaths, err = vcs.GetCommitDartFiles(repoPath, toCommit)
				if err != nil {
					return fmt.Errorf("failed to get files from commit: %w", err)
				}

				if len(filePaths) == 0 {
					return fmt.Errorf("no files changed in commit %s", toCommit)
				}
			}
		} else if len(betweenFiles) > 0 {
			// When --between is provided without --commit or --input, expand all files in working directory
			filePaths, err = expandPaths([]string{repoPath})
			if err != nil {
				return fmt.Errorf("failed to expand working directory: %w", err)
			}

			if len(filePaths) == 0 {
				return fmt.Errorf("no supported files found in working directory")
			}
		} else if targetFile != "" {
			// When --file is provided, expand all files in working directory to build full graph
			filePaths, err = expandPaths([]string{repoPath})
			if err != nil {
				return fmt.Errorf("failed to expand working directory: %w", err)
			}

			if len(filePaths) == 0 {
				return fmt.Errorf("no supported files found in working directory")
			}
		} else {
			// Default: uncommitted files mode
			filePaths, err = vcs.GetUncommittedDartFiles(repoPath)
			if err != nil {
				return fmt.Errorf("failed to get uncommitted files: %w", err)
			}

			if len(filePaths) == 0 {
				fmt.Println("Working directory is clean (no uncommitted changes).")
				fmt.Println()
				fmt.Println("To visualize the most recent commit:")
				fmt.Println("  sanity graph -c HEAD")
				fmt.Println()
				fmt.Println("To visualize a specific commit:")
				fmt.Println("  sanity graph -c <commit-hash>")
				return nil
			}
		}

		// Build the dependency graph
		// Create the appropriate content reader based on whether we're analyzing a commit
		var contentReader vcs.ContentReader
		if toCommit != "" {
			contentReader = func(absPath string) ([]byte, error) {
				relPath := parsers.GetRelativePath(absPath, repoPath)
				return vcs.GetFileContentFromCommit(repoPath, toCommit, relPath)
			}
		} else {
			contentReader = vcs.FilesystemContentReader()
		}

		graph, err := parsers.BuildDependencyGraph(filePaths, contentReader)
		if err != nil {
			return fmt.Errorf("failed to build dependency graph: %w", err)
		}

		// Apply level filtering if --file flag is provided
		if targetFile != "" {
			// Resolve the target file to absolute path
			absTargetFile, err := filepath.Abs(targetFile)
			if err != nil {
				return fmt.Errorf("failed to resolve file path: %w", err)
			}

			// Verify the target file exists in the graph
			if _, ok := graph[absTargetFile]; !ok {
				return fmt.Errorf("file not found in graph: %s", targetFile)
			}

			// Filter graph to only include nodes within the specified level
			graph = filterGraphByLevel(graph, absTargetFile, depthLevel)

			// Update filePaths to match filtered graph for accurate file count
			filePaths = make([]string, 0, len(graph))
			for f := range graph {
				filePaths = append(filePaths, f)
			}
		}

		// Apply path filtering if --between flag is provided
		if len(betweenFiles) > 0 {
			// Resolve paths to absolute paths and validate they exist in the graph
			resolvedPaths, missingPaths := resolveAndValidatePaths(betweenFiles, graph)
			if len(missingPaths) > 0 {
				return fmt.Errorf("files not found in graph: %v", missingPaths)
			}
			if len(resolvedPaths) < 2 {
				return fmt.Errorf("at least 2 files required for --between, found %d in graph", len(resolvedPaths))
			}
			graph = parsers.FindPathNodes(graph, resolvedPaths)

			// Update filePaths to match filtered graph for accurate file count
			filePaths = make([]string, 0, len(graph))
			for f := range graph {
				filePaths = append(filePaths, f)
			}
		}

		// Get file statistics for DOT/Mermaid formats
		var fileStats map[string]vcs.FileStats
		format, _ := formatters.ParseOutputFormat(outputFormat)
		if (format == formatters.OutputFormatDOT || format == formatters.OutputFormatMermaid) && repoPath != "" {
			if commitID != "" {
				if isCommitRange {
					// Get stats for commit range
					fileStats, err = vcs.GetCommitRangeFileStats(repoPath, fromCommit, toCommit)
				} else {
					// Get stats for single commit
					fileStats, err = vcs.GetCommitFileStats(repoPath, toCommit)
				}
				if err != nil {
					// Don't fail if we can't get stats, just log and continue without them
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get file statistics: %v\n", err)
				}
			} else {
				// Get stats for uncommitted changes
				fileStats, err = vcs.GetUncommittedFileStats(repoPath)
				if err != nil {
					// Don't fail if we can't get stats, just log and continue without them
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get file statistics: %v\n", err)
				}
			}
		}

		// Build label with commit hash and dirty status for DOT/Mermaid formats
		var label string
		if format == formatters.OutputFormatDOT || format == formatters.OutputFormatMermaid {
			// Determine the repo path to use (use current directory if not specified)
			labelRepoPath := repoPath
			if labelRepoPath == "" {
				labelRepoPath = "."
			}

			// Get repository root and extract directory name
			repoRoot, err := vcs.GetRepositoryRoot(labelRepoPath)
			if err == nil {
				projectName := filepath.Base(repoRoot)
				label = fmt.Sprintf("%s • ", projectName)
			}

			// Get commit hash or range label
			var commitLabel string
			if commitID != "" {
				if isCommitRange {
					// When analyzing a commit range, show "abc123...def456"
					commitLabel, err = vcs.GetCommitRangeLabel(labelRepoPath, fromCommit, toCommit)
				} else {
					// When analyzing a specific commit, show that commit's hash
					commitLabel, err = vcs.GetShortCommitHash(labelRepoPath, toCommit)
				}
			} else {
				// When analyzing uncommitted changes, show current HEAD
				commitLabel, err = vcs.GetCurrentCommitHash(labelRepoPath)
			}
			if err == nil {
				label += commitLabel

				// Only check for uncommitted changes when analyzing current state (not a specific commit)
				if commitID == "" {
					isDirty, err := vcs.HasUncommittedChanges(labelRepoPath)
					if err == nil && isDirty {
						label += "-dirty"
					}
				}

				// Add number of files changed
				fileCount := len(filePaths)
				if fileCount == 1 {
					label += fmt.Sprintf(" • %d file", fileCount)
				} else {
					label += fmt.Sprintf(" • %d files", fileCount)
				}
			}
		}

		// Create formatter and generate output
		formatter, err := formatters.NewFormatter(outputFormat)
		if err != nil {
			return err
		}

		opts := formatters.FormatOptions{
			Label:     label,
			FileStats: fileStats,
		}

		output, err := formatter.Format(graph, opts)
		if err != nil {
			return fmt.Errorf("failed to format graph: %w", err)
		}

		// Handle URL generation and output
		if generateURL {
			if urlStr, ok := formatter.GenerateURL(output); ok {
				fmt.Println(urlStr)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: URL generation is not supported for %s format\n\n", format)
				fmt.Println(output)
			}
		} else {
			fmt.Println(output)
		}

		// Copy to clipboard if flag is enabled
		if copyToClipboard {
			if err := clipboard.WriteAll(output); err != nil {
				return fmt.Errorf("failed to copy to clipboard: %w", err)
			}
			fmt.Println("\n✅ Content copied to your clipboard.")
		}

		return nil
	},
}

func init() {
	// Add format flag
	GraphCmd.Flags().StringVarP(&outputFormat, "format", "f", formatters.OutputFormatDOT.String(),
		fmt.Sprintf("Output format (%s, %s, %s)", formatters.OutputFormatDOT, formatters.OutputFormatJSON, formatters.OutputFormatMermaid))
	// Add repo flag
	GraphCmd.Flags().StringVarP(&repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	// Add commit flag
	GraphCmd.Flags().StringVarP(&commitID, "commit", "c", "", "Git commit or range to analyze (e.g., f0459ec, HEAD~3, f0459ec...be3d11a)")
	// Add URL flag
	GraphCmd.Flags().BoolVarP(&generateURL, "url", "u", false, "Generate visualization URL (supported formats: dot, mermaid)")
	// Add input flag for explicit files/directories
	GraphCmd.Flags().StringSliceVarP(&includes, "input", "i", nil, "Build graph from specific files and/or directories (comma-separated)")
	// Add between flag for finding paths between files
	GraphCmd.Flags().StringSliceVarP(&betweenFiles, "between", "w", nil, "Find all paths between specified files (comma-separated)")
	// Add clipboard flag for copying output to clipboard
	GraphCmd.Flags().BoolVarP(&copyToClipboard, "clipboard", "b", false, "Automatically copy output to clipboard")
	// Add file flag for showing dependencies of a specific file
	GraphCmd.Flags().StringVarP(&targetFile, "file", "p", "", "Show dependencies for a specific file")
	// Add level flag for limiting dependency depth
	GraphCmd.Flags().IntVarP(&depthLevel, "level", "l", 1, "Depth level for dependencies (used with --file)")
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
func resolveAndValidatePaths(paths []string, graph parsers.DependencyGraph) (resolved []string, missing []string) {
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			missing = append(missing, p)
			continue
		}

		if _, ok := graph[absPath]; ok {
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
func filterGraphByLevel(graph parsers.DependencyGraph, targetFile string, level int) parsers.DependencyGraph {
	// Build reverse adjacency map (who depends on this file)
	reverseDeps := make(map[string][]string)
	for file, deps := range graph {
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
			for _, dep := range graph[file] {
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
	filtered := make(parsers.DependencyGraph)
	for file := range visited {
		// Only include edges where both source and target are in the filtered set
		var filteredDeps []string
		for _, dep := range graph[file] {
			if visited[dep] {
				filteredDeps = append(filteredDeps, dep)
			}
		}
		filtered[file] = filteredDeps
	}

	return filtered
}
