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
	modules := Modules()
	languages := make([]LanguageSupport, len(modules))
	for i, module := range modules {
		languages[i] = LanguageSupport{
			Name:       module.Name(),
			Extensions: append([]string(nil), module.Extensions()...),
			Maturity:   module.Maturity(),
		}
	}
	return languages
}

// SupportedLanguageExtensions returns all supported language extensions in sorted order.
func SupportedLanguageExtensions() []string {
	extensions := make(map[string]bool)
	for _, module := range modules {
		for _, ext := range module.Extensions() {
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
	_, ok := ModuleForExtension(ext)
	return ok
}
