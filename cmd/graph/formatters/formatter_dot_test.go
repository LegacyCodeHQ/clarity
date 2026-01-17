package formatters_test

import (
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/vcs"
	"github.com/LegacyCodeHQ/sanity/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencyGraph_ToDOT(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	assert.Contains(t, dot, "digraph dependencies")
	assert.Contains(t, dot, "main.dart")
	assert.Contains(t, dot, "utils.dart")
	assert.Contains(t, dot, "->")
}

func TestDependencyGraph_ToDOT_NewFilesUseSeedlingLabel(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/new_file.dart":       {},
		"/project/new_with_stats.dart": {},
		"/project/existing.dart":       {},
	}

	stats := map[string]git.FileStats{
		"/project/new_file.dart": {
			IsNew: true,
		},
		"/project/new_with_stats.dart": {
			IsNew:     true,
			Additions: 12,
			Deletions: 1,
		},
		"/project/existing.dart": {
			Additions: 3,
		},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	assert.Contains(t, dot, "\"new_file.dart\" [label=\"🪴 new_file.dart\",")
	assert.Contains(t, dot, "\"new_with_stats.dart\" [label=\"🪴 new_with_stats.dart\\n+12 -1\",")
	assert.Contains(t, dot, "\"existing.dart\" [label=\"existing.dart\\n+3\",")
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen(t *testing.T) {
	// Test Go test files
	graph := parsers.DependencyGraph{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be light green
	assert.Contains(t, dot, "main_test.go")
	assert.Contains(t, dot, "utils_test.go")
	assert.Contains(t, dot, `"main_test.go" [label="main_test.go", style=filled, fillcolor=lightgreen]`)
	assert.Contains(t, dot, `"utils_test.go" [label="utils_test.go", style=filled, fillcolor=lightgreen]`)

	// Non-test files should not be light green
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.NotContains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=lightgreen]`)
	assert.NotContains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=lightgreen]`)
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen_Dart(t *testing.T) {
	// Test Dart test files (in test/ directory)
	graph := parsers.DependencyGraph{
		"/project/lib/main.dart":        {"/project/lib/utils.dart"},
		"/project/lib/utils.dart":       {},
		"/project/test/main_test.dart":  {"/project/lib/main.dart"},
		"/project/test/utils_test.dart": {"/project/lib/utils.dart"},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be light green
	assert.Contains(t, dot, "main_test.dart")
	assert.Contains(t, dot, "utils_test.dart")
	assert.Contains(t, dot, `"main_test.dart" [label="main_test.dart", style=filled, fillcolor=lightgreen]`)
	assert.Contains(t, dot, `"utils_test.dart" [label="utils_test.dart", style=filled, fillcolor=lightgreen]`)

	// Non-test files should not be light green
	assert.Contains(t, dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.dart" [label="utils.dart", style=filled, fillcolor=white]`)
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite(t *testing.T) {
	// Create graph with majority .go files (5 files) and minority .dart files (2 files)
	graph := parsers.DependencyGraph{
		"/project/main.go":    {"/project/utils.go"},
		"/project/utils.go":   {},
		"/project/types.go":   {},
		"/project/helpers.go": {},
		"/project/config.go":  {},
		"/project/main.dart":  {},
		"/project/utils.dart": {},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// All .go files (majority extension) should be white
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"types.go" [label="types.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"helpers.go" [label="helpers.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"config.go" [label="config.go", style=filled, fillcolor=white]`)

	// .dart files (minority extension) should have a different color (not white)
	// They should have a color from the extension color palette
	assert.Contains(t, dot, "main.dart")
	assert.Contains(t, dot, "utils.dart")
	// Verify they are not white
	assert.NotContains(t, dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`)
	assert.NotContains(t, dot, `"utils.dart" [label="utils.dart", style=filled, fillcolor=white]`)
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite_WithTestFiles(t *testing.T) {
	// Test that test files are light green even if they're part of majority extension
	graph := parsers.DependencyGraph{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/types.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
		"/project/main.dart":     {},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be light green (priority over majority extension)
	assert.Contains(t, dot, `"main_test.go" [label="main_test.go", style=filled, fillcolor=lightgreen]`)
	assert.Contains(t, dot, `"utils_test.go" [label="utils_test.go", style=filled, fillcolor=lightgreen]`)

	// Non-test .go files (majority extension) should be white
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"types.go" [label="types.go", style=filled, fillcolor=white]`)

	// .dart file (minority extension) should not be white
	assert.Contains(t, dot, "main.dart")
	assert.NotContains(t, dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`)
}

func TestDependencyGraph_ToDOT_MajorityExtensionTie(t *testing.T) {
	// Test when there's a tie for majority (should pick one deterministically)
	graph := parsers.DependencyGraph{
		"/project/main.go":    {},
		"/project/utils.go":   {},
		"/project/main.dart":  {},
		"/project/utils.dart": {},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// One extension should be white (the one chosen as majority)
	// The other should have a different color
	goIsWhite := strings.Contains(dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`) &&
		strings.Contains(dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	dartIsWhite := strings.Contains(dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`) &&
		strings.Contains(dot, `"utils.dart" [label="utils.dart", style=filled, fillcolor=white]`)

	// Exactly one extension should be white (not both)
	assert.True(t, goIsWhite != dartIsWhite, "Exactly one extension should be white, not both")
}

func TestDependencyGraph_ToDOT_SingleExtensionAllWhite(t *testing.T) {
	// When all files have the same extension, they should all be white
	graph := parsers.DependencyGraph{
		"/project/main.go":  {"/project/utils.go"},
		"/project/utils.go": {},
		"/project/types.go": {},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// All files should be white (single extension)
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"types.go" [label="types.go", style=filled, fillcolor=white]`)
}

func TestDependencyGraph_ToDOT_TypeScriptTestFiles(t *testing.T) {
	// Test TypeScript test files are styled as light green
	graph := parsers.DependencyGraph{
		"/project/src/App.tsx":                    {"/project/src/utils.tsx"},
		"/project/src/utils.tsx":                  {},
		"/project/src/App.test.tsx":               {"/project/src/App.tsx"},
		"/project/src/__tests__/utils.test.tsx":   {"/project/src/utils.tsx"},
		"/project/src/components/Button.spec.tsx": {},
	}

	formatter := &formatters.DOTFormatter{}
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files with .test.tsx suffix should be light green
	assert.Contains(t, dot, `"App.test.tsx" [label="App.test.tsx", style=filled, fillcolor=lightgreen]`)

	// Test files in __tests__ directory should be light green
	assert.Contains(t, dot, `"utils.test.tsx" [label="utils.test.tsx", style=filled, fillcolor=lightgreen]`)

	// Test files with .spec.tsx suffix should be light green
	assert.Contains(t, dot, `"Button.spec.tsx" [label="Button.spec.tsx", style=filled, fillcolor=lightgreen]`)

	// Non-test files should NOT be light green (they should be white as majority extension)
	assert.Contains(t, dot, `"App.tsx" [label="App.tsx", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.tsx" [label="utils.tsx", style=filled, fillcolor=white]`)
}
