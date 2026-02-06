package depgraph

import (
	"github.com/LegacyCodeHQ/sanity/depgraph/dart"
	"github.com/LegacyCodeHQ/sanity/depgraph/golang"
	"github.com/LegacyCodeHQ/sanity/depgraph/java"
	"github.com/LegacyCodeHQ/sanity/depgraph/javascript"
	"github.com/LegacyCodeHQ/sanity/depgraph/kotlin"
	"github.com/LegacyCodeHQ/sanity/depgraph/langsupport"
	"github.com/LegacyCodeHQ/sanity/depgraph/python"
	"github.com/LegacyCodeHQ/sanity/depgraph/typescript"
)

type languageRegistryEntry struct {
	Module langsupport.Module
}

// languageRegistry is the single source of truth for supported languages.
// Adding/removing a language should happen here.
var languageRegistry = []languageRegistryEntry{
	{Module: dart.Module{}},
	{Module: golang.Module{}},
	{Module: javascript.Module{}},
	{Module: java.Module{}},
	{Module: kotlin.Module{}},
	{Module: python.Module{}},
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
