package formatters

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

type dotFormatter struct {
	extensionColors   map[string]string
	nextColorPaletteI int
}

// Format converts the dependency graph to Graphviz DOT format.
func (f *dotFormatter) Format(g depgraph.FileDependencyGraph, opts RenderOptions) (string, error) {
	adjacency, err := depgraph.AdjacencyList(g.Graph)
	if err != nil {
		return "", err
	}

	scene := BuildScene(g, opts)
	var sb strings.Builder
	r := &dotRenderer{sb: &sb, explicit: scene.Header.TrailingNewline}
	r.Begin(scene.Header)

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

	extensionColors := f.assignExtensionColors(filePaths)

	// Count files by type key to find the majority type. Extensionless files
	// are keyed by basename so each one (e.g. `pre-commit`, `pre-push`) is its
	// own type rather than collapsing into a single empty-string bucket.
	extensionCounts := make(map[string]int)
	for source := range adjacency {
		extensionCounts[fileTypeKey(source)]++
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
		if fileTypeKey(source) == majorityExtension {
			filesWithMajorityExtension[source] = true
		}
	}

	// Count unique file extensions to determine if we need extension-based coloring
	uniqueExtensions := make(map[string]bool)
	for source := range adjacency {
		uniqueExtensions[fileTypeKey(source)] = true
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

	// Group the selected module's member nodes so they render inside a drawn
	// boundary. Populated only for the single-module boundary view; otherwise
	// every node declaration flows to outerDecls and no box is drawn.
	memberNodeKeys := make(map[string]bool)
	moduleClusterName := ""
	if g.Meta.ModuleCluster != nil {
		moduleClusterName = g.Meta.ModuleCluster.Name
		for _, member := range g.Meta.ModuleCluster.Members {
			memberNodeKeys[nodeKey(member, opts.BasePath)] = true
		}
	}
	var clusterDecls, outerDecls strings.Builder

	// First, define node styles based on file extensions
	for _, source := range filePaths {
		sourceBase := filepath.Base(source)
		sourceNodeKey := nodeKey(source, opts.BasePath)

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
				color = getColorForExtension(fileTypeKey(sourceBase))
			} else {
				// Priority 4: Single extension - use white (no need to differentiate)
				color = "white"
			}

			// Module nodes are synthetic (a collapsed set of files), so give
			// them a fixed fill rather than an extension-derived one.
			isModule := hasFileMetadata && fileMetadata.IsModule
			if isModule {
				color = "lightyellow"
			}

			// Build node label with file stats if available
			nodeLabel := nodeNames[source]
			if isModule {
				nodeLabel = moduleNodeLabel(nodeLabel, fileMetadata, "\n")
			} else if hasFileMetadata && fileMetadata.Stats != nil {
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
			isDeleted := hasFileMetadata && fileMetadata.State == depgraph.FileStateDeleted
			isRenamed := hasFileMetadata && fileMetadata.State == depgraph.FileStateRenamed
			if isDeleted {
				nodeLabel = fmt.Sprintf("%s\n(deleted)", nodeLabel)
			} else if isRenamed {
				nodeLabel = fmt.Sprintf("%s\n(renamed)", nodeLabel)
			}

			prodIsContext := hasFileMetadata &&
				fileMetadata.Phantom != nil &&
				fileMetadata.Phantom.Stats != nil &&
				!fileMetadata.Phantom.ProdChanged

			target := &outerDecls
			if memberNodeKeys[sourceNodeKey] {
				target = &clusterDecls
			}

			switch {
			case isDeleted:
				target.WriteString(fmt.Sprintf("  %q [label=%q, style=\"filled,dashed\", fillcolor=\"#ffe6e6\", color=\"#cc3333\", fontcolor=\"#7a0000\"];\n", sourceNodeKey, nodeLabel))
			case isRenamed:
				target.WriteString(fmt.Sprintf("  %q [label=%q, style=\"filled,dashed\", fillcolor=\"#fff3e0\", color=\"#cc8800\", fontcolor=\"#7a4d00\"];\n", sourceNodeKey, nodeLabel))
			case isModule:
				// A module renders as a single component-shaped node; keep the
				// red cycle border when the collapsed node participates in one.
				moduleBorder := ""
				if cycleNodes[source] {
					moduleBorder = ", color=red"
				}
				target.WriteString(fmt.Sprintf("  %q [label=%q, shape=component, style=filled, fillcolor=%s%s];\n", sourceNodeKey, nodeLabel, color, moduleBorder))
			case hasFileMetadata && fileMetadata.IsPruned && cycleNodes[source]:
				target.WriteString(fmt.Sprintf("  %q [label=%q, style=\"filled,dashed\", fillcolor=%s, color=red];\n", sourceNodeKey, nodeLabel, color))
			case hasFileMetadata && fileMetadata.IsPruned:
				target.WriteString(fmt.Sprintf("  %q [label=%q, style=\"filled,dashed\", fillcolor=%s, color=gray];\n", sourceNodeKey, nodeLabel, color))
			case cycleNodes[source]:
				target.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=%s, color=red];\n", sourceNodeKey, nodeLabel, color))
			case prodIsContext:
				target.WriteString(fmt.Sprintf("  %q [label=%q, style=\"filled,dashed\", fillcolor=%s];\n", sourceNodeKey, nodeLabel, color))
			default:
				target.WriteString(fmt.Sprintf("  %q [label=%q, style=filled, fillcolor=%s];\n", sourceNodeKey, nodeLabel, color))
			}
			styledNodes[sourceNodeKey] = true
		}
	}
	// Emit the module boundary box (if any) around its member declarations,
	// then everything else. The box is only drawn when a cluster was recorded,
	// which happens solely for the single-module view with crossing edges.
	if moduleClusterName != "" && clusterDecls.Len() > 0 {
		sb.WriteString(fmt.Sprintf("  subgraph cluster_module {\n    label=%q;\n    labeljust=l;\n    style=rounded;\n    color=\"#888888\";\n    fontname=Courier;\n", moduleClusterName))
		sb.WriteString(clusterDecls.String())
		sb.WriteString("  }\n")
	} else {
		sb.WriteString(clusterDecls.String())
	}
	sb.WriteString(outerDecls.String())

	for _, source := range filePaths {
		meta, ok := g.Meta.Files[source]
		if !ok || meta.Phantom == nil {
			continue
		}
		sourceKey := nodeKey(source, opts.BasePath)
		phantomKey := sourceKey + "::tests"
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
				phantomLabel = fmt.Sprintf("%s\n%s", phantomLabel, strings.Join(parts, " "))
			}
		}
		sb.WriteString(fmt.Sprintf("  %q [label=%q, style=\"filled,dotted\", fillcolor=lightgreen, color=darkgreen];\n", phantomKey, phantomLabel))
		sb.WriteString(fmt.Sprintf("  %q -> %q [style=dotted, color=darkgreen, arrowsize=1.2, penwidth=1.4];\n", phantomKey, sourceKey))
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

		sourceNodeKey := nodeKey(source, opts.BasePath)
		for _, dep := range sortedDeps {
			depNodeKey := nodeKey(dep, opts.BasePath)
			edgeMD := g.Meta.Edges[depgraph.FileEdge{From: source, To: dep}]

			var edgeAttrs []string
			if edgeMD.State == depgraph.EdgeStateDeleted {
				edgeAttrs = append(edgeAttrs, "color=\"#cc3333\"", "style=dashed", "fontcolor=\"#7a0000\"")
			}
			if edgeMD.State == depgraph.EdgeStateRenamed {
				edgeAttrs = append(edgeAttrs, "color=\"#cc8800\"", "style=dashed")
			}
			if edgeMD.InCycle {
				edgeAttrs = append(edgeAttrs, "color=red", "style=dashed")
			}

			if opts.EdgeLabels {
				// One arrow per underlying dependency: a collapsed module edge
				// keeps a distinct labeled arrow for each original edge it
				// represents, so labels are unchanged by collapsing.
				for _, label := range edgeLabels(g, source, dep, opts.BasePath) {
					attrs := append([]string{fmt.Sprintf("label=%q", label)}, edgeAttrs...)
					sb.WriteString(fmt.Sprintf("  %q -> %q [%s];\n", sourceNodeKey, depNodeKey, strings.Join(attrs, ", ")))
				}
				continue
			}

			if len(edgeAttrs) > 0 {
				sb.WriteString(fmt.Sprintf("  %q -> %q [%s];\n", sourceNodeKey, depNodeKey, strings.Join(edgeAttrs, ", ")))
			} else {
				sb.WriteString(fmt.Sprintf("  %q -> %q;\n", sourceNodeKey, depNodeKey))
			}
		}
	}

	return r.Finish()
}

