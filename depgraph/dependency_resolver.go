package depgraph

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

// DependencyResolver resolves project imports per file and can finalize graph-wide dependencies.
type DependencyResolver interface {
	SupportsFileExtension(ext string) bool
	ResolveProjectImports(absPath, filePath, ext string) ([]string, error)
	FinalizeGraph(graph DependencyGraph) error
}

type defaultDependencyResolver struct {
	extensionResolvers map[string]moduleapi.Resolver
	resolvers          []moduleapi.Resolver
}

// NewDefaultDependencyResolver creates the built-in language-aware dependency resolver.
func NewDefaultDependencyResolver(ctx *dependencyGraphContext, contentReader vcs.ContentReader) DependencyResolver {
	resolver := &defaultDependencyResolver{
		extensionResolvers: make(map[string]moduleapi.Resolver),
	}

	for _, module := range registry.Modules() {
		moduleResolver := module.NewResolver(ctx, contentReader)
		if moduleResolver == nil {
			continue
		}

		resolver.resolvers = append(resolver.resolvers, moduleResolver)
		for _, ext := range module.Extensions() {
			resolver.extensionResolvers[ext] = moduleResolver
		}
	}

	return resolver
}

func (b *defaultDependencyResolver) SupportsFileExtension(ext string) bool {
	_, ok := b.extensionResolvers[ext]
	return ok
}

func (b *defaultDependencyResolver) ResolveProjectImports(absPath, filePath, ext string) ([]string, error) {
	resolver, ok := b.extensionResolvers[ext]
	if !ok {
		return []string{}, nil
	}

	return resolver.ResolveProjectImports(absPath, filePath, ext)
}

func (b *defaultDependencyResolver) FinalizeGraph(graph DependencyGraph) error {
	for _, resolver := range b.resolvers {
		if err := resolver.FinalizeGraph(graph); err != nil {
			return err
		}
	}

	return nil
}
