package ruby

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type Provider struct{}

func (Provider) Name() string {
	return "Ruby"
}

func (Provider) Extensions() []string {
	return []string{".rb"}
}

func (Provider) Maturity() moduleapi.MaturityLevel {
	return moduleapi.MaturityBasicTests
}

func (Provider) NewResolver(ctx *moduleapi.Context, contentReader vcs.ContentReader) moduleapi.Resolver {
	return resolver{ctx: ctx, contentReader: contentReader}
}

func (Provider) IsTestFile(filePath string, _ vcs.ContentReader) bool {
	return IsTestFile(filePath)
}

type resolver struct {
	ctx           *moduleapi.Context
	contentReader vcs.ContentReader
}

func (r resolver) ResolveProjectImports(absPath, filePath, ext string) ([]string, error) {
	return ResolveRubyProjectImports(absPath, filePath, r.ctx.SuppliedFiles, r.contentReader)
}

func (resolver) FinalizeGraph(_ moduleapi.Graph) error {
	return nil
}
