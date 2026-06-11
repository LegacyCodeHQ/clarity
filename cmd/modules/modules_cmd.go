package modules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/LegacyCodeHQ/clarity/clarityconfig"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/spf13/cobra"
)

type moduleInfo struct {
	Name    string
	NonTest int
	Test    int
}

func (m moduleInfo) Total() int { return m.NonTest + m.Test }

type options struct {
	repoPath string
	sortBy   string
}

// Cmd represents the modules command.
var Cmd = NewCommand()

// NewCommand returns a new modules command instance.
func NewCommand() *cobra.Command {
	opts := &options{sortBy: "name"}

	cmd := &cobra.Command{
		Use:   "modules",
		Short: "List the modules declared for this project",
		Long: `List the modules declared in .clarity/modules.json.

Each module reports the files it resolves to after expanding globs, split into
test and non-test files (mistyped patterns surface as a count of 0). Pass a
listed name to "clarity show --module <name>" to render that module.

Examples:
  clarity modules
  clarity modules --sort-by size
  clarity modules --repo path/to/repo`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModules(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	cmd.Flags().StringVar(&opts.sortBy, "sort-by", opts.sortBy, "Order modules by: name (A→Z) or size (largest first)")
	return cmd
}

func runModules(cmd *cobra.Command, opts *options) error {
	sortBy := strings.ToLower(opts.sortBy)
	if sortBy != "name" && sortBy != "size" {
		return fmt.Errorf("unknown sort-by: %s (valid options: name, size)", opts.sortBy)
	}

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

	// Test vs non-test is computed live with the same detector the graph uses,
	// rather than stored — nothing in .clarity/modules.json records it.
	reader := vcs.FilesystemContentReader()
	infos := make([]moduleInfo, 0, len(modules))
	for _, m := range modules {
		info := moduleInfo{Name: m.Name}
		for _, file := range m.Files {
			if registry.IsTestFile(file, reader) {
				info.Test++
			} else {
				info.NonTest++
			}
		}
		infos = append(infos, info)
	}
	switch sortBy {
	case "size":
		// Largest first; ties fall back to name so output stays deterministic.
		sort.Slice(infos, func(i, j int) bool {
			if infos[i].Total() != infos[j].Total() {
				return infos[i].Total() > infos[j].Total()
			}
			return infos[i].Name < infos[j].Name
		})
	default:
		sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	}

	return renderModulesText(cmd, infos)
}

func renderModulesText(cmd *cobra.Command, infos []moduleInfo) error {
	out := cmd.OutOrStdout()
	if len(infos) == 0 {
		_, err := fmt.Fprintln(out, "No modules declared (.clarity/modules.json not found or empty).")
		return err
	}

	numW := len(strconv.Itoa(len(infos)))
	nameW := len("Module")
	nonW, testW, totalW := len("Non-test"), len("Test"), len("Total")
	var sumNon, sumTest int
	for _, info := range infos {
		sumNon += info.NonTest
		sumTest += info.Test
		if l := len(info.Name); l > nameW {
			nameW = l
		}
		if l := len(strconv.Itoa(info.NonTest)); l > nonW {
			nonW = l
		}
		if l := len(strconv.Itoa(info.Test)); l > testW {
			testW = l
		}
		if l := len(strconv.Itoa(info.Total())); l > totalW {
			totalW = l
		}
	}

	header := fmt.Sprintf("%-*s  %*s  %*s  %*s", numW+2+nameW, "Module", nonW, "Non-test", testW, "Test", totalW, "Total")
	rule := strings.Repeat("─", len([]rune(header)))
	lines := make([]string, 0, len(infos)+3)
	lines = append(lines, header, rule)
	for i, info := range infos {
		row := fmt.Sprintf("%*d. %-*s  %*d  %*d  %*d", numW, i+1, nameW, info.Name, nonW, info.NonTest, testW, info.Test, totalW, info.Total())
		lines = append(lines, row)
	}
	footer := fmt.Sprintf("%d modules · %d non-test · %d test · %d files", len(infos), sumNon, sumTest, sumNon+sumTest)
	lines = append(lines, "", footer)

	_, err := fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}
