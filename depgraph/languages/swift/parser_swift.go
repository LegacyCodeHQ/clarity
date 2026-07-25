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
			// Extension members are owner-scoped and resolved separately.
			// Treating them as global declarations creates false edges for
			// common member names such as `error`, `json`, and `span`.
			continue
		case isTopLevelSwiftSymbolDeclaration(child):
			if isPrivateSwiftDeclaration(child, sourceCode) {
				continue
			}
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

		if isSwiftReferenceNode(n) &&
			!isQualifiedSwiftMemberName(n, sourceCode) &&
			!isSwiftDeclarationName(n) &&
			!isImplicitSwiftCatchError(n, sourceCode) {
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

// SwiftQualifiedReference is a member referenced through an explicit type-like
// qualifier (for example Parser.parse or Self.parse).
type SwiftQualifiedReference struct {
	Qualifier string
	Member    string
}

// ExtractSwiftQualifiedReferences returns explicit type/Self member references.
// Lowercase value-member navigation such as logger.error is deliberately
// excluded because it cannot be matched safely without type information.
func ExtractSwiftQualifiedReferences(sourceCode []byte) []SwiftQualifiedReference {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	seen := make(map[SwiftQualifiedReference]bool)
	var result []SwiftQualifiedReference
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "navigation_expression" {
			target := n.ChildByFieldName("target")
			suffix := n.ChildByFieldName("suffix")
			member := firstDescendantContent(suffix, sourceCode, "simple_identifier", "identifier")
			qualifier := ""
			if target != nil {
				qualifier = strings.TrimSpace(target.Content(sourceCode))
			}
			if member != "" && (qualifier == "Self" || isLikelySwiftTypeName(qualifier)) {
				ref := SwiftQualifiedReference{Qualifier: qualifier, Member: member}
				if !seen[ref] {
					seen[ref] = true
					result = append(result, ref)
				}
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(tree.RootNode())
	return result
}

// SwiftExtensionMember identifies a cross-file-visible extension member and
// the type whose extension declares it.
type SwiftExtensionMember struct {
	Owner  string
	Member string
}

// ParseSwiftExtensionMembers returns non-private direct extension members.
func ParseSwiftExtensionMembers(sourceCode []byte) []SwiftExtensionMember {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	var result []SwiftExtensionMember
	root := tree.RootNode()
	for i := 0; i < int(root.ChildCount()); i++ {
		ext := root.Child(i)
		if ext == nil || !isSwiftExtensionDeclaration(ext) ||
			isPrivateSwiftDeclaration(ext, sourceCode) {
			continue
		}
		owner := extractSwiftDeclarationName(ext, sourceCode)
		body := firstDirectChildOfType(ext, "class_body", "enum_class_body")
		if owner == "" || body == nil {
			continue
		}
		for j := 0; j < int(body.ChildCount()); j++ {
			member := body.Child(j)
			if member == nil || !isSwiftDeclarationNode(member) ||
				isPrivateSwiftDeclaration(member, sourceCode) {
				continue
			}
			for _, name := range swiftDeclarationNames(member, sourceCode) {
				if name != "" {
					result = append(result, SwiftExtensionMember{Owner: owner, Member: name})
				}
			}
		}
	}
	return result
}

// ParseSwiftExtensionOwners returns the types extended by this file.
func ParseSwiftExtensionOwners(sourceCode []byte) []string {
	tree, err := parseSwift(sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	var owners []string
	seen := make(map[string]bool)
	root := tree.RootNode()
	for i := 0; i < int(root.ChildCount()); i++ {
		ext := root.Child(i)
		if ext == nil || !isSwiftExtensionDeclaration(ext) {
			continue
		}
		owner := extractSwiftDeclarationName(ext, sourceCode)
		if owner != "" && !seen[owner] {
			seen[owner] = true
			owners = append(owners, owner)
		}
	}
	return owners
}

func isQualifiedSwiftMemberName(node *sitter.Node, sourceCode []byte) bool {
	if node == nil {
		return false
	}
	parent := node.Parent()
	if parent == nil {
		return false
	}
	if parent.Type() == "navigation_suffix" {
		return true
	}
	// Swift's leading-dot shorthand (`.error`, including switch patterns)
	// retains the dot on the parent expression/pattern rather than on the
	// identifier node itself. It is a member selection, not a bare reference.
	return strings.HasPrefix(strings.TrimSpace(parent.Content(sourceCode)), ".")
}

func isSwiftDeclarationName(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	parent := node.Parent()
	if parent == nil {
		return false
	}
	switch parent.Type() {
	case "enum_entry":
		return true
	default:
		return false
	}
}

func isImplicitSwiftCatchError(node *sitter.Node, sourceCode []byte) bool {
	if node == nil || strings.TrimSpace(node.Content(sourceCode)) != "error" {
		return false
	}
	for ancestor := node.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		if ancestor.Type() == "catch_block" {
			return true
		}
		if ancestor.Type() == "function_declaration" {
			return false
		}
	}
	return false
}

func isPrivateSwiftDeclaration(node *sitter.Node, sourceCode []byte) bool {
	if node == nil {
		return false
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "modifiers" {
			continue
		}
		for j := 0; j < int(child.ChildCount()); j++ {
			modifier := child.Child(j)
			if modifier == nil || modifier.Type() != "visibility_modifier" {
				continue
			}
			switch strings.TrimSpace(modifier.Content(sourceCode)) {
			case "private", "fileprivate":
				return true
			}
		}
	}
	return false
}

func firstDescendantContent(node *sitter.Node, sourceCode []byte, types ...string) string {
	found := firstDescendant(node, types...)
	if found == nil {
		return ""
	}
	return strings.TrimSpace(found.Content(sourceCode))
}

func firstDescendant(node *sitter.Node, types ...string) *sitter.Node {
	if node == nil {
		return nil
	}
	for _, nodeType := range types {
		if node.Type() == nodeType {
			return node
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if value := firstDescendant(node.Child(i), types...); value != nil {
			return value
		}
	}
	return nil
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
