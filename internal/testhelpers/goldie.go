package testhelpers

import (
	"testing"

	"github.com/sebdah/goldie/v2"
)

func MermaidGoldie(t *testing.T) *goldie.Goldie {
	return goldieWithExtension(t, "mermaid")
}

func DotGoldie(t *testing.T) *goldie.Goldie {
	return goldieWithExtension(t, "dot")
}

func TextGoldie(t *testing.T) *goldie.Goldie {
	return goldieWithExtension(t, "txt")
}

// goldieWithExtension creates a Goldie instance with a golden file suffix.
func goldieWithExtension(t *testing.T, suffix string) *goldie.Goldie {
	t.Helper()
	return goldie.New(t, goldie.WithNameSuffix(".gold."+suffix))
}
