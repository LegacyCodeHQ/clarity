package csharp

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	tscsharp "github.com/smacker/go-tree-sitter/csharp"
)

var (
	csharpLanguage   = tscsharp.GetLanguage()
	csharpParserPool = sync.Pool{
		New: func() any {
			parser := sitter.NewParser()
			parser.SetLanguage(csharpLanguage)
			return parser
		},
	}
)

// CSharpImport represents a using directive.
type CSharpImport struct {
	Path string
}

// CSharpImports parses a C# file and returns its imports.
func CSharpImports(filePath string) ([]CSharpImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseCSharpImports(string(sourceCode)), nil
}

// ParseCSharpImports parses C# source code and extracts using directives.
func ParseCSharpImports(source string) []CSharpImport {
	sourceCode := []byte(source)
	tree, cleanup, err := parseCSharpTree(sourceCode)
	if err != nil {
		return parseCSharpImportsFallback(source)
	}
	defer cleanup()

	return extractUsingDirectives(tree.RootNode(), sourceCode)
}

func parseCSharpTree(sourceCode []byte) (*sitter.Tree, func(), error) {
	parser, _ := csharpParserPool.Get().(*sitter.Parser)
	if parser == nil {
		parser = sitter.NewParser()
		parser.SetLanguage(csharpLanguage)
	}
	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		csharpParserPool.Put(parser)
		return nil, nil, fmt.Errorf("failed to parse C# code: %w", err)
	}
	cleanup := func() {
		tree.Close()
		csharpParserPool.Put(parser)
	}
	return tree, cleanup, nil
}

func extractUsingDirectives(root *sitter.Node, sourceCode []byte) []CSharpImport {
	var imports []CSharpImport
	if root == nil {
		return imports
	}

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Type() == "using_directive" {
			path := extractUsingPath(node, sourceCode)
			if path != "" {
				imports = append(imports, CSharpImport{Path: path})
			}
			return
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(root)

	return imports
}

func extractUsingPath(usingNode *sitter.Node, sourceCode []byte) string {
	if usingNode == nil {
		return ""
	}

	var fallback string
	for i := 0; i < int(usingNode.NamedChildCount()); i++ {
		child := usingNode.NamedChild(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "qualified_name", "alias_qualified_name":
			return strings.TrimSpace(child.Content(sourceCode))
		case "identifier":
			text := strings.TrimSpace(child.Content(sourceCode))
			if text != "" {
				fallback = text
			}
		}
	}
	return fallback
}

func parseCSharpImportsFallback(source string) []CSharpImport {
	lines := strings.Split(source, "\n")
	var imports []CSharpImport
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "using ") || !strings.Contains(trimmed, ";") {
			continue
		}
		statement := strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
		statement = strings.TrimPrefix(statement, "using ")
		statement = strings.TrimPrefix(statement, "static ")
		if eq := strings.Index(statement, "="); eq >= 0 {
			statement = strings.TrimSpace(statement[eq+1:])
		}
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "(") {
			continue
		}
		imports = append(imports, CSharpImport{Path: statement})
	}
	return imports
}

var csharpTypeIdentifierPattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9_]*\b`)

// ParseCSharpNamespace extracts the file namespace declaration.
func ParseCSharpNamespace(source string) string {
	sourceCode := []byte(source)
	tree, cleanup, err := parseCSharpTree(sourceCode)
	if err == nil {
		defer cleanup()
		namespace := extractNamespace(tree.RootNode(), sourceCode)
		if namespace != "" {
			return namespace
		}
	}
	withoutComments := stripCSharpComments(source)
	if matches := regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z_][A-Za-z0-9_\.]*)\s*[;{]`).FindStringSubmatch(withoutComments); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

var csharpPartialTypePattern = regexp.MustCompile(`\bpartial\s+(?:class|struct|interface|record)\s+([A-Za-z_][A-Za-z0-9_]*)`)

