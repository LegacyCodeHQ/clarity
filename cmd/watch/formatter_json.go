package watch

import (
	"encoding/json"
	"sort"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph"
)

type jsonGraphFormatter struct{}

type jsonGraphOutput struct {
	Label  string           `json:"label,omitempty"`
	Nodes  []jsonGraphNode  `json:"nodes"`
	Edges  []jsonGraphEdge  `json:"edges"`
	Cycles []jsonGraphCycle `json:"cycles"`
}

type jsonGraphNode struct {
	Path       string           `json:"path"`
	Name       string           `json:"name"`
	Attributes []string         `json:"attributes,omitempty"`
	Stats      *jsonNodeStats   `json:"stats,omitempty"`
	Phantom    *jsonPhantomNode `json:"phantom,omitempty"`
}

type jsonNodeStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

type jsonPhantomNode struct {
	Kind        string         `json:"kind"`
	Stats       *jsonNodeStats `json:"stats,omitempty"`
	ProdChanged bool           `json:"prodChanged"`
	IsNew       bool           `json:"isNew,omitempty"`
}

type jsonGraphEdge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	InCycle bool   `json:"inCycle"`
}

type jsonGraphCycle struct {
	Path []string `json:"path"`
}

// Format converts the dependency graph to JSON for internal watch server communication.
func (f jsonGraphFormatter) Format(g depgraph.FileDependencyGraph, label string) (string, error) {
	adjacency, err := depgraph.AdjacencyList(g.Graph)
	if err != nil {
		return "", err
	}

	filePaths := make([]string, 0, len(adjacency))
	for path := range adjacency {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)
	nodeNames := formatters.BuildNodeNames(filePaths)

	nodes := make([]jsonGraphNode, 0, len(filePaths))
	for _, path := range filePaths {
		fileMetadata := g.Meta.Files[path]
		node := jsonGraphNode{
			Path: path,
			Name: nodeNames[path],
		}
		if fileMetadata.IsTest {
			node.Attributes = append(node.Attributes, "test")
		}
		if fileMetadata.Stats != nil {
			node.Stats = &jsonNodeStats{
				Additions: fileMetadata.Stats.Additions,
				Deletions: fileMetadata.Stats.Deletions,
			}
			if fileMetadata.Stats.IsNew {
				node.Attributes = append(node.Attributes, "new")
			}
		}
		if fileMetadata.Phantom != nil {
			phantom := &jsonPhantomNode{
				Kind:        fileMetadata.Phantom.Kind,
				ProdChanged: fileMetadata.Phantom.ProdChanged,
			}
			if fileMetadata.Phantom.Stats != nil {
				phantom.Stats = &jsonNodeStats{
					Additions: fileMetadata.Phantom.Stats.Additions,
					Deletions: fileMetadata.Phantom.Stats.Deletions,
				}
				phantom.IsNew = fileMetadata.Phantom.Stats.IsNew
			}
			node.Phantom = phantom
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
		Label:  label,
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
