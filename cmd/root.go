package cmd

import (
	"log/slog"
	"os"
	"runtime/pprof"

	cyclescmd "github.com/LegacyCodeHQ/clarity/cmd/cycles"
	extensionscmd "github.com/LegacyCodeHQ/clarity/cmd/extensions"
	"github.com/LegacyCodeHQ/clarity/cmd/languages"
	modulescmd "github.com/LegacyCodeHQ/clarity/cmd/modules"
	setupcmd "github.com/LegacyCodeHQ/clarity/cmd/setup"
	"github.com/LegacyCodeHQ/clarity/cmd/show"
	watchcmd "github.com/LegacyCodeHQ/clarity/cmd/watch"
	workspacecmd "github.com/LegacyCodeHQ/clarity/cmd/workspace"
	"github.com/spf13/cobra"
)

// version is set via build-time ldflags
var version = "dev"

// commit is set via build-time ldflags
var commit = "unknown"

var cpuProfilePath string
var cpuProfileFile *os.File

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "clarity",
	Short: "A software design tool for AI-native developers and coding agents.",
	Long: `A software design tool for AI-native developers and coding agents.

Use cases:
- Keep a live impact view while coding with "clarity watch"
- Generate focused change snapshots with "clarity show"
- Run repeatable design checks in developer and coding-agent workflows`,
	Version: version,
	RunE:    runRoot,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")
		level := slog.LevelWarn
		if verbose {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{
			Level: level,
		})))

		if cpuProfilePath != "" {
			f, err := os.Create(cpuProfilePath)
			if err != nil {
				return err
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				_ = f.Close()
				return err
			}
			cpuProfileFile = f
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if cpuProfileFile != nil {
			pprof.StopCPUProfile()
			_ = cpuProfileFile.Close()
			cpuProfileFile = nil
		}
	},
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
	rootCmd.AddCommand(cyclescmd.Cmd)
	rootCmd.AddCommand(workspacecmd.Cmd)
	rootCmd.AddCommand(languages.Cmd)
	rootCmd.AddCommand(modulescmd.Cmd)
	rootCmd.AddCommand(extensionscmd.Cmd)
	rootCmd.AddCommand(setupcmd.Cmd)
	rootCmd.AddCommand(watchcmd.Cmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Global flags inherited by all subcommands.
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose/debug output")
	rootCmd.PersistentFlags().BoolP("version", "V", false, "Print version information and exit")
	rootCmd.PersistentFlags().StringVar(&cpuProfilePath, "cpu-profile", "", "Write CPU profile to file")

	// Initialize annotations for version template
	if rootCmd.Annotations == nil {
		rootCmd.Annotations = make(map[string]string)
	}
	rootCmd.Annotations["commit"] = commit

	// Update version field dynamically (in case it was set via ldflags)
	rootCmd.Version = version

	// Customize version template to show additional build info
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
Commit: {{printf "%s" (index .Annotations "commit")}}
`)
}
