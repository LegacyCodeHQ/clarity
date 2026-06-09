package registry

import (
	"sort"

	"github.com/LegacyCodeHQ/clarity/depgraph/moduleapi"
)

// LanguageSupport describes one supported programming language and
// the file extensions that map to it.
type LanguageSupport struct {
	Name       string
	Extensions []string
	Maturity   moduleapi.MaturityLevel
}

// SupportedLanguages returns a copy of all supported languages and their extensions.
func SupportedLanguages() []LanguageSupport {
	providers := Providers()
	languages := make([]LanguageSupport, len(providers))
	for i, provider := range providers {
		languages[i] = LanguageSupport{
			Name:       provider.Name(),
			Extensions: append([]string(nil), provider.Extensions()...),
			Maturity:   provider.Maturity(),
		}
	}
	return languages
}

// SupportedLanguageExtensions returns all supported language extensions in sorted order.
func SupportedLanguageExtensions() []string {
	extensions := make(map[string]bool)
	for _, provider := range providers {
		for _, ext := range provider.Extensions() {
			extensions[ext] = true
		}
	}

	result := make([]string, 0, len(extensions))
	for ext := range extensions {
		result = append(result, ext)
	}
	sort.Strings(result)
	return result
}

// IsSupportedLanguageExtension reports whether Clarity can analyze files with the extension.
func IsSupportedLanguageExtension(ext string) bool {
	_, ok := ProviderForExtension(ext)
	return ok
}
