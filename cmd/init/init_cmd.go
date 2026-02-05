package init

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

const agentsSnippet = `## Dependency Visualization

This project uses **sanity** for dependency visualization and design analysis.
Run ` + "`sanity prime`" + ` for workflow context.

**Quick reference:**
- ` + "`sanity graph -u`" + `          - Visualize and open in browser
- ` + "`sanity graph -f mermaid`" + `  - Output mermaid diagram
- ` + "`sanity graph -c HEAD~3`" + `   - Graph files from recent commits

For full workflow details: ` + "`sanity prime`" + `
`

var (
	flagCopilot bool
	flagForce   bool
	flagQuiet   bool
)

//go:embed INIT.md
var initTemplate string

// Cmd represents the init command
var Cmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize sanity in the current directory",
	Long: `Initialize sanity in the current directory by adding instructions to AGENTS.md.

This creates or updates AGENTS.md with a minimal snippet that helps AI agents
understand how to use sanity for dependency visualization.

With --copilot: Also creates .github/copilot-instructions.md for GitHub Copilot.

With --force: Overwrites existing files instead of appending.`,
	RunE: runInit,
}

func init() {
	Cmd.Flags().BoolVar(&flagCopilot, "copilot", false, "Also create .github/copilot-instructions.md")
	Cmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing files instead of appending")
	Cmd.Flags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress output")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if we're in a git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository (no .git directory found)")
	}

	// Create/update AGENTS.md
	if err := writeAgentsFile("AGENTS.md"); err != nil {
		return err
	}

	// Optionally create .github/copilot-instructions.md
	if flagCopilot {
		if err := os.MkdirAll(".github", 0755); err != nil {
			return fmt.Errorf("failed to create .github directory: %w", err)
		}
		if err := writeAgentsFile(".github/copilot-instructions.md"); err != nil {
			return err
		}
	}

	if !flagQuiet {
		fmt.Println("Sanity initialized successfully!")
		fmt.Println("")
		fmt.Println("Files updated:")
		fmt.Println("  - AGENTS.md")
		if flagCopilot {
			fmt.Println("  - .github/copilot-instructions.md")
		}
		fmt.Println("")
		fmt.Print(renderInitTemplate())
	}

	return nil
}

func renderInitTemplate() string {
	tmpl, err := template.New("init").Parse(initTemplate)
	if err != nil {
		// Best effort fallback to keep init usable even if the template is broken.
		return ""
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, nil); err != nil {
		return ""
	}

	return out.String()
}

func writeAgentsFile(filename string) error {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if file exists
	_, err = os.Stat(filename)
	fileExists := !os.IsNotExist(err)

	if fileExists && !flagForce {
		// Append to existing file
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", filename, err)
		}
		defer f.Close()

		// Add newlines before appending
		if _, err := f.WriteString("\n\n" + agentsSnippet); err != nil {
			return fmt.Errorf("failed to append to %s: %w", filename, err)
		}

		if !flagQuiet {
			fmt.Printf("Appended sanity instructions to %s\n", absPath)
		}
	} else {
		// Create new file or overwrite
		if err := os.WriteFile(filename, []byte(agentsSnippet), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}

		if !flagQuiet {
			if fileExists {
				fmt.Printf("Overwrote %s with sanity instructions\n", absPath)
			} else {
				fmt.Printf("Created %s with sanity instructions\n", absPath)
			}
		}
	}

	return nil
}
