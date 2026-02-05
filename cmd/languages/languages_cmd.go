package languages

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/LegacyCodeHQ/sanity/depgraph"
	"github.com/spf13/cobra"
)

// Cmd represents the languages command.
var Cmd = NewCommand()

// NewCommand returns a new languages command instance.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "languages",
		Short: "List all supported languages and file extensions",
		Long: `List all supported programming languages and their mapped file extensions.

Examples:
  sanity languages`,
		RunE: runLanguages,
	}

	return cmd
}

func runLanguages(cmd *cobra.Command, _ []string) error {
	languages := depgraph.SupportedLanguages()

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(writer, "Language\tExtensions"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "--------\t----------"); err != nil {
		return err
	}

	for _, language := range languages {
		if _, err := fmt.Fprintf(writer, "%s\t%s\n", language.Name, strings.Join(language.Extensions, ", ")); err != nil {
			return err
		}
	}

	return writer.Flush()
}
