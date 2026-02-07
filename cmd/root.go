package cmd

import (
	"os"

	"github.com/LegacyCodeHQ/sanity/cmd/graph"
	initcmd "github.com/LegacyCodeHQ/sanity/cmd/init"
	"github.com/LegacyCodeHQ/sanity/cmd/languages"
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
	Short: "Audit AI-generated code, stabilize vibe-coded apps, and understand unfamiliar codebases.",
	Long: `Sanity helps you audit AI-generated code, stabilize vibe-coded apps, and build
a solid understanding of unfamiliar codebases.

It uses a file-based dependency graph to visualize the impact of changes,
showing you the relationships between files and the order to review them.

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
	rootCmd.AddCommand(graph.Cmd)
	rootCmd.AddCommand(initcmd.Cmd)
	rootCmd.AddCommand(languages.Cmd)

	// Global flags inherited by all subcommands.
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose/debug output")
	rootCmd.PersistentFlags().BoolP("version", "V", false, "Print version information and exit")

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
