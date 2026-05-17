package zig

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type Module struct{}

func (Module) Name() string {
	return "Zig"
}

func (Module) Extensions() []string {
	return []string{".zig"}
}

func (Module) Maturity() moduleapi.MaturityLevel {
	return moduleapi.MaturityBasicTests
}

func (Module) NewResolver(ctx *moduleapi.Context, contentReader vcs.ContentReader) moduleapi.Resolver {
	return resolver{ctx: ctx, contentReader: contentReader}
}

func (Module) IsTestFile(filePath string, contentReader vcs.ContentReader) bool {
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
