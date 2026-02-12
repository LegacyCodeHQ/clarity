package formatters

import (
	"encoding/json"
	"sort"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

type jsonGraphOutput struct {
	Label  string           `json:"label,omitempty"`
	Nodes  []jsonGraphNode  `json:"nodes"`
	Edges  []jsonGraphEdge  `json:"edges"`
	Cycles []jsonGraphCycle `json:"cycles"`
}

type jsonGraphNode struct {
	Path      string         `json:"path"`
	Name      string         `json:"name"`
	IsTest    bool           `json:"isTest"`
	Extension string         `json:"extension"`
	Stats     *jsonNodeStats `json:"stats,omitempty"`
}

type jsonNodeStats struct {
	Additions int  `json:"additions"`
	Deletions int  `json:"deletions"`
	IsNew     bool `json:"isNew"`
}

type jsonGraphEdge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	InCycle bool   `json:"inCycle"`
}

type jsonGraphCycle struct {
	Path []string `json:"path"`
}

// Format converts the dependency graph to JSON.
func (f jsonFormatter) Format(g depgraph.FileDependencyGraph, opts RenderOptions) (string, error) {
	adjacency, err := depgraph.AdjacencyList(g.Graph)
	if err != nil {
		return "", err
	}

	filePaths := make([]string, 0, len(adjacency))
	for path := range adjacency {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)
	nodeNames := BuildNodeNames(filePaths)

	nodes := make([]jsonGraphNode, 0, len(filePaths))
	for _, path := range filePaths {
		fileMetadata := g.Meta.Files[path]
		node := jsonGraphNode{
			Path:      path,
			Name:      nodeNames[path],
			IsTest:    fileMetadata.IsTest,
			Extension: fileMetadata.Extension,
		}
		if fileMetadata.Stats != nil {
			node.Stats = &jsonNodeStats{
				Additions: fileMetadata.Stats.Additions,
				Deletions: fileMetadata.Stats.Deletions,
				IsNew:     fileMetadata.Stats.IsNew,
			}
		}
		nodes = append(nodes, node)
	}

	edges := []jsonGraphEdge{}
	for _, source := range filePaths {
		deps := append([]string(nil), adjacency[source]...)
		sort.Strings(deps)
		for _, dep := range deps {
			edgeMetadata := g.Meta.Edges[depgraph.FileEdge{From: source, To: dep}]
			edges = append(edges, jsonGraphEdge{
				From:    source,
				To:      dep,
				InCycle: edgeMetadata.InCycle,
			})
		}
	}

	cycles := make([]jsonGraphCycle, 0, len(g.Meta.Cycles))
	for _, cycle := range g.Meta.Cycles {
		cyclePath := append([]string(nil), cycle.Path...)
		cycles = append(cycles, jsonGraphCycle{Path: cyclePath})
	}

	output := jsonGraphOutput{
		Label:  opts.Label,
		Nodes:  nodes,
		Edges:  edges,
		Cycles: cycles,
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// GenerateURL reports that URL generation is not supported for JSON output.
func (f jsonFormatter) GenerateURL(output string) (string, bool) {
	return "", false
}