// dotRenderer emits Graphviz DOT syntax for Scene primitives. It holds only the
// output builder and trailing-newline flag; all graph derivation lives in the
// Scene.
type dotRenderer struct {
	sb       *strings.Builder
	explicit bool
}

func (r *dotRenderer) Begin(h GraphHeader) {
	r.sb.WriteString("digraph dependencies {\n")
	r.sb.WriteString(fmt.Sprintf("  rankdir=%s;\n", h.Orientation.String()))
	r.sb.WriteString("  node [shape=box];\n")

	if h.Title != "" {
		r.sb.WriteString(fmt.Sprintf("  label=\"%s\";\n", h.Title))
		r.sb.WriteString("  labelloc=t;\n")
		r.sb.WriteString("  labeljust=l;\n")
		r.sb.WriteString("  fontsize=10;\n")
		r.sb.WriteString("  fontname=Courier;\n")
	}
	r.sb.WriteString("\n")
}

func (r *dotRenderer) Finish() (string, error) {
	r.sb.WriteString("}")
	if r.explicit {
		r.sb.WriteString("\n")
	}
	return r.sb.String(), nil
}

// GenerateURL creates a GraphvizOnline URL with the DOT graph embedded.
func (f *dotFormatter) GenerateURL(output string) (string, bool) {
	encoded := url.PathEscape(output)
	return fmt.Sprintf("https://dreampuf.github.io/GraphvizOnline/?engine=dot#%s", encoded), true
}

func (f *dotFormatter) assignExtensionColors(filePaths []string) map[string]string {
	if f.extensionColors == nil {
		f.extensionColors = make(map[string]string)
	}

	uniqueExtensions := make(map[string]bool)
	for _, filePath := range filePaths {
		uniqueExtensions[fileTypeKey(filePath)] = true
	}

	sortedExtensions := make([]string, 0, len(uniqueExtensions))
	for ext := range uniqueExtensions {
		sortedExtensions = append(sortedExtensions, ext)
	}
	sort.Strings(sortedExtensions)

	for _, ext := range sortedExtensions {
		if _, exists := f.extensionColors[ext]; exists {
			continue
		}
		color := extensionColorPalette[f.nextColorPaletteI%len(extensionColorPalette)]
		f.extensionColors[ext] = color
		f.nextColorPaletteI++
	}

	currentExtensions := make(map[string]string, len(sortedExtensions))
	for _, ext := range sortedExtensions {
		currentExtensions[ext] = f.extensionColors[ext]
	}
	return currentExtensions
}
