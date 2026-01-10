package cmd

import (
	"fmt"

	"sanity/git"
	"sanity/parser"

	"github.com/spf13/cobra"
)

var outputFormat string
var repoPath string
var commitID string

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
  2. Uncommitted files: Analyze all uncommitted files (--repo)
  3. Commit analysis: Analyze files changed in a commit (--repo --commit)

Output formats:
  - list: Simple text list (default)
  - json: JSON format
  - dot: Graphviz DOT format for visualization

Example usage:
  sanity graph file1.dart file2.dart file3.dart
  sanity graph --repo .
  sanity graph --repo . --commit abc123
  sanity graph --repo . --commit HEAD~1 --format=json
  sanity graph --repo /path/to/repo --commit main --format=dot`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var filePaths []string
		var err error

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
			// Validate --commit requires --repo
			if commitID != "" {
				return fmt.Errorf("--commit flag requires --repo to specify repository path")
			}

			// Explicit file mode
			if len(args) == 0 {
				return fmt.Errorf("no files specified (use --repo or provide file paths)")
			}
			filePaths = args
		}

		// Build the dependency graph
		graph, err := parser.BuildDependencyGraph(filePaths)
		if err != nil {
			return fmt.Errorf("failed to build dependency graph: %w", err)
		}

		// Output based on format
		switch outputFormat {
		case "json":
			jsonData, err := graph.ToJSON()
			if err != nil {
				return fmt.Errorf("failed to generate JSON: %w", err)
			}
			fmt.Println(string(jsonData))

		case "dot":
			fmt.Print(graph.ToDOT())

		case "list":
			fmt.Print(graph.ToList())

		default:
			return fmt.Errorf("unknown output format: %s (valid options: list, json, dot)", outputFormat)
		}

		return nil
	},
}

func init() {
	// Add format flag
	graphCmd.Flags().StringVarP(&outputFormat, "format", "f", "list", "Output format (list, json, dot)")
	// Add repo flag
	graphCmd.Flags().StringVarP(&repoPath, "repo", "r", "", "Git repository path to analyze uncommitted files")
	// Add commit flag
	graphCmd.Flags().StringVarP(&commitID, "commit", "c", "", "Git commit to analyze (requires --repo)")
}
