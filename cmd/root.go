package cmd

import (
	"os"

	"github.com/LegacyCodeHQ/sanity/cmd/graph"
	"github.com/spf13/cobra"
)

// version is set via build-time ldflags
var version = "dev"

// buildDate is set via build-time ldflags
var buildDate = "unknown"

// commit is set via build-time ldflags
var commit = "unknown"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sanity",
	Short: "Assess risk and confidently review AI-generated code before you commit it.",
	Long: `Sanity helps you assess risk and confidently review AI-generated code before
you commit it.

It generates a file-level dependency graph showing relationships between
project files.

Use 'sanity --help' to see all available commands, or 'sanity <command> --help'
for detailed information about a specific command.`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Register subcommands
	rootCmd.AddCommand(graph.GraphCmd)

	// Initialize annotations for version template
	if rootCmd.Annotations == nil {
		rootCmd.Annotations = make(map[string]string)
	}
	rootCmd.Annotations["buildDate"] = buildDate
	rootCmd.Annotations["commit"] = commit

	// Update version field dynamically (in case it was set via ldflags)
	rootCmd.Version = version

	// Customize version template to show additional build info
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
Build date: {{printf "%s" (index .Annotations "buildDate")}}
Commit: {{printf "%s" (index .Annotations "commit")}}
`)

}
