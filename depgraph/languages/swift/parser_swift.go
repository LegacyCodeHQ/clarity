package swift

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/swift"
)

var (
	swiftLanguage   = swift.GetLanguage()
	swiftParserPool = sync.Pool{
		New: func() any {
			parser := sitter.NewParser()
			parser.SetLanguage(swiftLanguage)
			return parser
		},
	}
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

// ParseSwiftTopLevelSymbolNames returns the names of all top-level declarations
// in Swift source: types plus top-level functions, constants/variables, and
// custom operators. This is the declaration side of cross-file reference
// resolution (CLR-14): global funcs, lets, and operators can be referenced
// across files just like types.
func ParseSwiftTopLevelSymbolNames(sourceCode []byte) []string {
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
		var childNames []string
		switch {
		case isSwiftExtensionDeclaration(child):
			// An extension declares nothing of its own name, but its body
			// members are referenceable across files (CLR-31).
			childNames = swiftExtensionMemberNames(child, sourceCode)
		case isTopLevelSwiftSymbolDeclaration(child):
			childNames = swiftDeclarationNames(child, sourceCode)
		default:
			continue
		}
		for _, name := range childNames {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}

	return names
}

// ExtractSwiftReferencedSymbols returns referenced symbol names in Swift source:
// type identifiers plus bare identifiers (function/constant references) and
// custom operators. Names declared at the top level of this file are excluded
// so a file is never treated as depending on itself.
func ExtractSwiftReferencedSymbols(sourceCode []byte) []string {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return []string{}
	}
	defer tree.Close()

	// Exclude names declared anywhere in this file, not just at the top level
	// (CLR-15): a type/func/const declared at an inner or local scope must not
	// be treated as an external reference to a same-named top-level declaration
	// in another file.
	declared := collectSwiftDeclaredNames(tree.RootNode(), sourceCode)

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

		if isSwiftReferenceNode(n) {
			name := strings.TrimSpace(n.Content(sourceCode))
			if name != "" && !declared[name] && !seen[name] {
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
	parser, _ := swiftParserPool.Get().(*sitter.Parser)
	if parser == nil {
		parser = sitter.NewParser()
		parser.SetLanguage(swiftLanguage)
	}
	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	swiftParserPool.Put(parser)
	return tree, err
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

func isTopLevelSwiftSymbolDeclaration(node *sitter.Node) bool {
	if node == nil || node.Parent() == nil {
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
		"typealias_declaration",
		"function_declaration",
		"property_declaration",
		"operator_declaration":
		return true
	default:
		return false
	}
}

func isSwiftDeclarationNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type() {
	case "class_declaration",
		"struct_declaration",
		"enum_declaration",
		"protocol_declaration",
		"actor_declaration",
		"typealias_declaration",
		"function_declaration",
		"property_declaration",
		"operator_declaration":
		return true
	default:
		return false
	}
}

// collectSwiftDeclaredNames returns every name declared anywhere in the tree,
// regardless of scope (top-level, nested, or function-local). Extension
// declarations are skipped so that `extension Foo` still counts as a reference
// to the extended type Foo rather than a declaration of it — but declarations
// nested inside an extension body are still collected.
func collectSwiftDeclaredNames(root *sitter.Node, sourceCode []byte) map[string]bool {
	declared := make(map[string]bool)

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if isSwiftDeclarationNode(n) && !isSwiftExtensionDeclaration(n) {
			for _, name := range swiftDeclarationNames(n, sourceCode) {
				if name != "" {
					declared[name] = true
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(root)
	return declared
}

// swiftDeclarationNames returns the declared name(s) for a top-level declaration
// node. Types and operators yield a single name; a property declaration can bind
// several names (e.g. `let (a, b) = ...`).
func swiftDeclarationNames(node *sitter.Node, sourceCode []byte) []string {
	if node == nil {
		return nil
	}
	switch node.Type() {
	case "function_declaration":
		// The name follows the `func` keyword: either an identifier or, for
		// operator functions, a custom operator token.
		if name := firstDirectChildContent(node, sourceCode, "simple_identifier", "identifier", "custom_operator"); name != "" {
			return []string{name}
		}
	case "operator_declaration":
		if name := firstDirectChildContent(node, sourceCode, "custom_operator"); name != "" {
			return []string{name}
		}
	case "property_declaration":
		return swiftPropertyBindingNames(node, sourceCode)
	default:
		if name := extractSwiftDeclarationName(node, sourceCode); name != "" {
			return []string{name}
		}
	}
	return nil
}

// swiftPropertyBindingNames collects the identifiers bound by a property
// declaration's pattern, ignoring the initializer expression on the right.
func swiftPropertyBindingNames(node *sitter.Node, sourceCode []byte) []string {
	var names []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "pattern" {
			continue
		}
		collectSwiftSimpleIdentifiers(child, sourceCode, &names)
	}
	return names
}

func collectSwiftSimpleIdentifiers(node *sitter.Node, sourceCode []byte, out *[]string) {
	if node == nil {
		return
	}
	if node.Type() == "simple_identifier" || node.Type() == "identifier" {
		if name := strings.TrimSpace(node.Content(sourceCode)); name != "" {
			*out = append(*out, name)
		}
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		collectSwiftSimpleIdentifiers(node.Child(i), sourceCode, out)
	}
}

func firstDirectChildContent(node *sitter.Node, sourceCode []byte, types ...string) string {
	if node == nil {
		return ""
	}
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && typeSet[child.Type()] {
			return strings.TrimSpace(child.Content(sourceCode))
		}
	}
	return ""
}

func firstDirectChildOfType(node *sitter.Node, types ...string) *sitter.Node {
	if node == nil {
		return nil
	}
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && typeSet[child.Type()] {
			return child
		}
	}
	return nil
}

// swiftExtensionMemberNames collects the names of members declared directly in
// an extension body: the methods, computed properties, nested types, and
// typealiases the extension adds to the extended type. These are what other
// files reference when they use the extension (CLR-31), so an extension-only
// file must contribute them to the declaration index. The extended type name
// itself is deliberately not collected — `extension Foo` is a *reference* to
// Foo, not a declaration of it (mirrors collectSwiftDeclaredNames). Only direct
// body members are collected, not function-local declarations inside method
// bodies, matching the one-level scope of the top-level collection.
func swiftExtensionMemberNames(extNode *sitter.Node, sourceCode []byte) []string {
	body := firstDirectChildOfType(extNode, "class_body", "enum_class_body")
	if body == nil {
		return nil
	}
	var names []string
	for i := 0; i < int(body.ChildCount()); i++ {
		member := body.Child(i)
		if member == nil {
			continue
		}
		if isSwiftExtensionDeclaration(member) {
			// A nested extension adds members to some other type; recurse.
			names = append(names, swiftExtensionMemberNames(member, sourceCode)...)
			continue
		}
		if isSwiftDeclarationNode(member) {
			names = append(names, swiftDeclarationNames(member, sourceCode)...)
		}
	}
	return names
}

func isSwiftReferenceNode(node *sitter.Node) bool {
	switch node.Type() {
	case "type_identifier", "simple_type_identifier", "simple_identifier", "identifier", "user_type", "custom_operator":
		return true
	default:
		return false
	}
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
