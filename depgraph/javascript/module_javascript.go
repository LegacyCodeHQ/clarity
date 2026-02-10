package javascript

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/langsupport"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type Module struct{}

func (Module) Name() string {
	return "JavaScript"
}

func (Module) Extensions() []string {
	return []string{".js", ".jsx"}
}

func (Module) Maturity() langsupport.MaturityLevel {
	return langsupport.MaturityBasicTests
}

func (Module) NewResolver(ctx *langsupport.Context, contentReader vcs.ContentReader) langsupport.Resolver {
	return resolver{ctx: ctx, contentReader: contentReader}
}

func (Module) IsTestFile(filePath string, _ vcs.ContentReader) bool {
	return IsTestFile(filePath)
}

type resolver struct {
	ctx           *langsupport.Context
	contentReader vcs.ContentReader
}

func (r resolver) ResolveProjectImports(absPath, filePath, ext string) ([]string, error) {
	return ResolveJavaScriptProjectImports(absPath, filePath, ext, r.ctx.SuppliedFiles, r.contentReader)
}

func (resolver) FinalizeGraph(_ langsupport.Graph) error {
	return nil
}
