package formatters

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/sanity/parsers"
)

func init() {
	Register(FormatDOT, func() Formatter { return &DOTFormatter{} })
}

// DOTFormatter formats dependency graphs as Graphviz DOT.
type DOTFormatter struct{}

// Format converts the dependency graph to Graphviz DOT format.
func (f *DOTFormatter) Format(g parsers.DependencyGraph, opts FormatOptions) (string, error) {
	var sb strings.Builder
	sb.WriteString("digraph dependencies {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box];\n")

	// Add label if provided
	if opts.Label != "" {
		sb.WriteString(fmt.Sprintf("  label=\"%s\";\n", opts.Label))
		sb.WriteString("  labelloc=t;\n")
		sb.WriteString("  labeljust=l;\n")
		sb.WriteString("  fontsize=10;\n")
		sb.WriteString("  fontname=Courier;\n")
	}
	sb.WriteString("\n")

	// Collect all file paths from the graph to determine extension colors
	filePaths := make([]string, 0, len(g))
	for source := range g {
		filePaths = append(filePaths, source)
	}

	// Get extension colors using the shared function
	extensionColors := GetExtensionColors(filePaths)

	// Count files by extension to find the majority extension
	extensionCounts := make(map[string]int)
	for source := range g {
		ext := filepath.Ext(filepath.Base(source))
		extensionCounts[ext]++
	}

	// Find the extension with the majority count
	maxCount := 0
	majorityExtension := ""
	for ext, count := range extensionCounts {
		if count > maxCount {
			maxCount = count
			majorityExtension = ext
		}
	}

	// Track all files that have the majority extension
	filesWithMajorityExtension := make(map[string]bool)
	for source := range g {
		ext := filepath.Ext(filepath.Base(source))
		if ext == majorityExtension {
			filesWithMajorityExtension[source] = true
		}
	}

	// Count unique file extensions to determine if we need extension-based coloring
	uniqueExtensions := make(map[string]bool)
	for source := range g {
		ext := filepath.Ext(filepath.Base(source))
		uniqueExtensions[ext] = true
	}
	hasMultipleExtensions := len(uniqueExtensions) > 1

	// Helper function to get color for an extension
	getColorForExtension := func(ext string) string {
		if color, ok := extensionColors[ext]; ok {
			return color
		}
		// If extension not found (e.g., empty extension), return white as default
		return "white"
	}

	// Track which nodes have been styled to avoid duplicates
	styledNodes := make(map[string]bool)

	// First, define node styles based on file extensions
	for source := range g {
		sourceBase := filepath.Base(source)

		if !styledNodes[sourceBase] {
			var color string

			// Priority 1: Test files are always light green
			if IsTestFile(source) {
				color = "lightgreen"
			} else if filesWithMajorityExtension[source] {
				// Priority 2: Files with majority extension count are always white
				color = "white"
			} else if hasMultipleExtensions {
				// Priority 3: Color based on extension (only if multiple extensions exist)
				ext := filepath.Ext(sourceBase)
				color = getColorForExtension(ext)
			} else {
				// Priority 4: Single extension - use white (no need to differentiate)
				color = "white"
			}

			// Build node label with file stats if available
			nodeLabel := sourceBase
			if opts.FileStats != nil {
				if stats, ok := opts.FileStats[source]; ok {
					labelPrefix := sourceBase
					if stats.IsNew {
						labelPrefix = fmt.Sprintf("🪴 %s", labelPrefix)
					}

					if stats.Additions > 0 || stats.Deletions > 0 {
						var statsParts []string
						if stats.Additions > 0 {
							statsParts = append(statsParts, fmt.Sprintf("+%d", stats.Additions))
						}
						if stats.Deletions > 0 {
							statsParts = append(statsParts, fmt.Sprintf("-%d", stats.Deletions))
						}
						if len(statsParts) > 0 {
							nodeLabel = fmt.Sprintf("%s\n%s", labelPrefix, strings.Join(statsParts, " "))
						} else {
							nodeLabel = labelPrefix
						}
					} else if stats.IsNew {
						nodeLabel = labelPrefix
					}
				}
			}

			sb.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=%s];\n", sourceBase, nodeLabel, color))
			styledNodes[sourceBase] = true
		}
	}
	if len(styledNodes) > 0 {
		sb.WriteString("\n")
	}

	for source, deps := range g {
		// Use base filename for cleaner visualization
		sourceBase := filepath.Base(source)
		for _, dep := range deps {
			depBase := filepath.Base(dep)
			sb.WriteString(fmt.Sprintf("  %q -> %q;\n", sourceBase, depBase))
		}

		// Handle files with no dependencies
		if len(deps) == 0 {
			sb.WriteString(fmt.Sprintf("  %q;\n", sourceBase))
		}
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}
