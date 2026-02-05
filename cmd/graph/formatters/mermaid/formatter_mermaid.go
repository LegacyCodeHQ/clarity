package mermaid

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/depgraph"
)

// Formatter formats dependency graphs as Mermaid.js flowcharts.
type Formatter struct{}

// Format converts the dependency graph to Mermaid.js flowchart format.
func (f *Formatter) Format(g depgraph.DependencyGraph, opts formatters.FormatOptions) (string, error) {
	var sb strings.Builder

	// Add title if label provided
	if opts.Label != "" {
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("title: %s\n", opts.Label))
		sb.WriteString("---\n")
	}

	sb.WriteString("flowchart LR\n")

	// Collect and sort file paths for deterministic output
	filePaths := make([]string, 0, len(g))
	for source := range g {
		filePaths = append(filePaths, source)
	}
	sort.Strings(filePaths)

	// Create a mapping from base filename to a valid Mermaid node ID
	// Mermaid node IDs can't have dots or special characters
	nodeIDs := make(map[string]string)
	nodeCounter := 0
	for _, source := range filePaths {
		sourceBase := filepath.Base(source)
		if _, exists := nodeIDs[sourceBase]; !exists {
			nodeIDs[sourceBase] = fmt.Sprintf("n%d", nodeCounter)
			nodeCounter++
		}
	}

	// Count files by extension to find the majority extension
	extensionCounts := make(map[string]int)
	for _, source := range filePaths {
		ext := filepath.Ext(filepath.Base(source))
		extensionCounts[ext]++
	}

	// Sort extensions for deterministic majority selection when counts are tied
	sortedExtensions := make([]string, 0, len(extensionCounts))
	for ext := range extensionCounts {
		sortedExtensions = append(sortedExtensions, ext)
	}
	sort.Strings(sortedExtensions)

	// Find the extension with the majority count
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
	for _, source := range filePaths {
		ext := filepath.Ext(filepath.Base(source))
		if ext == majorityExtension {
			filesWithMajorityExtension[source] = true
		}
	}

	// Track which nodes have been defined
	definedNodes := make(map[string]bool)

	// Define nodes with labels and styles
	for _, source := range filePaths {
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
	for _, source := range filePaths {
		deps := g[source]
		sortedDeps := make([]string, len(deps))
		copy(sortedDeps, deps)
		sort.Strings(sortedDeps)

		sourceBase := filepath.Base(source)
		sourceID := nodeIDs[sourceBase]
		for _, dep := range sortedDeps {
			depBase := filepath.Base(dep)
			depID := nodeIDs[depBase]
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", sourceID, depID))
		}
	}

	sb.WriteString("\n")

	// Add styles for different node types
	// Mermaid uses classDef for styling and class for applying styles
	var testNodes []string
	var majorityExtensionNodes []string

	// Count unique file extensions to determine if majority styling is meaningful.
	uniqueExtensions := make(map[string]bool)
	for _, source := range filePaths {
		ext := filepath.Ext(filepath.Base(source))
		uniqueExtensions[ext] = true
	}
	hasMultipleExtensions := len(uniqueExtensions) > 1

	for _, source := range filePaths {
		sourceBase := filepath.Base(source)
		nodeID := nodeIDs[sourceBase]

		if depgraph.IsTestFile(source) {
			testNodes = append(testNodes, nodeID)
		} else if hasMultipleExtensions && filesWithMajorityExtension[source] {
			majorityExtensionNodes = append(majorityExtensionNodes, nodeID)
		}
	}

	// Define style classes
	sb.WriteString("    classDef testFile fill:#90EE90,stroke:#228B22,color:#000000\n")
	if len(majorityExtensionNodes) > 0 {
		sb.WriteString("    classDef majorityExtension fill:#FFFFFF,stroke:#999999,color:#000000\n")
	}

	// Apply styles to nodes
	if len(testNodes) > 0 {
		sb.WriteString(fmt.Sprintf("    class %s testFile\n", strings.Join(testNodes, ",")))
	}
	if len(majorityExtensionNodes) > 0 {
		sb.WriteString(fmt.Sprintf("    class %s majorityExtension\n", strings.Join(majorityExtensionNodes, ",")))
	}
	return strings.TrimSuffix(sb.String(), "\n"), nil
}

// GenerateURL creates a mermaid.live URL with the diagram embedded.
func (f *Formatter) GenerateURL(output string) (string, bool) {
	payload := map[string]interface{}{
		"code": output,
		"mermaid": map[string]interface{}{
			"theme": "default",
		},
		"autoSync":      true,
		"updateDiagram": true,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		// Fallback: just return the code URL-encoded
		return fmt.Sprintf("https://mermaid.live/edit#%s", url.PathEscape(output)), true
	}

	encoded := base64.URLEncoding.EncodeToString(jsonBytes)
	return fmt.Sprintf("https://mermaid.live/edit#base64:%s", encoded), true
}
