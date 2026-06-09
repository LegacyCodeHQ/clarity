package kotlin

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

type Provider struct{}

func (Provider) Name() string {
	return "Kotlin"
}

func (Provider) Extensions() []string {
	return []string{".kt", ".kts"}
}

func (Provider) Maturity() moduleapi.MaturityLevel {
	return moduleapi.MaturityBasicTests
}

func (Provider) NewResolver(ctx *moduleapi.Context, contentReader vcs.ContentReader) moduleapi.Resolver {
	kotlinFiles := append(append([]string(nil), ctx.FilesByExtension[".kt"]...), ctx.FilesByExtension[".kts"]...)
	packageIndex, packageTypes, filePackages := BuildKotlinIndices(kotlinFiles, contentReader)
	return resolver{
		ctx:           ctx,
		contentReader: contentReader,
		packageIndex:  packageIndex,
		packageTypes:  packageTypes,
		filePackages:  filePackages,
	}
}

func (Provider) IsTestFile(filePath string, _ vcs.ContentReader) bool {
	return IsTestFile(filePath)
}

type resolver struct {
	ctx           *moduleapi.Context
	contentReader vcs.ContentReader
	packageIndex  map[string][]string
	packageTypes  map[string]map[string][]string
	filePackages  map[string]string
}

func (r resolver) ResolveProjectImports(absPath, filePath, _ string) ([]string, error) {
	return ResolveKotlinProjectImports(
		absPath,
		filePath,
		r.packageIndex,
		r.packageTypes,
		r.filePackages,
		r.ctx.SuppliedFiles,
		r.contentReader)
}

func (resolver) FinalizeGraph(_ moduleapi.Graph) error {
	return nil
}
