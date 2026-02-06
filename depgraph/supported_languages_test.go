package depgraph

import "testing"

func TestSupportedLanguages(t *testing.T) {
	languages := SupportedLanguages()
	if len(languages) == 0 {
		t.Fatalf("SupportedLanguages() returned no languages")
	}

	foundJavaScript := false
	foundTypeScript := false
	for _, language := range languages {
		switch language.Name {
		case "JavaScript":
			foundJavaScript = true
			if len(language.Extensions) != 2 {
				t.Fatalf("JavaScript extension count = %d, want 2", len(language.Extensions))
			}
		case "TypeScript":
			foundTypeScript = true
			if len(language.Extensions) != 2 {
				t.Fatalf("TypeScript extension count = %d, want 2", len(language.Extensions))
			}
		}
	}

	if !foundJavaScript {
		t.Fatalf("SupportedLanguages() missing JavaScript")
	}
	if !foundTypeScript {
		t.Fatalf("SupportedLanguages() missing TypeScript")
	}
}

func TestIsSupportedLanguageExtension(t *testing.T) {
	if !IsSupportedLanguageExtension(".go") {
		t.Fatalf("IsSupportedLanguageExtension(.go) = false, want true")
	}
	if !IsSupportedLanguageExtension(".js") {
		t.Fatalf("IsSupportedLanguageExtension(.js) = false, want true")
	}
	if !IsSupportedLanguageExtension(".jsx") {
		t.Fatalf("IsSupportedLanguageExtension(.jsx) = false, want true")
	}
	if !IsSupportedLanguageExtension(".kts") {
		t.Fatalf("IsSupportedLanguageExtension(.kts) = false, want true")
	}
	if IsSupportedLanguageExtension(".md") {
		t.Fatalf("IsSupportedLanguageExtension(.md) = true, want false")
	}
}
