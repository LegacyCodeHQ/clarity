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

type mermaidFormatter struct{}

// Format converts the dependency graph to Mermaid.js flowchart format.
func (f mermaidFormatter) Format(g depgraph.FileDependencyGraph, opts RenderOptions) (string, error) {
	adjacency, err := depgraph.AdjacencyList(g.Graph)
	if err != nil {
		return "", err
	}

	scene := BuildScene(g, opts)
	var sb strings.Builder
	r := &mermaidRenderer{sb: &sb, explicit: scene.Header.TrailingNewline}
	r.Begin(scene.Header)

	cycleNodes := scene.CycleNodes
	if len(g.Meta.Cycles) > 0 {
		for i, cycle := range g.Meta.Cycles {
			if len(cycle.Path) == 0 {
				continue
			}

			var cycleParts []string
			for _, node := range cycle.Path {
				cycleParts = append(cycleParts, filepath.Base(node))
			}
			cycleParts = append(cycleParts, filepath.Base(cycle.Path[0]))
			sb.WriteString(fmt.Sprintf("%%%% C%d: %s\n", i+1, strings.Join(cycleParts, " -> ")))
		}
	}

	filePaths := scene.FilePaths
	nodeNames := scene.NodeNames

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

	// Track which nodes have been defined
	definedNodes := make(map[string]bool)

	// Group the selected module's member nodes so they render inside a subgraph
	// boundary. Populated only for the single-module boundary view; otherwise
	// every definition flows to outerDefs and no subgraph is drawn.
	memberSources := make(map[string]bool)
	if scene.Cluster != nil {
		for _, member := range scene.Cluster.Members {
			memberSources[member] = true
		}
	}
	var clusterDefs, outerDefs strings.Builder

	// Define nodes with labels and styles
	for _, source := range filePaths {
		sourceNodeKey := nodeNames[source]
		nodeID := nodeIDs[sourceNodeKey]

		if !definedNodes[sourceNodeKey] {
			fileMetadata, hasFileMetadata := g.Meta.Files[source]
			isModule := hasFileMetadata && fileMetadata.IsModule

			// Node label content is resolved once in the Scene; join its lines
			// with Mermaid's break and escape quotes.
			nodeLabel := strings.ReplaceAll(strings.Join(scene.Nodes[source].LabelLines, "<br/>"), "\"", "#quot;")

			// Module nodes use the subroutine shape ([[ ]]) to read as a
			// collapsed container, distinct from plain file nodes.
			target := &outerDefs
			if memberSources[source] {
				target = &clusterDefs
			}
			if isModule {
				target.WriteString(fmt.Sprintf("    %s[[\"%s\"]]\n", nodeID, nodeLabel))
			} else {
				target.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, nodeLabel))
			}
			definedNodes[sourceNodeKey] = true
		}
	}

	// Emit the module boundary subgraph (if any) around its member definitions,
	// then everything else. The subgraph is drawn only when a cluster was
	// recorded, which happens solely for the single-module view with crossings.
	if scene.Cluster != nil && clusterDefs.Len() > 0 {
		r.OpenCluster(*scene.Cluster)
		sb.WriteString(clusterDefs.String())
		r.CloseCluster()
	} else {
		sb.WriteString(clusterDefs.String())
	}
	sb.WriteString(outerDefs.String())

	phantomIDs := make(map[string]string)
	var phantomNodes []string
	var prodContextNodes []string
	for _, source := range filePaths {
		meta, ok := g.Meta.Files[source]
		if !ok || meta.Phantom == nil {
			continue
		}
		prodID := nodeIDs[nodeNames[source]]
		phantomID := prodID + "p"
		phantomIDs[source] = phantomID

		phantomLabel := nodeNames[source]
		if meta.Phantom.Stats != nil {
			stats := *meta.Phantom.Stats
			if stats.IsNew {
				phantomLabel = fmt.Sprintf("🪴 %s", phantomLabel)
			}
			var parts []string
			if stats.Additions > 0 {
				parts = append(parts, fmt.Sprintf("+%d", stats.Additions))
			}
			if stats.Deletions > 0 {
				parts = append(parts, fmt.Sprintf("-%d", stats.Deletions))
			}
			if len(parts) > 0 {
				phantomLabel = fmt.Sprintf("%s<br/>%s", phantomLabel, strings.Join(parts, " "))
			}
		}
		phantomLabel = strings.ReplaceAll(phantomLabel, "\"", "#quot;")
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", phantomID, phantomLabel))
		phantomNodes = append(phantomNodes, phantomID)

		if meta.Phantom.Stats != nil && !meta.Phantom.ProdChanged {
			prodContextNodes = append(prodContextNodes, prodID)
		}
	}

	// Define edges
	var edgesSB strings.Builder
	hasEdges := false
	edgeIndex := 0
	var cycleEdgeIndices []int
	var deletedEdgeIndices []int
	var renamedEdgeIndices []int
	var phantomEdgeIndices []int
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
			edgeMD := g.Meta.Edges[depgraph.FileEdge{From: source, To: dep}]

			if opts.EdgeLabels {
				// One arrow per underlying dependency, each labeled by its
				// original endpoints, so a collapsed module edge keeps the
				// labels it had without the module.
				for _, label := range edgeLabels(g, source, dep, opts.BasePath) {
					edgesSB.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", sourceID, label, depID))
					switch edgeMD.State {
					case depgraph.EdgeStateDeleted:
						deletedEdgeIndices = append(deletedEdgeIndices, edgeIndex)
					case depgraph.EdgeStateRenamed:
						renamedEdgeIndices = append(renamedEdgeIndices, edgeIndex)
					case depgraph.EdgeStatePresent:
					}
					if edgeMD.InCycle {
						cycleEdgeIndices = append(cycleEdgeIndices, edgeIndex)
					}
					edgeIndex++
				}
				continue
			}

			edgesSB.WriteString(fmt.Sprintf("    %s --> %s\n", sourceID, depID))
			switch edgeMD.State {
			case depgraph.EdgeStateDeleted:
				deletedEdgeIndices = append(deletedEdgeIndices, edgeIndex)
			case depgraph.EdgeStateRenamed:
				renamedEdgeIndices = append(renamedEdgeIndices, edgeIndex)
			case depgraph.EdgeStatePresent:
			}
			if edgeMD.InCycle {
				cycleEdgeIndices = append(cycleEdgeIndices, edgeIndex)
			}
			edgeIndex++
		}
	}

	for _, source := range filePaths {
		phantomID, ok := phantomIDs[source]
		if !ok {
			continue
		}
		prodID := nodeIDs[nodeNames[source]]
		hasEdges = true
		edgesSB.WriteString(fmt.Sprintf("    %s -.-> %s\n", phantomID, prodID))
		phantomEdgeIndices = append(phantomEdgeIndices, edgeIndex)
		edgeIndex++
	}

	// Add styles for different node types
	// Mermaid uses classDef for styling and class for applying styles
	var testNodes []string
	var majorityExtensionNodes []string
	var prunedNodes []string
	var moduleNodes []string
	var deletedNodes []string
	var renamedNodes []string

	for _, source := range filePaths {
		sourceNodeKey := nodeNames[source]
		nodeID := nodeIDs[sourceNodeKey]

		fileMetadata, hasFileMetadata := g.Meta.Files[source]
		if hasFileMetadata && fileMetadata.State == depgraph.FileStateDeleted {
			deletedNodes = append(deletedNodes, nodeID)
		}
		if hasFileMetadata && fileMetadata.State == depgraph.FileStateRenamed {
			renamedNodes = append(renamedNodes, nodeID)
		}
		if hasFileMetadata && fileMetadata.IsPruned {
			prunedNodes = append(prunedNodes, nodeID)
		}
		if hasFileMetadata && fileMetadata.IsModule {
			moduleNodes = append(moduleNodes, nodeID)
		} else if hasFileMetadata && fileMetadata.IsTest {
			testNodes = append(testNodes, nodeID)
		} else if scene.HasMultipleTypes && scene.FileType[source] == scene.MajorityType {
			majorityExtensionNodes = append(majorityExtensionNodes, nodeID)
		}
	}

	hasStyles := len(testNodes) > 0 || len(majorityExtensionNodes) > 0 || len(cycleNodes) > 0 || len(cycleEdgeIndices) > 0 || len(deletedEdgeIndices) > 0 || len(prunedNodes) > 0 || len(phantomNodes) > 0 || len(prodContextNodes) > 0 || len(moduleNodes) > 0 || len(deletedNodes) > 0 || len(renamedNodes) > 0 || len(renamedEdgeIndices) > 0
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
	if len(moduleNodes) > 0 {
		stylesSB.WriteString("    classDef moduleNode fill:#FFFFE0,stroke:#999999,color:#000000\n")
		stylesSB.WriteString(fmt.Sprintf("    class %s moduleNode\n", strings.Join(moduleNodes, ",")))
	}
	if len(deletedNodes) > 0 {
		stylesSB.WriteString("    classDef deletedFile fill:#FFE6E6,stroke:#CC3333,stroke-dasharray: 5 5,color:#7A0000\n")
		stylesSB.WriteString(fmt.Sprintf("    class %s deletedFile\n", strings.Join(deletedNodes, ",")))
	}
	if len(renamedNodes) > 0 {
		stylesSB.WriteString("    classDef renamedFile fill:#FFF3E0,stroke:#CC8800,stroke-dasharray: 5 5,color:#7A4D00\n")
		stylesSB.WriteString(fmt.Sprintf("    class %s renamedFile\n", strings.Join(renamedNodes, ",")))
	}
	if len(prunedNodes) > 0 {
		stylesSB.WriteString("    classDef prunedFile fill:#FFFFFF,stroke:#999999,stroke-dasharray: 5 5\n")
		stylesSB.WriteString(fmt.Sprintf("    class %s prunedFile\n", strings.Join(prunedNodes, ",")))
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
	for _, idx := range deletedEdgeIndices {
		stylesSB.WriteString(fmt.Sprintf("    linkStyle %d stroke:#CC3333,stroke-width:2px,stroke-dasharray: 5 5\n", idx))
	}
	for _, idx := range renamedEdgeIndices {
		stylesSB.WriteString(fmt.Sprintf("    linkStyle %d stroke:#CC8800,stroke-width:2px,stroke-dasharray: 5 5\n", idx))
	}
	if len(phantomNodes) > 0 {
		stylesSB.WriteString("    classDef phantomTest fill:#90EE90,stroke:#228B22,stroke-dasharray: 1 4,color:#000000\n")
		stylesSB.WriteString(fmt.Sprintf("    class %s phantomTest\n", strings.Join(phantomNodes, ",")))
	}
	if len(prodContextNodes) > 0 {
		stylesSB.WriteString("    classDef phantomProdContext stroke:#666666,stroke-dasharray: 5 5\n")
		stylesSB.WriteString(fmt.Sprintf("    class %s phantomProdContext\n", strings.Join(prodContextNodes, ",")))
	}
	for _, idx := range phantomEdgeIndices {
		stylesSB.WriteString(fmt.Sprintf("    linkStyle %d stroke:#228B22,stroke-dasharray: 5 5\n", idx))
	}

	if hasEdges {
		sb.WriteString("\n")
		sb.WriteString(edgesSB.String())
	}
	if hasStyles {
		sb.WriteString("\n")
		sb.WriteString(stylesSB.String())
	}

	return r.Finish()
}

// mermaidRenderer emits Mermaid flowchart syntax for Scene primitives. It holds
// only the output builder and trailing-newline flag; all graph derivation lives
// in the Scene.
type mermaidRenderer struct {
	sb       *strings.Builder
	explicit bool
}

func (r *mermaidRenderer) Begin(h GraphHeader) {
	if h.Title != "" {
		r.sb.WriteString("---\n")
		r.sb.WriteString(fmt.Sprintf("title: %s\n", h.Title))
		r.sb.WriteString("---\n")
	}
	r.sb.WriteString(fmt.Sprintf("flowchart %s\n", h.Orientation.String()))
}

func (r *mermaidRenderer) OpenCluster(c SceneCluster) {
	escaped := strings.ReplaceAll(c.Name, "\"", "#quot;")
	r.sb.WriteString(fmt.Sprintf("    subgraph moduleCluster[\"%s\"]\n", escaped))
}

func (r *mermaidRenderer) CloseCluster() {
	r.sb.WriteString("    end\n")
}

func (r *mermaidRenderer) Finish() (string, error) {
	output := strings.TrimSuffix(r.sb.String(), "\n")
	if r.explicit {
		return output + "\n", nil
	}
	return output, nil
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
