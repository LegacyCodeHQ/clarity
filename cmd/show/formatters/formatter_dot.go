package formatters

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

// Format converts the dependency graph to Graphviz DOT format.
func (f dotFormatter) Format(g depgraph.FileDependencyGraph, opts RenderOptions) (string, error) {
	adjacency, err := depgraph.AdjacencyList(g.Graph)
	if err != nil {
		return "", err
	}

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

	cycleNodes := make(map[string]bool)
	if len(g.Meta.Cycles) > 0 {
		sb.WriteString("  // Cyclic paths:\n")
		for i, cycle := range g.Meta.Cycles {
			if len(cycle.Path) == 0 {
				continue
			}
			var cycleParts []string
			for _, node := range cycle.Path {
				cycleParts = append(cycleParts, filepath.Base(node))
				cycleNodes[node] = true
			}
			cycleParts = append(cycleParts, filepath.Base(cycle.Path[0]))
			sb.WriteString(fmt.Sprintf("  // C%d: %s\n", i+1, strings.Join(cycleParts, " -> ")))
		}
		sb.WriteString("\n")
	}
	for edge, md := range g.Meta.Edges {
		if !md.InCycle {
			continue
		}
		cycleNodes[edge.From] = true
		cycleNodes[edge.To] = true
	}

	// Collect all file paths from the graph to determine extension colors
	// Sort for deterministic output
	filePaths := make([]string, 0, len(adjacency))
	for source := range adjacency {
		filePaths = append(filePaths, source)
	}
	sort.Strings(filePaths)
	nodeNames := BuildNodeNames(filePaths)

	extensionColors := getExtensionColors(filePaths)

	// Count files by extension to find the majority extension
	extensionCounts := make(map[string]int)
	for source := range adjacency {
		ext := filepath.Ext(filepath.Base(source))
		extensionCounts[ext]++
	}

	// Find the extension with the majority count
	// Sort extensions for deterministic selection when counts are tied
	sortedExtensions := make([]string, 0, len(extensionCounts))
	for ext := range extensionCounts {
		sortedExtensions = append(sortedExtensions, ext)
	}
	sort.Strings(sortedExtensions)

	maxCount := 0
	majorityExtension := ""
	for _, ext := range sortedExtensions {
		count := extensionCounts[ext]
		if count > maxCount {
			maxCount = count
			majorityExtension = ext
		}
	}

	// Track all files that have the majority extension
	filesWithMajorityExtension := make(map[string]bool)
	for source := range adjacency {
		ext := filepath.Ext(filepath.Base(source))
		if ext == majorityExtension {
			filesWithMajorityExtension[source] = true
		}
	}

	// Count unique file extensions to determine if we need extension-based coloring
	uniqueExtensions := make(map[string]bool)
	for source := range adjacency {
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
	for _, source := range filePaths {
		sourceBase := filepath.Base(source)
		sourceNodeKey := nodeNames[source]

		if !styledNodes[sourceNodeKey] {
			var color string

			fileMetadata, hasFileMetadata := g.Meta.Files[source]

			// Priority 1: Test files are always light green
			if hasFileMetadata && fileMetadata.IsTest {
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
			nodeLabel := nodeNames[source]
			if hasFileMetadata && fileMetadata.Stats != nil {
				stats := *fileMetadata.Stats
				labelPrefix := nodeLabel
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

			if cycleNodes[source] {
				sb.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=%s, color=red];\n", sourceNodeKey, nodeLabel, color))
			} else {
				sb.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=%s];\n", sourceNodeKey, nodeLabel, color))
			}
			styledNodes[sourceNodeKey] = true
		}
	}
	// Determine whether we have any edges before writing the section separator.
	hasEdges := false
	for _, deps := range adjacency {
		if len(deps) > 0 {
			hasEdges = true
			break
		}
	}
	if len(styledNodes) > 0 && hasEdges {
		sb.WriteString("\n")
	}

	// Write edges (nodes are already declared above with styling)
	for _, source := range filePaths {
		deps := adjacency[source]
		sortedDeps := make([]string, len(deps))
		copy(sortedDeps, deps)
		sort.Strings(sortedDeps)

		sourceNodeKey := nodeNames[source]
		for _, dep := range sortedDeps {
			depNodeKey := nodeNames[dep]
			edgeMD := g.Meta.Edges[depgraph.FileEdge{From: source, To: dep}]
			if edgeMD.InCycle {
				sb.WriteString(fmt.Sprintf("  %q -> %q [color=red, style=dashed];\n", sourceNodeKey, depNodeKey))
			} else {
				sb.WriteString(fmt.Sprintf("  %q -> %q;\n", sourceNodeKey, depNodeKey))
			}
		}
	}

	sb.WriteString("}")
	return sb.String(), nil
}

// GenerateURL creates a GraphvizOnline URL with the DOT graph embedded.
func (f dotFormatter) GenerateURL(output string) (string, bool) {
	encoded := url.PathEscape(output)
	return fmt.Sprintf("https://dreampuf.github.io/GraphvizOnline/?engine=dot#%s", encoded), true
}
