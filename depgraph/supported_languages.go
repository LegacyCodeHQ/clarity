package depgraph

import (
	"sort"

	"github.com/LegacyCodeHQ/clarity/depgraph/langsupport"
)

// LanguageSupport describes one supported programming language and
// the file extensions that map to it.
type LanguageSupport struct {
	Name       string
	Extensions []string
	Maturity   langsupport.MaturityLevel
}

var supportedLanguageExtensions = buildSupportedLanguageExtensions()

func buildSupportedLanguageExtensions() map[string]bool {
	extensions := make(map[string]bool)
	for _, language := range languageRegistry {
		for _, ext := range language.Module.Extensions() {
			extensions[ext] = true
		}
	}
	return extensions
}

// SupportedLanguages returns a copy of all supported languages and their extensions.
func SupportedLanguages() []LanguageSupport {
	languages := make([]LanguageSupport, len(languageRegistry))
	for i, language := range languageRegistry {
		languages[i] = LanguageSupport{
			Name:       language.Module.Name(),
			Extensions: append([]string(nil), language.Module.Extensions()...),
			Maturity:   language.Module.Maturity(),
		}
	}
	return languages
}

// IsSupportedLanguageExtension reports whether Clarity can analyze files with the extension.
func IsSupportedLanguageExtension(ext string) bool {
	return supportedLanguageExtensions[ext]
}

// SupportedLanguageExtensions returns all supported language extensions in sorted order.
func SupportedLanguageExtensions() []string {
	extensions := make([]string, 0, len(supportedLanguageExtensions))
	for ext := range supportedLanguageExtensions {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)
	return extensions
}
