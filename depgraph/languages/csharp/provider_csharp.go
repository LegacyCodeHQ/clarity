package csharp

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type Provider struct{}

func (Provider) Name() string {
	return "C#"
}

func (Provider) Extensions() []string {
	return []string{".cs"}
}

func (Provider) Maturity() moduleapi.MaturityLevel {
	return moduleapi.MaturityBasicTests
}

func (Provider) NewResolver(ctx *moduleapi.Context, contentReader vcs.ContentReader) moduleapi.Resolver {
	namespaceToFiles, namespaceToTypes, fileToNamespace, fileToScope := BuildCSharpIndices(ctx.SuppliedFiles, contentReader)
	return resolver{
		ctx:              ctx,
		contentReader:    contentReader,
		namespaceToFiles: namespaceToFiles,
		namespaceToTypes: namespaceToTypes,
		fileToNamespace:  fileToNamespace,
		fileToScope:      fileToScope,
	}
}

func (Provider) IsTestFile(filePath string, _ vcs.ContentReader) bool {
	return IsTestFile(filePath)
}

type resolver struct {
	ctx              *moduleapi.Context
	contentReader    vcs.ContentReader
	namespaceToFiles map[string][]string
	namespaceToTypes map[string]map[string][]string
	fileToNamespace  map[string]string
	fileToScope      map[string]string
}

func (r resolver) ResolveProjectImports(absPath, filePath, ext string) ([]string, error) {
	return ResolveCSharpProjectImports(
		absPath,
		filePath,
		r.namespaceToFiles,
		r.namespaceToTypes,
		r.fileToNamespace,
		r.fileToScope,
		r.ctx.SuppliedFiles,
		r.contentReader)
}

func (resolver) FinalizeGraph(_ moduleapi.Graph) error {
	return nil
}
