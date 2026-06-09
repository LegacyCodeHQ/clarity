package moduleapi

import graphlib "github.com/dominikbraun/graph"

// Graph is the minimal graph contract language resolvers need during finalization.
type Graph interface {
	Vertex(hash string) (string, error)
	AddEdge(sourceHash, targetHash string, options ...func(*graphlib.EdgeProperties)) error
}