// ParseCSharpPartialTypeNames returns the set of type names declared with the
// `partial` modifier. In valid C# the same type name may appear in multiple
// files of one namespace only as a partial type, so this lets the resolver link
// every file of a partial class (the common Foo.cs / Foo.Async.cs split) while
// still treating non-partial duplicate names as ambiguous.
func ParseCSharpPartialTypeNames(source string) map[string]bool {
	stripped := stripCSharpComments(source)
	result := make(map[string]bool)
	for _, m := range csharpPartialTypePattern.FindAllStringSubmatch(stripped, -1) {
		if len(m) >= 2 && m[1] != "" {
			result[m[1]] = true
		}
	}
	return result
}

// ParseTopLevelCSharpTypeNames extracts top-level type names declared in a file.
func ParseTopLevelCSharpTypeNames(source string) []string {
	sourceCode := []byte(source)
	tree, cleanup, err := parseCSharpTree(sourceCode)
	if err == nil {
		defer cleanup()
		return extractTopLevelTypeNames(tree.RootNode(), sourceCode)
	}

	withoutComments := stripCSharpComments(source)
	seen := make(map[string]bool)
	var names []string

	typeDeclPattern := regexp.MustCompile(`(?m)^\s*(?:public|private|internal|protected|sealed|static|abstract|partial|readonly|file|unsafe|new|\s)*\b(?:class|interface|struct|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	for _, match := range typeDeclPattern.FindAllStringSubmatch(withoutComments, -1) {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}

	delegateDeclPattern := regexp.MustCompile(`(?m)^\s*(?:public|private|internal|protected|static|unsafe|new|\s)*\bdelegate\b[^{;(]*?\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	for _, match := range delegateDeclPattern.FindAllStringSubmatch(withoutComments, -1) {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}

	return names
}

func extractNamespace(root *sitter.Node, sourceCode []byte) string {
	if root == nil {
		return ""
	}
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() != "namespace_declaration" && child.Type() != "file_scoped_namespace_declaration" {
			continue
		}
		for j := 0; j < int(child.NamedChildCount()); j++ {
			ns := child.NamedChild(j)
			if ns == nil {
				continue
			}
			if ns.Type() == "qualified_name" || ns.Type() == "identifier" {
				return strings.TrimSpace(ns.Content(sourceCode))
			}
		}
	}
	return ""
}

func extractTopLevelTypeNames(root *sitter.Node, sourceCode []byte) []string {
	typeNodes := map[string]bool{
		"class_declaration":     true,
		"interface_declaration": true,
		"struct_declaration":    true,
		"enum_declaration":      true,
		"record_declaration":    true,
		"delegate_declaration":  true,
	}
	enclosingTypeNodes := map[string]bool{
		"class_declaration":     true,
		"interface_declaration": true,
		"struct_declaration":    true,
		"record_declaration":    true,
		"enum_declaration":      true,
		"delegate_declaration":  true,
	}

	seen := make(map[string]bool)
	var names []string
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if typeNodes[node.Type()] && isTopLevelDeclaration(node, enclosingTypeNodes) {
			if name := extractDeclarationName(node, sourceCode); name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(root)

	return names
}

func isTopLevelDeclaration(node *sitter.Node, enclosingTypeNodes map[string]bool) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		if enclosingTypeNodes[p.Type()] {
			return false
		}
	}
	return true
}

func extractDeclarationName(node *sitter.Node, sourceCode []byte) string {
	if node == nil {
		return ""
	}
	if named := node.ChildByFieldName("name"); named != nil {
		return strings.TrimSpace(named.Content(sourceCode))
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type() == "identifier" {
			return strings.TrimSpace(child.Content(sourceCode))
		}
	}
	return ""
}

// ExtractCSharpTypeIdentifiers extracts likely type identifiers from code references.
func ExtractCSharpTypeIdentifiers(source string) []string {
	sourceCode := []byte(source)
	tree, cleanup, err := parseCSharpTree(sourceCode)
	if err == nil {
		defer cleanup()
		identifiers := extractTypeIdentifiersFromTree(tree.RootNode(), sourceCode)
		if len(identifiers) > 0 {
			return identifiers
		}
	}

	normalized := stripCSharpStringsAndComments(source)
	matches := csharpTypeIdentifierPattern.FindAllString(normalized, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var identifiers []string
	for _, match := range matches {
		if seen[match] {
			continue
		}
		seen[match] = true
		identifiers = append(identifiers, match)
	}
	return identifiers
}

func extractTypeIdentifiersFromTree(root *sitter.Node, sourceCode []byte) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]bool)
	var out []string
	add := func(name string) {
		if name == "" {
			return
		}
		if isCSharpBuiltinType(name) {
			return
		}
		if seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}

	var collectTypeNames func(*sitter.Node)
	collectTypeNames = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "identifier":
			add(strings.TrimSpace(node.Content(sourceCode)))
			return
		case "qualified_name", "alias_qualified_name":
			last := ""
			for i := 0; i < int(node.NamedChildCount()); i++ {
				child := node.NamedChild(i)
				if child == nil || child.Type() != "identifier" {
					continue
				}
				last = strings.TrimSpace(child.Content(sourceCode))
			}
			add(last)
			return
		case "generic_name":
			for i := 0; i < int(node.NamedChildCount()); i++ {
				child := node.NamedChild(i)
				if child == nil {
					continue
				}
				// Do not add the generic container type (e.g. Task, List) as
				// dependency signals; only collect contained type arguments.
				if child.Type() == "type_argument_list" {
					collectTypeNames(child)
				}
			}
			return
		case "array_type", "nullable_type", "pointer_type", "function_pointer_type", "tuple_type", "type_argument_list":
			for i := 0; i < int(node.NamedChildCount()); i++ {
				collectTypeNames(node.NamedChild(i))
			}
			return
		}
	}

	collectIfTypeContext := func(node *sitter.Node) {
		if node == nil {
			return
		}
		collectTypeNames(node)
	}

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "base_list":
			for i := 0; i < int(node.NamedChildCount()); i++ {
				collectIfTypeContext(node.NamedChild(i))
			}
		case "variable_declaration":
			collectIfTypeContext(node.NamedChild(0))
		case "parameter":
			collectIfTypeContext(node.NamedChild(0))
		case "method_declaration":
			collectIfTypeContext(node.NamedChild(0))
		case "property_declaration":
			collectIfTypeContext(node.NamedChild(0))
		case "indexer_declaration":
			collectIfTypeContext(node.NamedChild(0))
		case "operator_declaration", "conversion_operator_declaration":
			collectIfTypeContext(node.NamedChild(0))
		case "object_creation_expression":
			collectIfTypeContext(node.NamedChild(0))
		case "member_access_expression":
			collectIfTypeContext(node.NamedChild(0))
		case "cast_expression", "as_expression", "is_expression", "declaration_pattern", "recursive_pattern", "type_of_expression", "default_expression", "sizeof_expression":
			collectIfTypeContext(node.NamedChild(0))
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}

	walk(root)
	return out
}

