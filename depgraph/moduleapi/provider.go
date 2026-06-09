package moduleapi

import "github.com/LegacyCodeHQ/clarity/vcs"

// Provider describes pluggable language support.
type Provider interface {
	Name() string
	Extensions() []string
	Maturity() MaturityLevel
	NewResolver(ctx *Context, contentReader vcs.ContentReader) Resolver
	IsTestFile(filePath string, contentReader vcs.ContentReader) bool
}
