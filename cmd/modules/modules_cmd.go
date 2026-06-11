package modules

import (
	"fmt"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/LegacyCodeHQ/clarity/clarityconfig"
	"github.com/spf13/cobra"
)

type moduleInfo struct {
	Name      string
	FileCount int
}

type options struct {
	repoPath string
}

// Cmd represents the modules command.
var Cmd = NewCommand()

// NewCommand returns a new modules command instance.
func NewCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "modules",
		Short: "List the modules declared for this project",
		Long: `List the modules declared in .clarity/modules.json.

Each module reports the number of files it resolves to after expanding globs, so
empty or mistyped patterns surface as a count of 0. Pass a listed name to
"clarity show --module <name>" to render that module and its boundary.

Examples:
  clarity modules
  clarity modules --repo path/to/repo`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModules(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	return cmd
}

func runModules(cmd *cobra.Command, opts *options) error {
	repoPath := opts.repoPath
	if repoPath == "" {
		repoPath = "."
	}
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve repo path %q: %w", repoPath, err)
	}

	modules, err := clarityconfig.LoadModules(absRepo)
	if err != nil {
		return err
	}

	infos := make([]moduleInfo, 0, len(modules))
	for _, m := range modules {
		infos = append(infos, moduleInfo{
			Name:      m.Name,
			FileCount: len(m.Files),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	return renderModulesText(cmd, infos)
}

func renderModulesText(cmd *cobra.Command, infos []moduleInfo) error {
	if len(infos) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No modules declared (.clarity/modules.json not found or empty).")
		return err
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "MODULE\tFILES"); err != nil {
		return err
	}
	for _, info := range infos {
		if _, err := fmt.Fprintf(writer, "%s\t%d\n", info.Name, info.FileCount); err != nil {
			return err
		}
	}
	return writer.Flush()
}
