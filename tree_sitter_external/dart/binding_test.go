package dart_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/tree_sitter_external/dart"
)

func TestCanLoadGrammar(t *testing.T) {
	language := dart.GetLanguage()
	if language == nil {
		t.Errorf("Error loading Dart grammar")
	}
}
