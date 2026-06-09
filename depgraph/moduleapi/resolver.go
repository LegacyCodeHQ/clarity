package moduleapi

// Resolver resolves project imports for one language and can finalize graph-wide state.
type Resolver interface {
	ResolveProjectImports(absPath, filePath, ext string) ([]string, error)
	FinalizeGraph(graph Graph) error
}
