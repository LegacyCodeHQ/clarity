package formatters

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

// Format converts the dependency graph to Mermaid.js flowchart format.
func (f mermaidFormatter) Format(g depgraph.FileDependencyGraph, opts RenderOptions) (string, error) {
	adjacency, err := depgraph.AdjacencyList(g.Graph)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	// Add title if label provided
	if opts.Label != "" {
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("title: %s\n", opts.Label))
		sb.WriteString("---\n")
	}

	sb.WriteString("flowchart LR\n")

	cycleNodes := make(map[string]bool)
	if len(g.Meta.Cycles) > 0 {
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
			sb.WriteString(fmt.Sprintf("%%%% C%d: %s\n", i+1, strings.Join(cycleParts, " -> ")))
		}
	}
	for edge, md := range g.Meta.Edges {
		if !md.InCycle {
			continue
		}
		cycleNodes[edge.From] = true
		cycleNodes[edge.To] = true
	}

	// Collect and sort file paths for deterministic output
	filePaths := make([]string, 0, len(adjacency))
	for source := range adjacency {
		filePaths = append(filePaths, source)
	}
	sort.Strings(filePaths)
	nodeNames := BuildNodeNames(filePaths)

	// Create a mapping from node keys to valid Mermaid node IDs.
	// Mermaid node IDs can't have dots or special characters.
	nodeIDs := make(map[string]string)
	nodeCounter := 0
	for _, source := range filePaths {
		sourceNodeKey := nodeNames[source]
		if _, exists := nodeIDs[sourceNodeKey]; !exists {
			nodeIDs[sourceNodeKey] = fmt.Sprintf("n%d", nodeCounter)
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
		sourceNodeKey := nodeNames[source]
		nodeID := nodeIDs[sourceNodeKey]

		if !definedNodes[sourceNodeKey] {
			// Build node label with file stats if available
			nodeLabel := nodeNames[source]
			if fileMetadata, ok := g.Meta.Files[source]; ok && fileMetadata.Stats != nil {
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
						nodeLabel = fmt.Sprintf("%s<br/>%s", labelPrefix, strings.Join(statsParts, " "))
					} else {
						nodeLabel = labelPrefix
					}
				} else if stats.IsNew {
					nodeLabel = labelPrefix
				}
			}

			// Escape quotes in labels
			nodeLabel = strings.ReplaceAll(nodeLabel, "\"", "#quot;")

			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, nodeLabel))
			definedNodes[sourceNodeKey] = true
		}
	}

	// Define edges
	var edgesSB strings.Builder
	hasEdges := false
	edgeIndex := 0
	var cycleEdgeIndices []int
	for _, source := range filePaths {
		deps := adjacency[source]
		sortedDeps := make([]string, len(deps))
		copy(sortedDeps, deps)
		sort.Strings(sortedDeps)

		sourceNodeKey := nodeNames[source]
		sourceID := nodeIDs[sourceNodeKey]
		for _, dep := range sortedDeps {
			depNodeKey := nodeNames[dep]
			depID := nodeIDs[depNodeKey]
			hasEdges = true
			edgesSB.WriteString(fmt.Sprintf("    %s --> %s\n", sourceID, depID))
			edgeMD := g.Meta.Edges[depgraph.FileEdge{From: source, To: dep}]
			if edgeMD.InCycle {
				cycleEdgeIndices = append(cycleEdgeIndices, edgeIndex)
			}
			edgeIndex++
		}
	}

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
		sourceNodeKey := nodeNames[source]
		nodeID := nodeIDs[sourceNodeKey]

		fileMetadata, hasFileMetadata := g.Meta.Files[source]
		if hasFileMetadata && fileMetadata.IsTest {
			testNodes = append(testNodes, nodeID)
		} else if hasMultipleExtensions && filesWithMajorityExtension[source] {
			majorityExtensionNodes = append(majorityExtensionNodes, nodeID)
		}
	}

	hasStyles := len(testNodes) > 0 || len(majorityExtensionNodes) > 0 || len(cycleNodes) > 0 || len(cycleEdgeIndices) > 0
	var stylesSB strings.Builder

	// Define style classes
	if len(testNodes) > 0 {
		stylesSB.WriteString("    classDef testFile fill:#90EE90,stroke:#228B22,color:#000000\n")
	}
	if len(majorityExtensionNodes) > 0 {
		stylesSB.WriteString("    classDef majorityExtension fill:#FFFFFF,stroke:#999999,color:#000000\n")
	}

	// Apply styles to nodes
	if len(testNodes) > 0 {
		stylesSB.WriteString(fmt.Sprintf("    class %s testFile\n", strings.Join(testNodes, ",")))
	}
	if len(majorityExtensionNodes) > 0 {
		stylesSB.WriteString(fmt.Sprintf("    class %s majorityExtension\n", strings.Join(majorityExtensionNodes, ",")))
	}
	for _, source := range filePaths {
		if !cycleNodes[source] {
			continue
		}
		sourceNodeKey := nodeNames[source]
		stylesSB.WriteString(fmt.Sprintf("    style %s stroke:#d62728,stroke-width:3px\n", nodeIDs[sourceNodeKey]))
	}
	for _, idx := range cycleEdgeIndices {
		stylesSB.WriteString(fmt.Sprintf("    linkStyle %d stroke:#d62728,stroke-width:3px,stroke-dasharray: 5 5\n", idx))
	}

	if hasEdges {
		sb.WriteString("\n")
		sb.WriteString(edgesSB.String())
	}
	if hasStyles {
		sb.WriteString("\n")
		sb.WriteString(stylesSB.String())
	}

	return strings.TrimSuffix(sb.String(), "\n"), nil
}

// GenerateURL creates a mermaid.live URL with the diagram embedded.
func (f mermaidFormatter) GenerateURL(output string) (string, bool) {
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
