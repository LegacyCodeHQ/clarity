package zig

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type Provider struct{}

func (Provider) Name() string {
	return "Zig"
}

func (Provider) Extensions() []string {
	return []string{".zig"}
}

func (Provider) Maturity() moduleapi.MaturityLevel {
	return moduleapi.MaturityBasicTests
}

func (Provider) NewResolver(ctx *moduleapi.Context, contentReader vcs.ContentReader) moduleapi.Resolver {
	return resolver{ctx: ctx, contentReader: contentReader}
}

func (Provider) IsTestFile(filePath string, contentReader vcs.ContentReader) bool {
	return IsTestFileWithContent(filePath, contentReader)
}

type resolver struct {
	ctx           *moduleapi.Context
	contentReader vcs.ContentReader
}

func (r resolver) ResolveProjectImports(absPath, filePath, ext string) ([]string, error) {
	return ResolveZigProjectImports(absPath, filePath, ext, r.ctx.SuppliedFiles, r.contentReader)
}

func (resolver) FinalizeGraph(_ moduleapi.Graph) error {
	return nil
}
