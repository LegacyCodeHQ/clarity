package swift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSwiftImports(t *testing.T) {
	source := `
import Foundation
import MyModule
`
	imports, err := ParseSwiftImports([]byte(source))

	require.NoError(t, err)
	assert.Len(t, imports, 2)
	assert.Equal(t, "Foundation", imports[0].Path)
	assert.Equal(t, "MyModule", imports[1].Path)
}

func TestSwiftImports_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "App.swift")

	content := `
import SwiftUI
`
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	imports, err := SwiftImports(tmpFile)

	require.NoError(t, err)
	assert.Len(t, imports, 1)
	assert.Equal(t, "SwiftUI", imports[0].Path)
}

func TestExtractSwiftTypeIdentifiers_SimpleIdentifier(t *testing.T) {
	source := `
import CodexBarCore

struct FactoryStatusProbeFetchTests {
    func testProbe() {
        let probe = FactoryStatusProbe()
        _ = probe
    }
}
`
	identifiers := ExtractSwiftTypeIdentifiers([]byte(source))
	assert.Contains(t, identifiers, "FactoryStatusProbe")
}

func TestParseSwiftTopLevelTypeNames_SkipsExtensions(t *testing.T) {
	source := `
extension SettingsStore {
    var foo: Int { 1 }
}

struct RealType {
    let value: Int
}
`
	types := ParseSwiftTopLevelTypeNames([]byte(source))
	assert.Contains(t, types, "RealType")
	assert.NotContains(t, types, "SettingsStore")
}

func TestParseSwiftTopLevelTypeNames_WithProtocolConformance(t *testing.T) {
	source := `
import Foundation

struct GraphFileNode: Identifiable, Hashable {
    let id: String
}
`
	types := ParseSwiftTopLevelTypeNames([]byte(source))
	assert.Contains(t, types, "GraphFileNode")
}
