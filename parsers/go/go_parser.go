package _go

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// GoImport represents an import in a Go file
type GoImport interface {
	Path() string
}

// StandardLibraryImport represents a Go standard library import
type StandardLibraryImport struct {
	path string
}

func (s StandardLibraryImport) Path() string {
	return s.path
}

// ExternalImport represents an external module import
type ExternalImport struct {
	path string
}

func (e ExternalImport) Path() string {
	return e.path
}

// InternalImport represents an internal project import
type InternalImport struct {
	path string
}

func (i InternalImport) Path() string {
	return i.path
}

// classifyGoImport classifies a Go import path
func classifyGoImport(importPath string) GoImport {
	// Standard library imports don't contain dots or slashes (mostly)
	// or they start with certain known patterns
	if isStandardLibrary(importPath) {
		return StandardLibraryImport{path: importPath}
	}

	// External imports typically contain domain names (dots)
	if strings.Contains(importPath, ".") {
		return ExternalImport{path: importPath}
	}

	// Otherwise, consider it internal (relative imports in Go modules)
	return InternalImport{path: importPath}
}

// isStandardLibrary checks if an import path is from the Go standard library
func isStandardLibrary(path string) bool {
	// Common standard library packages
	stdLibPrefixes := []string{
		"fmt", "os", "io", "net", "time", "sync", "context",
		"bytes", "strings", "strconv", "errors", "math", "sort",
		"encoding/", "crypto/", "database/", "net/", "text/",
		"image/", "runtime", "reflect", "testing", "flag",
	}

	for _, prefix := range stdLibPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// GoImports parses a Go file and returns its imports
func GoImports(filePath string) ([]GoImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseGoImports(sourceCode)
}

// ParseGoImports parses Go source code and extracts imports
func ParseGoImports(sourceCode []byte) ([]GoImport, error) {
	lang := golang.GetLanguage()

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go code: %w", err)
	}
	defer tree.Close()

	// Try primary query pattern
	imports, err := queryGoImports(tree.RootNode(), sourceCode, goImportQueryPattern)
	if err != nil {
		return nil, err
	}

	return imports, nil
}

const goImportQueryPattern = `
(import_spec
  path: (interpreted_string_literal) @import.path)
`

// queryGoImports executes a tree-sitter query and extracts import paths
func queryGoImports(rootNode *sitter.Node, sourceCode []byte, pattern string) ([]GoImport, error) {
	lang := golang.GetLanguage()

	query, err := sitter.NewQuery([]byte(pattern), lang)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, rootNode)

	imports := []GoImport{}

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		match = cursor.FilterPredicates(match, sourceCode)

		for _, capture := range match.Captures {
			content := capture.Node.Content(sourceCode)
			// Remove quotes from string literal
			importPath := cleanGoImportPath(content)
			imports = append(imports, classifyGoImport(importPath))
		}
	}

	return imports, nil
}

// cleanGoImportPath removes quotes and trims whitespace from import paths
func cleanGoImportPath(raw string) string {
	// Remove backticks or double quotes
	cleaned := strings.Trim(raw, "`\"")
	return strings.TrimSpace(cleaned)
}

// GoEmbed represents an embedded file from a //go:embed directive
type GoEmbed struct {
	Pattern string // The embed pattern (file path or glob)
}

// ParseGoEmbeds parses Go source code and extracts //go:embed directives
func ParseGoEmbeds(sourceCode []byte) ([]GoEmbed, error) {
	lang := golang.GetLanguage()

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go code: %w", err)
	}
	defer tree.Close()

	return queryGoEmbeds(tree.RootNode(), sourceCode)
}

// queryGoEmbeds extracts //go:embed directives from comments
func queryGoEmbeds(rootNode *sitter.Node, sourceCode []byte) ([]GoEmbed, error) {
	lang := golang.GetLanguage()

	// Query for comment nodes
	query, err := sitter.NewQuery([]byte(`(comment) @comment`), lang)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, rootNode)

	var embeds []GoEmbed

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		for _, capture := range match.Captures {
			content := capture.Node.Content(sourceCode)
			// Check if this is a go:embed directive
			if strings.HasPrefix(content, "//go:embed ") {
				// Extract the patterns after "//go:embed "
				patterns := strings.TrimPrefix(content, "//go:embed ")
				// Split by whitespace to get individual patterns
				for _, pattern := range strings.Fields(patterns) {
					// Remove "all:" prefix if present (used for including hidden files)
					pattern = strings.TrimPrefix(pattern, "all:")
					embeds = append(embeds, GoEmbed{Pattern: pattern})
				}
			}
		}
	}

	return embeds, nil
}
