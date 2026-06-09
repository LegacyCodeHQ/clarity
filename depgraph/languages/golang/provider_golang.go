package golang

import (
	"errors"

	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/vcs"
	graphlib "github.com/dominikbraun/graph"
)

type Provider struct{}

func (Provider) Name() string {
	return "Go"
}

func (Provider) Extensions() []string {
	return []string{".go"}
}

func (Provider) Maturity() moduleapi.MaturityLevel {
	return moduleapi.MaturityActivelyTested
}

func (Provider) NewResolver(ctx *moduleapi.Context, contentReader vcs.ContentReader) moduleapi.Resolver {
	return resolver{
		ctx:             ctx,
		contentReader:   contentReader,
		projectResolver: NewProjectImportResolver(ctx.DirToFiles, ctx.SuppliedFiles, contentReader),
	}
}

func (Provider) IsTestFile(filePath string, _ vcs.ContentReader) bool {
	return IsTestFile(filePath)
}

type resolver struct {
	ctx             *moduleapi.Context
	contentReader   vcs.ContentReader
	projectResolver *ProjectImportResolver
}

func (r resolver) ResolveProjectImports(absPath, filePath, _ string) ([]string, error) {
	return r.projectResolver.ResolveProjectImports(absPath, filePath)
}

func (r resolver) FinalizeGraph(graph moduleapi.Graph) error {
	return addGoIntraPackageDependencies(graph, r.ctx.FilesByExtension[".go"], r.contentReader, r.projectResolver)
}

func addGoIntraPackageDependencies(
	graph moduleapi.Graph,
	goFiles []string,
	contentReader vcs.ContentReader,
	projectResolver *ProjectImportResolver,
) error {
	if len(goFiles) == 0 {
		return nil
	}

	var symbolLookup func(filePath string) (*GoSymbolInfo, bool)
	if projectResolver != nil {
		symbolLookup = projectResolver.getSymbolInfo
	}

	intraDeps, err := BuildIntraPackageDependenciesWithSymbolLookup(goFiles, vcs.ContentReader(contentReader), symbolLookup)
	if err != nil {
		return err
	}

	for file, deps := range intraDeps {
		if _, err := graph.Vertex(file); err != nil {
			continue
		}
		for _, dep := range deps {
			if _, err := graph.Vertex(dep); err != nil {
				continue
			}
			if err := graph.AddEdge(file, dep); err != nil && !errors.Is(err, graphlib.ErrEdgeAlreadyExists) {
				return err
			}
		}
	}

	return nil
}
