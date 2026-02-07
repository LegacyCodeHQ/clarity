package swift

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/swift"
)

// SwiftImport represents an import in a Swift file.
type SwiftImport struct {
	Path string
}

// SwiftImports parses a Swift file and returns its imports.
func SwiftImports(filePath string) ([]SwiftImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseSwiftImports(sourceCode)
}

// ParseSwiftImports parses Swift source code and extracts imports.
func ParseSwiftImports(sourceCode []byte) ([]SwiftImport, error) {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Swift code: %w", err)
	}
	defer tree.Close()

	return extractImports(tree.RootNode(), sourceCode), nil
}

func extractImports(rootNode *sitter.Node, sourceCode []byte) []SwiftImport {
	var imports []SwiftImport

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "import_declaration" {
			if module := extractImportModule(n, sourceCode); module != "" {
				imports = append(imports, SwiftImport{Path: module})
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(rootNode)
	return imports
}

func extractImportModule(node *sitter.Node, sourceCode []byte) string {
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

// ParseSwiftTopLevelTypeNames returns top-level type-like declaration names in Swift source.
func ParseSwiftTopLevelTypeNames(sourceCode []byte) []string {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return []string{}
	}
	defer tree.Close()

	var names []string
	seen := make(map[string]bool)

	root := tree.RootNode()
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if !isTopLevelSwiftDeclaration(child) {
			continue
		}
		name := extractSwiftDeclarationName(child, sourceCode)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}

	return names
}

// ExtractSwiftTypeIdentifiers returns referenced type-like identifiers in Swift source.
func ExtractSwiftTypeIdentifiers(sourceCode []byte) []string {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return []string{}
	}
	defer tree.Close()

	declared := make(map[string]bool)
	for _, name := range ParseSwiftTopLevelTypeNames(sourceCode) {
		if name != "" {
			declared[name] = true
		}
	}

	seen := make(map[string]bool)
	var result []string

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "import_declaration" {
			return
		}

		if isSwiftIdentifierNode(n) {
			name := strings.TrimSpace(n.Content(sourceCode))
			if name != "" && isLikelySwiftTypeName(name) && !declared[name] && !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(tree.RootNode())

	return result
}

func parseSwift(sourceCode []byte) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(swift.GetLanguage())
	return parser.ParseCtx(context.Background(), nil, sourceCode)
}

func isTopLevelSwiftDeclaration(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	if node.Parent() == nil {
		return false
	}
	parentType := node.Parent().Type()
	if parentType != "source_file" && parentType != "program" {
		return false
	}
	if isSwiftExtensionDeclaration(node) {
		return false
	}
	switch node.Type() {
	case "class_declaration",
		"struct_declaration",
		"enum_declaration",
		"protocol_declaration",
		"actor_declaration",
		"typealias_declaration":
		return true
	default:
		return false
	}
}

func isSwiftExtensionDeclaration(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "extension" {
			return true
		}
	}
	return false
}

func extractSwiftDeclarationName(node *sitter.Node, sourceCode []byte) string {
	if node == nil {
		return ""
	}
	if name := node.ChildByFieldName("name"); name != nil {
		return strings.TrimSpace(name.Content(sourceCode))
	}
	if name := findFirstDescendantOfType(node, "type_identifier", "identifier"); name != nil {
		return strings.TrimSpace(name.Content(sourceCode))
	}
	return ""
}

func findFirstDescendantOfType(node *sitter.Node, types ...string) *sitter.Node {
	if node == nil {
		return nil
	}

	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	var walk func(*sitter.Node) *sitter.Node
	walk = func(n *sitter.Node) *sitter.Node {
		if n == nil {
			return nil
		}
		if typeSet[n.Type()] {
			return n
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			if found := walk(n.Child(i)); found != nil {
				return found
			}
		}
		return nil
	}

	return walk(node)
}

func isSwiftIdentifierNode(node *sitter.Node) bool {
	switch node.Type() {
	case "type_identifier", "simple_type_identifier", "simple_identifier", "identifier", "user_type":
		return true
	default:
		return false
	}
}

func isLikelySwiftTypeName(name string) bool {
	if name == "" {
		return false
	}
	r := name[0]
	return r >= 'A' && r <= 'Z'
}
