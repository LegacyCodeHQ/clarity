package cmd

import (
	"os"
	"strconv"

	diffcmd "github.com/LegacyCodeHQ/clarity/cmd/diff"
	"github.com/LegacyCodeHQ/clarity/cmd/languages"
	setupcmd "github.com/LegacyCodeHQ/clarity/cmd/setup"
	"github.com/LegacyCodeHQ/clarity/cmd/show"
	watchcmd "github.com/LegacyCodeHQ/clarity/cmd/watch"
	whycmd "github.com/LegacyCodeHQ/clarity/cmd/why"
	"github.com/spf13/cobra"
)

// version is set via build-time ldflags
var version = "dev"

// buildDate is set via build-time ldflags
var buildDate = "unknown"

// commit is set via build-time ldflags
var commit = "unknown"

// enableDevCommands is set via build-time ldflags
var enableDevCommands = "false"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "clarity",
	Short: "A software design tool for developers and coding agents.",
	Long: `A software design tool for developers and coding agents.

Use cases:
- Build maintainable software
- Understand codebases
- Audit AI-generated code
- Stabilize and reclaim apps built with AI`,
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
	rootCmd.AddCommand(show.Cmd)
	rootCmd.AddCommand(languages.Cmd)
	rootCmd.AddCommand(setupcmd.Cmd)
	rootCmd.AddCommand(watchcmd.Cmd)
	if isDevelopmentBuild(enableDevCommands) {
		rootCmd.AddCommand(diffcmd.Cmd)
		rootCmd.AddCommand(whycmd.Cmd)
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true

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

func isDevelopmentBuild(devCommandsFlag string) bool {
	devCommandsEnabled, err := strconv.ParseBool(devCommandsFlag)
	if err != nil {
		return false
	}

	return devCommandsEnabled
}
