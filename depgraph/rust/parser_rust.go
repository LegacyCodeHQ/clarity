package rust

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
)

// RustImport represents a Rust import statement.
type RustImport struct {
	Path string
}

// RustImports parses a Rust file and returns its imports.
func RustImports(filePath string) ([]RustImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseRustImports(sourceCode)
}

// ParseRustImports parses Rust source code and extracts imports.
func ParseRustImports(sourceCode []byte) ([]RustImport, error) {
	lang := rust.GetLanguage()

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Rust code: %w", err)
	}
	defer tree.Close()

	return extractImports(tree.RootNode(), sourceCode), nil
}

func extractImports(rootNode *sitter.Node, sourceCode []byte) []RustImport {
	var imports []RustImport

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "use_declaration":
			if path := extractUsePath(n, sourceCode); path != "" {
				imports = append(imports, RustImport{Path: path})
			}
		case "extern_crate_declaration":
			if crate := extractExternCrate(n, sourceCode); crate != "" {
				imports = append(imports, RustImport{Path: crate})
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(rootNode)
	return imports
}

func extractUsePath(node *sitter.Node, sourceCode []byte) string {
	pathNode := findUsePathNode(node)
	if pathNode == nil {
		return ""
	}
	return strings.TrimSpace(pathNode.Content(sourceCode))
}

func findUsePathNode(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if isUsePathNode(node.Type()) {
		return node
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if found := findUsePathNode(child); found != nil {
			return found
		}
	}
	return nil
}

func isUsePathNode(nodeType string) bool {
	switch nodeType {
	case "scoped_identifier", "identifier", "crate", "self", "super":
		return true
	default:
		return false
	}
}

func extractExternCrate(node *sitter.Node, sourceCode []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "identifier" {
			return strings.TrimSpace(child.Content(sourceCode))
		}
	}
	return ""
}
