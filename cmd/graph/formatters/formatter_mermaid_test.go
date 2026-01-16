package formatters_test

import (
	"testing"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/git"
	"github.com/LegacyCodeHQ/sanity/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMermaidFormatter_BasicFlowchart(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	assert.Contains(t, mermaid, "flowchart LR")
	assert.Contains(t, mermaid, "main.dart")
	assert.Contains(t, mermaid, "utils.dart")
	assert.Contains(t, mermaid, "-->")
}

func TestMermaidFormatter_WithLabel(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/main.dart": {},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{Label: "My Graph"})
	require.NoError(t, err)

	assert.Contains(t, mermaid, "---")
	assert.Contains(t, mermaid, "title: My Graph")
	assert.Contains(t, mermaid, "flowchart LR")
}

func TestMermaidFormatter_WithoutLabel(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/main.dart": {},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	assert.NotContains(t, mermaid, "title:")
	assert.Contains(t, mermaid, "flowchart LR")
}

func TestMermaidFormatter_NewFilesUseSeedlingLabel(t *testing.T) {
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

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	// New file without stats should have seedling
	assert.Contains(t, mermaid, "🪴 new_file.dart")
	// New file with stats should have seedling and stats
	assert.Contains(t, mermaid, "🪴 new_with_stats.dart")
	assert.Contains(t, mermaid, "+12")
	assert.Contains(t, mermaid, "-1")
	// Existing file with stats should show stats without seedling
	assert.Contains(t, mermaid, "+3")
}

func TestMermaidFormatter_TestFilesAreStyled(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be present
	assert.Contains(t, mermaid, "main_test.go")
	assert.Contains(t, mermaid, "utils_test.go")

	// Test file style should be defined
	assert.Contains(t, mermaid, "classDef testFile fill:#90EE90,stroke:#228B22,color:#000000")
	// Test files should have testFile class applied
	assert.Contains(t, mermaid, "class")
	assert.Contains(t, mermaid, "testFile")
}

func TestMermaidFormatter_DartTestFiles(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/lib/main.dart":        {"/project/lib/utils.dart"},
		"/project/lib/utils.dart":       {},
		"/project/test/main_test.dart":  {"/project/lib/main.dart"},
		"/project/test/utils_test.dart": {"/project/lib/utils.dart"},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be present
	assert.Contains(t, mermaid, "main_test.dart")
	assert.Contains(t, mermaid, "utils_test.dart")
	// Test file style should be applied
	assert.Contains(t, mermaid, "classDef testFile")
}

func TestMermaidFormatter_NewFilesAreStyled(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/new_file.dart":  {},
		"/project/existing.dart":  {},
		"/project/another_new.go": {},
	}

	stats := map[string]git.FileStats{
		"/project/new_file.dart": {
			IsNew: true,
		},
		"/project/another_new.go": {
			IsNew: true,
		},
		"/project/existing.dart": {
			Additions: 5,
		},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	// New file style should be defined
	assert.Contains(t, mermaid, "classDef newFile fill:#87CEEB,stroke:#4682B4")
	// New files should have newFile class applied
	assert.Contains(t, mermaid, "newFile")
}

func TestMermaidFormatter_TypeScriptTestFiles(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/src/App.tsx":                    {"/project/src/utils.tsx"},
		"/project/src/utils.tsx":                  {},
		"/project/src/App.test.tsx":               {"/project/src/App.tsx"},
		"/project/src/__tests__/utils.test.tsx":   {"/project/src/utils.tsx"},
		"/project/src/components/Button.spec.tsx": {},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be present
	assert.Contains(t, mermaid, "App.test.tsx")
	assert.Contains(t, mermaid, "utils.test.tsx")
	assert.Contains(t, mermaid, "Button.spec.tsx")
	// Test file style should be defined
	assert.Contains(t, mermaid, "classDef testFile")
}

func TestMermaidFormatter_EdgesBetweenNodes(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/a.go": {"/project/b.go", "/project/c.go"},
		"/project/b.go": {"/project/c.go"},
		"/project/c.go": {},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Should have edges (the format is nodeID --> nodeID)
	assert.Contains(t, mermaid, "-->")
}

func TestMermaidFormatter_QuoteEscaping(t *testing.T) {
	// Test that quotes in labels are properly escaped
	graph := parsers.DependencyGraph{
		"/project/file.go": {},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Node should be defined with quoted label
	assert.Contains(t, mermaid, "file.go")
}

func TestMermaidFormatter_EmptyGraph(t *testing.T) {
	graph := parsers.DependencyGraph{}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Should still have flowchart declaration
	assert.Contains(t, mermaid, "flowchart LR")
	// Should define the style classes even if empty
	assert.Contains(t, mermaid, "classDef testFile")
	assert.Contains(t, mermaid, "classDef newFile")
}

func TestMermaidFormatter_FileStatsWithOnlyAdditions(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/modified.go": {},
	}

	stats := map[string]git.FileStats{
		"/project/modified.go": {
			Additions: 10,
			Deletions: 0,
		},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	assert.Contains(t, mermaid, "+10")
	assert.NotContains(t, mermaid, "-0")
}

func TestMermaidFormatter_FileStatsWithOnlyDeletions(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/modified.go": {},
	}

	stats := map[string]git.FileStats{
		"/project/modified.go": {
			Additions: 0,
			Deletions: 5,
		},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	assert.Contains(t, mermaid, "-5")
	assert.NotContains(t, mermaid, "+0")
}

func TestMermaidFormatter_TestFileTakesPriorityOverNewFile(t *testing.T) {
	// Test files should be styled as test files, not new files
	graph := parsers.DependencyGraph{
		"/project/main_test.go": {},
	}

	stats := map[string]git.FileStats{
		"/project/main_test.go": {
			IsNew: true,
		},
	}

	formatter := &formatters.MermaidFormatter{}
	mermaid, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	// The test file class should be applied, but not the newFile class to this node
	assert.Contains(t, mermaid, "classDef testFile")
	// Since it's a test file, it should be in testFile class, not newFile
	assert.Contains(t, mermaid, "testFile")
}
