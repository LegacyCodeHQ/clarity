// Package cycles implements the experimental `clarity cycles` command, which
// lists circular dependencies between files within a scoped set of directories.
package cycles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/spf13/cobra"
)

const cycleArrow = " → "

// Cmd represents the cycles command.
var Cmd = NewCommand()

// NewCommand returns a new cycles command instance.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cycles [path...]",
		Short: "List circular dependencies within a scope (experimental)",
		Long: `List circular dependencies between files within a scope.

Scopes to the directories (or files) you pass, defaulting to the current
directory, and reports every cyclic group of files found within that scope.
Use it to audit a module you own without walking it directory by directory.

With --url, each cycle is rendered as its own focused diagram and the command
emits a shareable visualization URL beneath it.

This command is experimental; its output may change.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCycles(cmd, args)
		},
	}
	cmd.Flags().BoolP("url", "u", false, "Emit a visualization URL for each cycle")
	return cmd
}

func runCycles(cmd *cobra.Command, args []string) error {
	inputs := args
	if len(inputs) == 0 {
		inputs = []string{"."}
	}
	emitURL, _ := cmd.Flags().GetBool("url")

	files, err := expandInputs(inputs)
	if err != nil {
		return err
	}

	contentReader := vcs.FilesystemContentReader()
	graph, err := depgraph.BuildDependencyGraph(files, contentReader)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, contentReader)
	if err != nil {
		return fmt.Errorf("failed to analyze dependency graph: %w", err)
	}

	out := cmd.OutOrStdout()
	scope := strings.Join(inputs, ", ")
	base := scopeBase(inputs)
	items := renderCycles(fileGraph.Meta.Cycles, base)

	if len(items) == 0 {
		fmt.Fprintf(out, "No cycles found in %s.\n", scope)
		return nil
	}

	noun := "cycles"
	if len(items) == 1 {
		noun = "cycle"
	}
	fmt.Fprintf(out, "Found %d %s in %s:\n\n", len(items), noun, scope)
	for i, item := range items {
		fmt.Fprintf(out, "  %d. %s\n", i+1, item.line)
		if emitURL {
			url, err := cycleURL(item.files, base, contentReader)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "     %s\n", url)
		}
	}
	return nil
}

// renderedCycle pairs a cycle's display line with the files it spans, so a
// per-cycle diagram can be rendered for the same cycle the line describes.
type renderedCycle struct {
	line  string
	files []string
}

// renderCycles turns detected cycles into display items, each line closing the
// loop back to its first file (a → b → a). Sorted for deterministic output.
func renderCycles(cycles []depgraph.FileCycle, base string) []renderedCycle {
	items := make([]renderedCycle, 0, len(cycles))
	for _, cycle := range cycles {
		if len(cycle.Path) < 2 {
			continue
		}
		names := make([]string, 0, len(cycle.Path)+1)
		for _, file := range cycle.Path {
			names = append(names, relName(base, file))
		}
		names = append(names, relName(base, cycle.Path[0]))
		items = append(items, renderedCycle{
			line:  strings.Join(names, cycleArrow),
			files: cycle.Path,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].line < items[j].line })
	return items
}

// cycleURL renders the cycle's files as a focused dependency diagram and returns
// a shareable visualization URL, mirroring `clarity show -i <files> -u`.
func cycleURL(files []string, base string, contentReader vcs.ContentReader) (string, error) {
	graph, err := depgraph.BuildDependencyGraph(files, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to build cycle graph: %w", err)
	}
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, contentReader)
	if err != nil {
		return "", fmt.Errorf("failed to analyze cycle graph: %w", err)
	}

	formatter, err := formatters.NewFormatter(formatters.OutputFormatDOT.String())
	if err != nil {
		return "", err
	}
	output, err := formatter.Format(fileGraph, formatters.RenderOptions{
		Direction: formatters.DefaultDirection,
		BasePath:  base,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render cycle diagram: %w", err)
	}

	url, ok := formatter.GenerateURL(output)
	if !ok {
		return "", fmt.Errorf("visualization URL generation is unsupported")
	}
	return url, nil
}

func relName(base, file string) string {
	if base == "" {
		return file
	}
	if rel, err := filepath.Rel(base, file); err == nil {
		return rel
	}
	return file
}

// scopeBase returns the longest common directory of the inputs, used to shorten
// file paths in the output relative to the scope the user asked about.
func scopeBase(inputs []string) string {
	var dirs []string
	for _, p := range inputs {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			dirs = append(dirs, abs)
		} else {
			dirs = append(dirs, filepath.Dir(abs))
		}
	}
	return longestCommonDir(dirs)
}

func longestCommonDir(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	common := strings.Split(filepath.Clean(dirs[0]), string(filepath.Separator))
	for _, dir := range dirs[1:] {
		parts := strings.Split(filepath.Clean(dir), string(filepath.Separator))
		max := len(common)
		if len(parts) < max {
			max = len(parts)
		}
		i := 0
		for i < max && common[i] == parts[i] {
			i++
		}
		common = common[:i]
	}
	return strings.Join(common, string(filepath.Separator))
}

// expandInputs expands directories and files into a flat list of absolute
// source-file paths, filtered to languages clarity can parse.
//
// The directory-listing helpers below mirror those in cmd/show. They are
// duplicated rather than shared while this command is experimental; if cycles
// graduates, lift them into a package both commands can use.
func expandInputs(paths []string) ([]string, error) {
	var result []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("failed to access %s: %w", p, err)
		}
		if !info.IsDir() {
			abs, err := filepath.Abs(p)
			if err != nil {
				return nil, err
			}
			result = append(result, abs)
			continue
		}

		files, err := listDirFiles(p)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if registry.IsSupportedLanguageExtension(filepath.Ext(f)) {
				result = append(result, f)
			}
		}
	}
	return result, nil
}

func listDirFiles(dir string) ([]string, error) {
	if files, err := listGitFiles(dir); err == nil {
		return files, nil
	}
	return walkDirFiles(dir)
}

func listGitFiles(dir string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	tracked, err := gitLsFiles(absDir, "--cached", "--recurse-submodules")
	if err != nil {
		return nil, err
	}
	untracked, err := gitLsFiles(absDir, "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(tracked)+len(untracked))
	for _, rel := range append(tracked, untracked...) {
		result = append(result, filepath.Join(absDir, rel))
	}
	return result, nil
}

func gitLsFiles(dir string, args ...string) ([]string, error) {
	cmdArgs := append([]string{"ls-files", "-z"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, part := range strings.Split(string(out), "\x00") {
		if p := strings.TrimSpace(part); p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

var walkSkippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	".dart_tool":   true,
	"build":        true,
	"__pycache__":  true,
	".gradle":      true,
	".idea":        true,
	".vscode":      true,
}

func walkDirFiles(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if walkSkippedDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result, err
}
