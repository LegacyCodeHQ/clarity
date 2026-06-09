package registry

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/c"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/cpp"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/csharp"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/dart"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/golang"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/java"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/javascript"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/kotlin"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/markdown"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/python"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/ruby"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/rust"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/scala"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/svelte"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/swift"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/typescript"
	"github.com/LegacyCodeHQ/clarity/depgraph/languages/zig"
	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
)

var providers = []moduleapi.Provider{
	c.Provider{},
	cpp.Provider{},
	csharp.Provider{},
	dart.Provider{},
	golang.Provider{},
	javascript.Provider{},
	java.Provider{},
	kotlin.Provider{},
	markdown.Provider{},
	python.Provider{},
	ruby.Provider{},
	rust.Provider{},
	scala.Provider{},
	svelte.Provider{},
	swift.Provider{},
	typescript.Provider{},
	zig.Provider{},
}

// Providers returns supported language providers in deterministic order.
func Providers() []moduleapi.Provider {
	return append([]moduleapi.Provider(nil), providers...)
}

// ProviderForExtension returns the provider registered for the provided extension.
func ProviderForExtension(ext string) (moduleapi.Provider, bool) {
	for _, provider := range providers {
		for _, providerExt := range provider.Extensions() {
			if providerExt == ext {
				return provider, true
			}
		}
	}

	return nil, false
}
