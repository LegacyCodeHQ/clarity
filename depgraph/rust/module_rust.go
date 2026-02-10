package rust

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/langsupport"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type Module struct{}

func (Module) Name() string {
	return "Rust"
}

func (Module) Extensions() []string {
	return []string{".rs"}
}

func (Module) Maturity() langsupport.MaturityLevel {
	return langsupport.MaturityBasicTests
}

func (Module) NewResolver(ctx *langsupport.Context, contentReader vcs.ContentReader) langsupport.Resolver {
	return resolver{ctx: ctx, contentReader: contentReader}
}

func (Module) IsTestFile(filePath string, contentReader vcs.ContentReader) bool {
	return IsTestFileWithContent(filePath, contentReader)
}

type resolver struct {
	ctx           *langsupport.Context
	contentReader vcs.ContentReader
}

func (r resolver) ResolveProjectImports(absPath, filePath, ext string) ([]string, error) {
	return ResolveRustProjectImports(absPath, filePath, r.ctx.SuppliedFiles, r.contentReader)
}

func (resolver) FinalizeGraph(_ langsupport.Graph) error {
	return nil
}
