package cmd

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/LegacyCodeHQ/sanity/git"
	"github.com/LegacyCodeHQ/sanity/parsers"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

var outputFormat string
var repoPath string
var commitID string
var generateURL bool

// graphCmd represents the graph command
var graphCmd = &cobra.Command{
	Use:   "graph [files...]",
	Short: "Generate dependency graph for project imports",
	Long: `Analyzes files and generates a dependency graph showing relationships
between project files (excluding external package: and dart: imports).

All files are included in the graph. Dart files will show their dependencies,
while non-Dart files appear as standalone nodes with no connections.

Supports three modes:
  1. Explicit files: Analyze specific files
  2. Uncommitted files: Analyze all uncommitted files (default: current directory)
  3. Commit analysis: Analyze files changed in a commit (--commit)

Output formats:
  - dot: Graphviz DOT format for visualization (default)
  - json: JSON format

Example usage:
  sanity graph
  sanity graph --url
  sanity graph --commit 8d4f78
  sanity graph --commit 8d4f78 --format=json
  sanity graph --repo /path/to/repo --commit 8d4f78 --format=dot
  sanity graph file1.dart file2.dart file3.dart
  sanity graph --url --commit 8d4f78`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var filePaths []string
		var err error

		// If no explicit files provided and no repo path specified, default to current directory
		if len(args) == 0 && repoPath == "" {
			repoPath = "."
		}

		if repoPath != "" {
			// Ensure --repo and explicit files are not both provided
			if len(args) > 0 {
				return fmt.Errorf("cannot use --repo flag with explicit file arguments")
			}

			if commitID != "" {
				// Commit analysis mode
				filePaths, err = git.GetCommitDartFiles(repoPath, commitID)
				if err != nil {
					return fmt.Errorf("failed to get files from commit: %w", err)
				}

				if len(filePaths) == 0 {
					return fmt.Errorf("no files changed in commit %s", commitID)
				}
			} else {
				// Uncommitted files mode
				filePaths, err = git.GetUncommittedDartFiles(repoPath)
				if err != nil {
					return fmt.Errorf("failed to get uncommitted files: %w", err)
				}

				if len(filePaths) == 0 {
					return fmt.Errorf("no uncommitted files found in repository")
				}
			}
		} else {
			// Validate --commit cannot be used with explicit files
			if commitID != "" {
				return fmt.Errorf("--commit flag cannot be used with explicit file arguments")
			}

			// Explicit file mode
			filePaths = args
		}

		// Build the dependency graph
		// Pass repoPath and commitID if we're analyzing a commit (otherwise pass empty strings)
		graph, err := parsers.BuildDependencyGraph(filePaths, repoPath, commitID)
		if err != nil {
			return fmt.Errorf("failed to build dependency graph: %w", err)
		}

		// Build label with commit hash and dirty status for DOT format
		var label string
		if outputFormat == "dot" {
			// Determine the repo path to use (use current directory if not specified)
			labelRepoPath := repoPath
			if labelRepoPath == "" {
				labelRepoPath = "."
			}

			// Get repository root and extract directory name
			repoRoot, err := git.GetRepositoryRoot(labelRepoPath)
			if err == nil {
				projectName := filepath.Base(repoRoot)
				label = fmt.Sprintf("(%s) ", projectName)
			}

			// Get current commit hash
			commitHash, err := git.GetCurrentCommitHash(labelRepoPath)
			if err == nil {
				label += commitHash

				// Check if there are uncommitted changes
				isDirty, err := git.HasUncommittedChanges(labelRepoPath)
				if err == nil && isDirty {
					label += "-dirty"
				}
			}
		}

		// Output based on format
		var output string
		switch outputFormat {
		case "json":
			jsonData, err := graph.ToJSON()
			if err != nil {
				return fmt.Errorf("failed to generate JSON: %w", err)
			}
			output = string(jsonData)
			fmt.Println(output)

		case "dot":
			output = graph.ToDOT(label)

			// Generate GraphvizOnline URL if requested
			if generateURL {
				vizURL := generateGraphvizOnlineURL(output)
				fmt.Println(vizURL)
			} else {
				fmt.Print(output)
			}

		default:
			return fmt.Errorf("unknown output format: %s (valid options: dot, json)", outputFormat)
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

// generateGraphvizOnlineURL creates a URL for GraphvizOnline with the DOT graph embedded
func generateGraphvizOnlineURL(dotGraph string) string {
	// URL encode the DOT graph for use in fragment (spaces as %20, not +)
	encoded := url.PathEscape(dotGraph)

	// Create the GraphvizOnline URL with the encoded graph
	return fmt.Sprintf("https://dreampuf.github.io/GraphvizOnline/?engine=dot#%s", encoded)
}

func init() {
	// Add format flag
	graphCmd.Flags().StringVarP(&outputFormat, "format", "f", "dot", "Output format (dot, json)")
	// Add repo flag
	graphCmd.Flags().StringVarP(&repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	// Add commit flag
	graphCmd.Flags().StringVarP(&commitID, "commit", "c", "", "Git commit to analyze")
	// Add URL flag
	graphCmd.Flags().BoolVarP(&generateURL, "url", "u", false, "Generate GraphvizOnline URL for visualization")
}
