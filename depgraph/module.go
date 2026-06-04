package depgraph

import "sort"

// Module is a user-named set of files projected onto the dependency graph.
// Files holds graph node keys (the same absolute paths used as graph vertices).
type Module struct {
	Name  string
	Files []string
}

// CollapseModules contracts each module's member files into a single synthetic
// node named after the module. It is a config-driven post-processing transform
// applied to the built graph: edges touching a member are redirected to the
// module node, edges internal to a module (including self-loops) are dropped,
// and duplicate edges are deduplicated. Members absent from the graph are
// ignored; a module with no members in the graph contributes nothing.
//
// Running the transform on the dependency graph (before metadata is computed)
// lets cycles, edges, and stats recompute naturally against the collapsed
// shape, so a module reads as a single opaque node in the rendered graph.
//
// It returns the collapsed graph and the sorted node keys of the modules that
// were actually created (those with at least one member in the graph), so the
// caller can mark them as module nodes for rendering.
func CollapseModules(g DependencyGraph, modules []Module) (DependencyGraph, []string, error) {
	adjacency, err := AdjacencyList(g)
	if err != nil {
		return nil, nil, err
	}

	memberToModule := make(map[string]string)
	moduleNodeSet := make(map[string]bool)
	for _, module := range modules {
		for _, file := range module.Files {
			if _, ok := adjacency[file]; ok {
				memberToModule[file] = module.Name
				moduleNodeSet[module.Name] = true
			}
		}
	}
	if len(memberToModule) == 0 {
		return g, nil, nil
	}

	resolve := func(node string) string {
		if name, ok := memberToModule[node]; ok {
			return name
		}
		return node
	}

	edges := make(map[string]map[string]bool)
	ensure := func(node string) {
		if edges[node] == nil {
			edges[node] = make(map[string]bool)
		}
	}

	for source, deps := range adjacency {
		from := resolve(source)
		ensure(from)
		for _, dep := range deps {
			to := resolve(dep)
			if from == to {
				continue // drop edges internal to a module (and self-loops)
			}
			ensure(to)
			edges[from][to] = true
		}
	}

	collapsed := make(map[string][]string, len(edges))
	for node, targets := range edges {
		deps := make([]string, 0, len(targets))
		for target := range targets {
			deps = append(deps, target)
		}
		sort.Strings(deps)
		collapsed[node] = deps
	}

	moduleNodes := make([]string, 0, len(moduleNodeSet))
	for name := range moduleNodeSet {
		moduleNodes = append(moduleNodes, name)
	}
	sort.Strings(moduleNodes)

	graph, err := NewDependencyGraphFromAdjacency(collapsed)
	if err != nil {
		return nil, nil, err
	}
	return graph, moduleNodes, nil
}