func isCSharpBuiltinType(name string) bool {
	switch name {
	case "bool", "byte", "sbyte", "char", "decimal", "double", "float", "int", "uint", "nint", "nuint",
		"long", "ulong", "short", "ushort", "object", "string", "dynamic", "void":
		return true
	default:
		return false
	}
}

func stripCSharpComments(source string) string {
	return stripCSharp(source, false)
}

func stripCSharpStringsAndComments(source string) string {
	return stripCSharp(source, true)
}

func stripCSharp(source string, stripStrings bool) string {
	var b strings.Builder
	b.Grow(len(source))

	inLineComment := false
	inBlockComment := false
	inString := false
	inVerbatimString := false

	for i := 0; i < len(source); i++ {
		ch := source[i]
		next := byte(0)
		if i+1 < len(source) {
			next = source[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				b.WriteByte(ch)
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if stripStrings && inString {
			if ch == '\\' && next != 0 {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if stripStrings && inVerbatimString {
			if ch == '"' {
				if next == '"' {
					i++
					continue
				}
				inVerbatimString = false
			}
			continue
		}

		if ch == '/' && next == '/' {
			inLineComment = true
			i++
			continue
		}
		if ch == '/' && next == '*' {
			inBlockComment = true
			i++
			continue
		}

		if stripStrings {
			if ch == '@' && next == '"' {
				inVerbatimString = true
				i++
				continue
			}
			if ch == '"' {
				inString = true
				continue
			}
		}

		b.WriteByte(ch)
	}

	return b.String()
}
