package graph

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/git"
	"github.com/LegacyCodeHQ/sanity/parsers"

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

// GraphCmd represents the graph command
var GraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Generate dependency graph for project imports",
	Long: `Analyzes files and generates a dependency graph showing relationships
between project files (excluding external package: and dart: imports).

All files are included in the graph. Dart files will show their dependencies,
while non-Dart files appear as standalone nodes with no connections.

Supports three modes:
  1. Explicit files: Analyze specific files (--input)
  2. Uncommitted files: Analyze all uncommitted files (default: current directory)
  3. Commit analysis: Analyze files changed in a commit (--commit)

Output formats:
  - dot: Graphviz DOT format for visualization (default)
  - json: JSON format
  - mermaid: Mermaid.js flowchart format

Example usage:
  sanity graph
  sanity graph --url
  sanity graph --commit 8d4f78
  sanity graph --commit 8d4f78 --format=json
  sanity graph --commit 8d4f78 --format=mermaid
  sanity graph --format=mermaid --url
  sanity graph --repo /path/to/repo --commit 8d4f78 --format=dot
  sanity graph --input file1.dart,file2.dart,file3.dart
  sanity graph --url --commit 8d4f78
  sanity graph -u --between main.go,./git/repository.go`,
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

		// If no explicit files provided and no repo path specified, default to current directory
		if len(includes) == 0 && repoPath == "" {
			repoPath = "."
		}

		if repoPath != "" {
			// Ensure --repo and explicit files are not both provided
			if len(includes) > 0 {
				return fmt.Errorf("cannot use --repo flag with --input flag")
			}

			if commitID != "" {
				// Parse potential commit range
				fromCommit, toCommit, isCommitRange = git.ParseCommitRange(commitID)

				if isCommitRange {
					// Normalize commit range to chronological order (older...newer)
					fromCommit, toCommit, _, err = git.NormalizeCommitRange(repoPath, fromCommit, toCommit)
					if err != nil {
						return fmt.Errorf("failed to normalize commit range: %w", err)
					}

					// Commit range mode
					filePaths, err = git.GetCommitRangeFiles(repoPath, fromCommit, toCommit)
					if err != nil {
						return fmt.Errorf("failed to get files from commit range: %w", err)
					}

					if len(filePaths) == 0 {
						return fmt.Errorf("no files changed in commit range %s", commitID)
					}
				} else {
					// Single commit mode
					filePaths, err = git.GetCommitDartFiles(repoPath, toCommit)
					if err != nil {
						return fmt.Errorf("failed to get files from commit: %w", err)
					}

					if len(filePaths) == 0 {
						return fmt.Errorf("no files changed in commit %s", toCommit)
					}
				}
			} else if len(betweenFiles) > 0 {
				// When --between is provided without --commit, expand all files in working directory
				filePaths, err = expandPaths([]string{repoPath})
				if err != nil {
					return fmt.Errorf("failed to expand working directory: %w", err)
				}

				if len(filePaths) == 0 {
					return fmt.Errorf("no supported files found in working directory")
				}
			} else {
				// Uncommitted files mode
				filePaths, err = git.GetUncommittedDartFiles(repoPath)
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
		} else {
			// Validate --commit cannot be used with explicit files
			if commitID != "" {
				return fmt.Errorf("--commit flag cannot be used with --input flag")
			}

			// Explicit file mode - expand directories recursively
			filePaths, err = expandPaths(includes)
			if err != nil {
				return fmt.Errorf("failed to expand paths: %w", err)
			}

			if len(filePaths) == 0 {
				return fmt.Errorf("no supported files found in specified paths")
			}
		}

		// Build the dependency graph
		// Pass repoPath and toCommit if we're analyzing a commit (otherwise pass empty strings)
		// For ranges, toCommit is the right side; for single commits, it's the commit itself
		graph, err := parsers.BuildDependencyGraph(filePaths, repoPath, toCommit)
		if err != nil {
			return fmt.Errorf("failed to build dependency graph: %w", err)
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
		var fileStats map[string]git.FileStats
		format := formatters.OutputFormat(outputFormat)
		if (format == formatters.OutputFormatDOT || format == formatters.OutputFormatMermaid) && repoPath != "" {
			if commitID != "" {
				if isCommitRange {
					// Get stats for commit range
					fileStats, err = git.GetCommitRangeFileStats(repoPath, fromCommit, toCommit)
				} else {
					// Get stats for single commit
					fileStats, err = git.GetCommitFileStats(repoPath, toCommit)
				}
				if err != nil {
					// Don't fail if we can't get stats, just log and continue without them
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get file statistics: %v\n", err)
				}
			} else {
				// Get stats for uncommitted changes
				fileStats, err = git.GetUncommittedFileStats(repoPath)
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
			repoRoot, err := git.GetRepositoryRoot(labelRepoPath)
			if err == nil {
				projectName := filepath.Base(repoRoot)
				label = fmt.Sprintf("%s • ", projectName)
			}

			// Get commit hash or range label
			var commitLabel string
			if commitID != "" {
				if isCommitRange {
					// When analyzing a commit range, show "abc123...def456"
					commitLabel, err = git.GetCommitRangeLabel(labelRepoPath, fromCommit, toCommit)
				} else {
					// When analyzing a specific commit, show that commit's hash
					commitLabel, err = git.GetShortCommitHash(labelRepoPath, toCommit)
				}
			} else {
				// When analyzing uncommitted changes, show current HEAD
				commitLabel, err = git.GetCurrentCommitHash(labelRepoPath)
			}
			if err == nil {
				label += commitLabel

				// Only check for uncommitted changes when analyzing current state (not a specific commit)
				if commitID == "" {
					isDirty, err := git.HasUncommittedChanges(labelRepoPath)
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
				fmt.Print(output)
			}
		} else {
			fmt.Print(output)
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
	GraphCmd.Flags().StringVarP(&commitID, "commit", "c", "", "Git commit to analyze")
	// Add URL flag
	GraphCmd.Flags().BoolVarP(&generateURL, "url", "u", false, "Generate GraphvizOnline URL for visualization")
	// Add input flag for explicit files/directories
	GraphCmd.Flags().StringSliceVarP(&includes, "input", "i", nil, "Files or directories to analyze (comma-separated, directories are expanded recursively)")
	// Add between flag for finding paths between files
	GraphCmd.Flags().StringSliceVarP(&betweenFiles, "between", "w", nil, "Find all files on shortest paths between specified files (comma-separated)")
	// Add clipboard flag for copying output to clipboard
	GraphCmd.Flags().BoolVarP(&copyToClipboard, "clipboard", "b", false, "Automatically copy output to clipboard")
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
