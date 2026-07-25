package kotlin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKotlinEvidenceLocations(t *testing.T) {
	source := []byte(`package arena.core

import arena.model.Level as GameLevel

class Generator : GameLevel() {
    fun build(): GameLevel = GameLevel.create()
}
`)

	assert.Contains(t, ParseKotlinDeclarationLocations(source), KotlinDeclarationLocation{
		Name: "Generator", Kind: "class", Package: "arena.core", Line: 5,
	})
	assert.Contains(t, ParseKotlinImportLocations(source), KotlinImportLocation{
		Path: "arena.model.Level", Imported: "Level", Local: "GameLevel", Line: 3,
	})
	references := ExtractKotlinReferenceLocations(source)
	assert.Contains(t, references, KotlinReferenceLocation{
		Name: "GameLevel", Kind: "inheritance", Line: 5,
	})
	assert.Contains(t, references, KotlinReferenceLocation{
		Name: "GameLevel", Kind: "companion-member", Line: 6,
	})
}
