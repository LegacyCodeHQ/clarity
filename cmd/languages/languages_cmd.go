package languages

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/LegacyCodeHQ/sanity/depgraph"
	"github.com/LegacyCodeHQ/sanity/depgraph/langsupport"
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

	if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	for _, language := range languages {
		if _, err := fmt.Fprintf(
			writer,
			"%s %s\t%s\n",
			language.Maturity.Symbol(),
			language.Name,
			strings.Join(language.Extensions, ", "),
		); err != nil {
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------------"); err != nil {
		return err
	}
	legendParts := make([]string, 0, len(langsupport.MaturityLevels()))
	for _, level := range langsupport.MaturityLevels() {
		legendParts = append(legendParts, fmt.Sprintf("%s %s", level.Symbol(), level.DisplayName()))
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), strings.Join(legendParts, "  ")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}

	return nil
}
