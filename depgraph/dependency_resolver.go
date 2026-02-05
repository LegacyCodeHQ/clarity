package depgraph

import (
	"github.com/LegacyCodeHQ/sanity/depgraph/dart"
	"github.com/LegacyCodeHQ/sanity/depgraph/golang"
	"github.com/LegacyCodeHQ/sanity/depgraph/kotlin"
	"github.com/LegacyCodeHQ/sanity/depgraph/typescript"
	"github.com/LegacyCodeHQ/sanity/vcs"
)

// DependencyResolver resolves project imports per file and can finalize graph-wide dependencies.
type DependencyResolver interface {
	SupportsFileExtension(ext string) bool
	ResolveProjectImports(absPath, filePath, ext string) ([]string, error)
	FinalizeGraph(graph DependencyGraph) error
}

type importResolverFunc func(absPath, filePath, ext string) ([]string, error)

type defaultDependencyResolver struct {
	ctx                *dependencyGraphContext
	contentReader      vcs.ContentReader
	goImportResolver   *golang.ProjectImportResolver
	kotlinPackageIndex map[string][]string
	kotlinPackageTypes map[string]map[string][]string
	kotlinFilePackages map[string]string
	importResolvers    map[string]importResolverFunc
}

// NewDefaultDependencyResolver creates the built-in language-aware dependency resolver.
func NewDefaultDependencyResolver(ctx *dependencyGraphContext, contentReader vcs.ContentReader) DependencyResolver {
	goImportResolver := golang.NewProjectImportResolver(ctx.dirToFiles, ctx.suppliedFiles, contentReader)
	kotlinPackageIndex, kotlinPackageTypes, kotlinFilePackages := kotlin.BuildKotlinIndices(ctx.kotlinFiles, contentReader)

	resolver := &defaultDependencyResolver{
		ctx:                ctx,
		contentReader:      contentReader,
		goImportResolver:   goImportResolver,
		kotlinPackageIndex: kotlinPackageIndex,
		kotlinPackageTypes: kotlinPackageTypes,
		kotlinFilePackages: kotlinFilePackages,
	}

	resolver.importResolvers = map[string]importResolverFunc{
		".dart": resolver.resolveDartImports,
		".go":   resolver.resolveGoImports,
		".kt":   resolver.resolveKotlinImports,
		".ts":   resolver.resolveTypeScriptImports,
		".tsx":  resolver.resolveTypeScriptImports,
	}

	return resolver
}

func (b *defaultDependencyResolver) SupportsFileExtension(ext string) bool {
	_, ok := b.importResolvers[ext]
	return ok
}

func (b *defaultDependencyResolver) ResolveProjectImports(absPath, filePath, ext string) ([]string, error) {
	resolveImports, ok := b.importResolvers[ext]
	if !ok {
		return []string{}, nil
	}

	return resolveImports(absPath, filePath, ext)
}

func (b *defaultDependencyResolver) resolveDartImports(absPath, filePath, ext string) ([]string, error) {
	return dart.ResolveDartProjectImports(absPath, filePath, ext, b.ctx.suppliedFiles, b.contentReader)
}

func (b *defaultDependencyResolver) resolveGoImports(absPath, filePath, _ string) ([]string, error) {
	return b.goImportResolver.ResolveProjectImports(absPath, filePath)
}

func (b *defaultDependencyResolver) resolveKotlinImports(absPath, filePath, _ string) ([]string, error) {
	return kotlin.ResolveKotlinProjectImports(
		absPath,
		filePath,
		b.kotlinPackageIndex,
		b.kotlinPackageTypes,
		b.kotlinFilePackages,
		b.ctx.suppliedFiles,
		b.contentReader)
}

func (b *defaultDependencyResolver) resolveTypeScriptImports(absPath, filePath, ext string) ([]string, error) {
	return typescript.ResolveTypeScriptProjectImports(absPath, filePath, ext, b.ctx.suppliedFiles, b.contentReader)
}

func (b *defaultDependencyResolver) FinalizeGraph(graph DependencyGraph) error {
	return golang.AddGoIntraPackageDependencies(graph, b.ctx.goFiles, b.contentReader)
}
