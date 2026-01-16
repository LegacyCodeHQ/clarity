package formatters

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/sanity/parsers"
)

func init() {
	Register("mermaid", func() Formatter { return &MermaidFormatter{} })
}

// MermaidFormatter formats dependency graphs as Mermaid.js flowcharts.
type MermaidFormatter struct{}

// Format converts the dependency graph to Mermaid.js flowchart format.
func (f *MermaidFormatter) Format(g parsers.DependencyGraph, opts FormatOptions) (string, error) {
	var sb strings.Builder

	// Add title if label provided
	if opts.Label != "" {
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("title: %s\n", opts.Label))
		sb.WriteString("---\n")
	}

	sb.WriteString("flowchart LR\n")

	// Create a mapping from base filename to a valid Mermaid node ID
	// Mermaid node IDs can't have dots or special characters
	nodeIDs := make(map[string]string)
	nodeCounter := 0
	for source := range g {
		sourceBase := filepath.Base(source)
		if _, exists := nodeIDs[sourceBase]; !exists {
			nodeIDs[sourceBase] = fmt.Sprintf("n%d", nodeCounter)
			nodeCounter++
		}
	}

	// Collect all file paths from the graph to determine extension colors
	filePaths := make([]string, 0, len(g))
	for source := range g {
		filePaths = append(filePaths, source)
	}

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

	// Track which nodes have been defined
	definedNodes := make(map[string]bool)

	// Define nodes with labels and styles
	for source := range g {
		sourceBase := filepath.Base(source)
		nodeID := nodeIDs[sourceBase]

		if !definedNodes[sourceBase] {
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
							nodeLabel = fmt.Sprintf("%s<br/>%s", labelPrefix, strings.Join(statsParts, " "))
						} else {
							nodeLabel = labelPrefix
						}
					} else if stats.IsNew {
						nodeLabel = labelPrefix
					}
				}
			}

			// Escape quotes in labels
			nodeLabel = strings.ReplaceAll(nodeLabel, "\"", "#quot;")

			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, nodeLabel))
			definedNodes[sourceBase] = true
		}
	}

	sb.WriteString("\n")

	// Define edges
	for source, deps := range g {
		sourceBase := filepath.Base(source)
		sourceID := nodeIDs[sourceBase]
		for _, dep := range deps {
			depBase := filepath.Base(dep)
			depID := nodeIDs[depBase]
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", sourceID, depID))
		}
	}

	sb.WriteString("\n")

	// Add styles for different node types
	// Mermaid uses classDef for styling and class for applying styles
	testNodes := []string{}
	newFileNodes := []string{}

	for source := range g {
		sourceBase := filepath.Base(source)
		nodeID := nodeIDs[sourceBase]

		if IsTestFile(source) {
			testNodes = append(testNodes, nodeID)
		} else if opts.FileStats != nil {
			if stats, ok := opts.FileStats[source]; ok && stats.IsNew {
				newFileNodes = append(newFileNodes, nodeID)
			}
		}
	}

	// Define style classes
	sb.WriteString("    classDef testFile fill:#90EE90,stroke:#228B22,color:#000000\n")
	sb.WriteString("    classDef newFile fill:#87CEEB,stroke:#4682B4\n")

	// Apply styles to nodes
	if len(testNodes) > 0 {
		sb.WriteString(fmt.Sprintf("    class %s testFile\n", strings.Join(testNodes, ",")))
	}
	if len(newFileNodes) > 0 {
		sb.WriteString(fmt.Sprintf("    class %s newFile\n", strings.Join(newFileNodes, ",")))
	}
	return sb.String(), nil
}
