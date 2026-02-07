package setup

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	git "github.com/LegacyCodeHQ/sanity/vcs/git"
)

//go:embed SETUP.md
var setupTemplate string

// Cmd represents the setup command
var Cmd = &cobra.Command{
	Use:   "setup",
	Short: "Add sanity usage instructions to AGENTS.md",
	Long:  `Initialize AGENTS.md with instructions for AI agents to use sanity.`,
	RunE:  runSetup,
}

func runSetup(_ *cobra.Command, _ []string) error {
	repoRoot, err := git.GetRepositoryRoot(".")
	if err != nil {
		return fmt.Errorf("not a git repository (use 'git init' to initialize)")
	}

	// Create/update AGENTS.md
	created, updated, err := writeAgentsFile(filepath.Join(repoRoot, "AGENTS.md"))
	if err != nil {
		return err
	}

	if created {
		fmt.Println("✓ Created AGENTS.md with sanity usage instructions")
	} else if updated {
		fmt.Println("✓ Updated AGENTS.md with sanity usage instructions")
	} else {
		fmt.Println("✓ AGENTS.md already contains sanity usage instructions")
	}

	return nil
}

func writeAgentsFile(filename string) (bool, bool, error) {
	_, err := filepath.Abs(filename)
	if err != nil {
		return false, false, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if file exists
	_, err = os.Stat(filename)
	if err != nil && !os.IsNotExist(err) {
		return false, false, fmt.Errorf("failed to stat %s: %w", filename, err)
	}
	fileExists := !os.IsNotExist(err)

	if fileExists {
		existing, err := os.ReadFile(filename)
		if err != nil {
			return false, false, fmt.Errorf("failed to read %s: %w", filename, err)
		}

		if hasSetupBlock(existing) {
			return false, false, nil
		}

		updatedContent := appendSetupBlock(string(existing))
		if err := os.WriteFile(filename, []byte(updatedContent), 0644); err != nil {
			return false, false, fmt.Errorf("failed to update %s: %w", filename, err)
		}
	} else {
		// Create new file or overwrite
		if err := os.WriteFile(filename, []byte(buildSetupBlock(true)), 0644); err != nil {
			return false, false, fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return !fileExists, true, nil
}

func hasSetupBlock(contents []byte) bool {
	lower := strings.ToLower(string(contents))
	return strings.Contains(lower, "sanity graph")
}

func buildSetupBlock(withTrailingNewline bool) string {
	block := strings.TrimSpace(setupTemplate)
	if block == "" {
		return ""
	}
	assembled := block
	if withTrailingNewline {
		return assembled + "\n"
	}
	return assembled
}

func appendSetupBlock(existing string) string {
	trimmed := strings.TrimRight(existing, "\n")
	separator := "\n\n"
	if trimmed == "" {
		separator = ""
	}
	return trimmed + separator + buildSetupBlock(true)
}
