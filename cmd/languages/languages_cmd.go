package languages

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/spf13/cobra"
)

type languageInfo struct {
	Language   string   `json:"language"`
	Maturity   string   `json:"maturity"`
	Extensions []string `json:"extensions"`
}

type languagesOutput struct {
	Languages []languageInfo `json:"languages"`
}

type options struct {
	format string
}

// Cmd represents the languages command.
var Cmd = NewCommand()

// NewCommand returns a new languages command instance.
func NewCommand() *cobra.Command {
	opts := &options{
		format: "text",
	}

	cmd := &cobra.Command{
		Use:   "languages",
		Short: "List all supported languages and file extensions",
		Long: `List all supported programming languages and their mapped file extensions.

Examples:
  clarity languages
  clarity languages --format json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLanguages(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.format, "format", opts.format, "Output format (text, json)")

	return cmd
}

func runLanguages(cmd *cobra.Command, opts *options) error {
	switch strings.ToLower(opts.format) {
	case "json":
		return renderJSON(cmd)
	case "text":
		return renderText(cmd)
	default:
		return fmt.Errorf("unknown format: %s (valid options: text, json)", opts.format)
	}
}

func renderJSON(cmd *cobra.Command) error {
	languages := registry.SupportedLanguages()
	entries := make([]languageInfo, 0, len(languages))
	for _, language := range languages {
		entries = append(entries, languageInfo{
			Language:   language.Name,
			Maturity:   language.Maturity.MachineName(),
			Extensions: language.Extensions,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(languagesOutput{Languages: entries})
}

func renderText(cmd *cobra.Command) error {
	languages := registry.SupportedLanguages()

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
			strings.Join(language.Extensions, ", ")); err != nil {
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}
	legendParts := make([]string, 0, len(moduleapi.MaturityLevels()))
	for _, level := range moduleapi.MaturityLevels() {
		legendParts = append(legendParts, fmt.Sprintf("%s %s", level.Symbol(), level.DisplayName()))
	}
	legendLine := strings.Join(legendParts, "  ")
	separator := strings.Repeat("-", utf8.RuneCountInString(legendLine))
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), separator); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), legendLine); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}

	return nil
}
