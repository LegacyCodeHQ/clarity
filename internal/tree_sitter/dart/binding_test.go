package dart_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/internal/tree_sitter/dart"
)

func TestCanLoadGrammar(t *testing.T) {
	language := dart.GetLanguage()
	if language == nil {
		t.Errorf("Error loading Dart grammar")
	}
}
