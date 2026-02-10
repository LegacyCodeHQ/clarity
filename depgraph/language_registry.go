package depgraph

import (
	"github.com/LegacyCodeHQ/clarity/depgraph/c"
	"github.com/LegacyCodeHQ/clarity/depgraph/cpp"
	"github.com/LegacyCodeHQ/clarity/depgraph/csharp"
	"github.com/LegacyCodeHQ/clarity/depgraph/dart"
	"github.com/LegacyCodeHQ/clarity/depgraph/golang"
	"github.com/LegacyCodeHQ/clarity/depgraph/java"
	"github.com/LegacyCodeHQ/clarity/depgraph/javascript"
	"github.com/LegacyCodeHQ/clarity/depgraph/kotlin"
	"github.com/LegacyCodeHQ/clarity/depgraph/langsupport"
	"github.com/LegacyCodeHQ/clarity/depgraph/python"
	"github.com/LegacyCodeHQ/clarity/depgraph/ruby"
	"github.com/LegacyCodeHQ/clarity/depgraph/rust"
	"github.com/LegacyCodeHQ/clarity/depgraph/swift"
	"github.com/LegacyCodeHQ/clarity/depgraph/typescript"
)

type languageRegistryEntry struct {
	Module langsupport.Module
}

// languageRegistry is the single source of truth for supported languages.
// Adding/removing a language should happen here.
var languageRegistry = []languageRegistryEntry{
	{Module: c.Module{}},
	{Module: cpp.Module{}},
	{Module: csharp.Module{}},
	{Module: dart.Module{}},
	{Module: golang.Module{}},
	{Module: javascript.Module{}},
	{Module: java.Module{}},
	{Module: kotlin.Module{}},
	{Module: python.Module{}},
	{Module: ruby.Module{}},
	{Module: rust.Module{}},
	{Module: swift.Module{}},
	{Module: typescript.Module{}},
}

func moduleForExtension(ext string) (langsupport.Module, bool) {
	for _, language := range languageRegistry {
		for _, languageExt := range language.Module.Extensions() {
			if languageExt == ext {
				return language.Module, true
			}
		}
	}

	return nil, false
}
