package dart

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/LegacyCodeHQ/clarity/tree_sitter_external/dart"

	sitter "github.com/smacker/go-tree-sitter"
)

type Import interface {
	URI() string
}

// PackageImport represents an external dependency (dart:* or package:*)
type PackageImport struct {
	uri string
}

func (p PackageImport) URI() string {
	return p.uri
}

// ProjectImport represents an internal project file (relative paths)
type ProjectImport struct {
	uri string
}

func (p ProjectImport) URI() string {
	return p.uri
}

func classifyImport(uri string) Import {
	if strings.HasPrefix(uri, "dart:") || strings.HasPrefix(uri, "package:") {
		return PackageImport{uri: uri}
	}
	return ProjectImport{uri: uri}
}

func Imports(filePath string) ([]Import, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseImports(sourceCode)
}

func ParseImports(sourceCode []byte) ([]Import, error) {
	lang := dart.GetLanguage()

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Dart code: %w", err)
	}
	defer tree.Close()

	// Try primary query pattern
	imports, err := queryImports(tree.RootNode(), sourceCode, primaryQueryPattern)
	if err == nil && len(imports) > 0 {
		return imports, nil
	}

	// Fallback to alternative patterns if primary fails
	for _, pattern := range fallbackQueryPatterns {
		imports, err = queryImports(tree.RootNode(), sourceCode, pattern)
		if err == nil && len(imports) > 0 {
			return imports, nil
		}
	}

	// No error but also no imports found - return empty slice
	return []Import{}, nil
}

const primaryQueryPattern = `
(import_or_export
  (library_import
    (import_specification
      (configurable_uri
        (uri
          (string_literal) @import.uri)))))
`

var fallbackQueryPatterns = []string{
	`(configurable_uri (uri (string_literal) @import.uri))`,
	`(uri (string_literal) @import.uri)`,
}

// queryImports executes a tree-sitter query and extracts import URIs
func queryImports(rootNode *sitter.Node, sourceCode []byte, pattern string) ([]Import, error) {
	lang := dart.GetLanguage()

	query, err := sitter.NewQuery([]byte(pattern), lang)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, rootNode)

	var imports []Import

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		match = cursor.FilterPredicates(match, sourceCode)

		for _, capture := range match.Captures {
			content := capture.Node.Content(sourceCode)
			// Remove quotes from string literal
			importURI := cleanImportURI(content)
			imports = append(imports, classifyImport(importURI))
		}
	}

	return imports, nil
}

// cleanImportURI removes quotes and trims whitespace from import URIs
func cleanImportURI(raw string) string {
	// Remove single or double quotes
	cleaned := strings.Trim(raw, "'\"")
	return strings.TrimSpace(cleaned)
}
