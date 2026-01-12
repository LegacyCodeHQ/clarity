package tree_sitter_dart_test

import (
	"testing"

	"github.com/LegacyCodeHQ/sanity/tree_sitter_dart"
)

func TestCanLoadGrammar(t *testing.T) {
	language := tree_sitter_dart.GetLanguage()
	if language == nil {
		t.Errorf("Error loading Dart grammar")
	}
}
