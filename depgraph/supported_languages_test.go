package depgraph

import "testing"

func TestSupportedLanguages(t *testing.T) {
	languages := SupportedLanguages()
	if len(languages) == 0 {
		t.Fatalf("SupportedLanguages() returned no languages")
	}

	foundC := false
	foundCpp := false
	foundCSharp := false
	foundJavaScript := false
	foundPython := false
	foundRust := false
	foundSwift := false
	foundTypeScript := false
	for _, language := range languages {
		switch language.Name {
		case "C":
			foundC = true
			if len(language.Extensions) != 2 {
				t.Fatalf("C extension count = %d, want 2", len(language.Extensions))
			}
		case "C++":
			foundCpp = true
			if len(language.Extensions) != 6 {
				t.Fatalf("C++ extension count = %d, want 6", len(language.Extensions))
			}
		case "C#":
			foundCSharp = true
			if len(language.Extensions) != 1 {
				t.Fatalf("C# extension count = %d, want 1", len(language.Extensions))
			}
		case "JavaScript":
			foundJavaScript = true
			if len(language.Extensions) != 2 {
				t.Fatalf("JavaScript extension count = %d, want 2", len(language.Extensions))
			}
		case "Python":
			foundPython = true
			if len(language.Extensions) != 1 {
				t.Fatalf("Python extension count = %d, want 1", len(language.Extensions))
			}
		case "Rust":
			foundRust = true
			if len(language.Extensions) != 1 {
				t.Fatalf("Rust extension count = %d, want 1", len(language.Extensions))
			}
		case "Swift":
			foundSwift = true
			if len(language.Extensions) != 1 {
				t.Fatalf("Swift extension count = %d, want 1", len(language.Extensions))
			}
		case "TypeScript":
			foundTypeScript = true
			if len(language.Extensions) != 2 {
				t.Fatalf("TypeScript extension count = %d, want 2", len(language.Extensions))
			}
		}
	}

	if !foundC {
		t.Fatalf("SupportedLanguages() missing C")
	}
	if !foundCpp {
		t.Fatalf("SupportedLanguages() missing C++")
	}
	if !foundCSharp {
		t.Fatalf("SupportedLanguages() missing C#")
	}
	if !foundJavaScript {
		t.Fatalf("SupportedLanguages() missing JavaScript")
	}
	if !foundPython {
		t.Fatalf("SupportedLanguages() missing Python")
	}
	if !foundRust {
		t.Fatalf("SupportedLanguages() missing Rust")
	}
	if !foundSwift {
		t.Fatalf("SupportedLanguages() missing Swift")
	}
	if !foundTypeScript {
		t.Fatalf("SupportedLanguages() missing TypeScript")
	}
}

func TestIsSupportedLanguageExtension(t *testing.T) {
	if !IsSupportedLanguageExtension(".c") {
		t.Fatalf("IsSupportedLanguageExtension(.c) = false, want true")
	}
	if !IsSupportedLanguageExtension(".cpp") {
		t.Fatalf("IsSupportedLanguageExtension(.cpp) = false, want true")
	}
	if !IsSupportedLanguageExtension(".cs") {
		t.Fatalf("IsSupportedLanguageExtension(.cs) = false, want true")
	}
	if !IsSupportedLanguageExtension(".go") {
		t.Fatalf("IsSupportedLanguageExtension(.go) = false, want true")
	}
	if !IsSupportedLanguageExtension(".js") {
		t.Fatalf("IsSupportedLanguageExtension(.js) = false, want true")
	}
	if !IsSupportedLanguageExtension(".jsx") {
		t.Fatalf("IsSupportedLanguageExtension(.jsx) = false, want true")
	}
	if !IsSupportedLanguageExtension(".py") {
		t.Fatalf("IsSupportedLanguageExtension(.py) = false, want true")
	}
	if !IsSupportedLanguageExtension(".rs") {
		t.Fatalf("IsSupportedLanguageExtension(.rs) = false, want true")
	}
	if !IsSupportedLanguageExtension(".swift") {
		t.Fatalf("IsSupportedLanguageExtension(.swift) = false, want true")
	}
	if !IsSupportedLanguageExtension(".kts") {
		t.Fatalf("IsSupportedLanguageExtension(.kts) = false, want true")
	}
	if IsSupportedLanguageExtension(".md") {
		t.Fatalf("IsSupportedLanguageExtension(.md) = true, want false")
	}
}
