package java

import (
	"context"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	tsjava "github.com/smacker/go-tree-sitter/java"
)

var (
	javaLanguage   = tsjava.GetLanguage()
	javaParserPool = sync.Pool{
		New: func() any {
			parser := sitter.NewParser()
			parser.SetLanguage(javaLanguage)
			return parser
		},
	}
	javaTypeIdentifierQueryOnce     sync.Once
	javaCompiledTypeIdentifierQuery *sitter.Query
)

func ensureJavaQueries() {
	javaTypeIdentifierQueryOnce.Do(func() {
		var err error
		javaCompiledTypeIdentifierQuery, err = sitter.NewQuery([]byte(javaTypeIdentifierQuery), javaLanguage)
		if err != nil {
			panic("failed to compile java type identifier query: " + err.Error())
		}
	})
}

// JavaImport represents a Java import in source code.
type JavaImport interface {
	Path() string
	IsWildcard() bool
	Package() string
}

// StandardLibraryImport represents a Java/JDK standard library import.
type StandardLibraryImport struct {
	path       string
	isWildcard bool
}

func (s StandardLibraryImport) Path() string {
	return s.path
}

func (s StandardLibraryImport) IsWildcard() bool {
	return s.isWildcard
}

func (s StandardLibraryImport) Package() string {
	return javaImportPackage(s.path)
}

// ExternalImport represents a third-party import.
type ExternalImport struct {
	path       string
	isWildcard bool
}

func (e ExternalImport) Path() string {
	return e.path
}

func (e ExternalImport) IsWildcard() bool {
	return e.isWildcard
}

func (e ExternalImport) Package() string {
	return javaImportPackage(e.path)
}

// InternalImport represents an internal project import.
type InternalImport struct {
	path       string
	isWildcard bool
}

func (i InternalImport) Path() string {
	return i.path
}

func (i InternalImport) IsWildcard() bool {
	return i.isWildcard
}

func (i InternalImport) Package() string {
	return javaImportPackage(i.path)
}

var (
	javaTopLevelDeclarationTypes = map[string]bool{
		"class_declaration":           true,
		"interface_declaration":       true,
		"enum_declaration":            true,
		"record_declaration":          true,
		"annotation_type_declaration": true,
	}
)

// ParsePackageDeclaration extracts the Java package from source code.
func ParsePackageDeclaration(sourceCode []byte) string {
	tree, err := parseJava(sourceCode)
	if err != nil {
		return ""
	}
	defer tree.Close()

	node := findFirstNodeOfType(tree.RootNode(), "package_declaration")
	if node == nil {
		return ""
	}

	if name := node.ChildByFieldName("name"); name != nil {
		return strings.TrimSpace(name.Content(sourceCode))
	}

	if name := findFirstChildOfType(node, "scoped_identifier", "identifier"); name != nil {
		return strings.TrimSpace(name.Content(sourceCode))
	}

	return ""
}

// ParseTopLevelTypeNames extracts declared type names from Java source code.
func ParseTopLevelTypeNames(sourceCode []byte) []string {
	tree, err := parseJava(sourceCode)
	if err != nil {
		return []string{}
	}
	defer tree.Close()

	seen := make(map[string]bool)
	result := []string{}

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		if javaTopLevelDeclarationTypes[node.Type()] && isTopLevelDeclaration(node) {
			if name := extractDeclarationName(node, sourceCode); name != "" && !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}

	walk(tree.RootNode())

	return result
}

// ParseJavaImports parses Java source code and classifies imports.
func ParseJavaImports(sourceCode []byte, projectPackages map[string]bool) []JavaImport {
	tree, err := parseJava(sourceCode)
	if err != nil {
		return []JavaImport{}
	}
	defer tree.Close()

	importDecls := findNodesOfType(tree.RootNode(), "import_declaration")
	if len(importDecls) == 0 {
		return []JavaImport{}
	}

	imports := make([]JavaImport, 0, len(importDecls))
	for _, node := range importDecls {
		path, isWildcard := extractImportPath(node, sourceCode)
		if path == "" {
			continue
		}
		if isWildcard && !strings.HasSuffix(path, ".*") {
			path += ".*"
		}
		imports = append(imports, classifyJavaImport(path, projectPackages))
	}

	return imports
}

func classifyJavaImport(importPath string, projectPackages map[string]bool) JavaImport {
	isWildcard := strings.HasSuffix(importPath, ".*")
	if isStandardLibraryImport(importPath) {
		return StandardLibraryImport{path: importPath, isWildcard: isWildcard}
	}

	if isInternalJavaImport(importPath, projectPackages) {
		return InternalImport{path: importPath, isWildcard: isWildcard}
	}

	return ExternalImport{path: importPath, isWildcard: isWildcard}
}

func isStandardLibraryImport(path string) bool {
	prefixes := []string{
		"java.",
		"javax.",
		"jdk.",
		"sun.",
		"com.sun.",
		"org.w3c.",
		"org.xml.sax.",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func isInternalJavaImport(importPath string, projectPackages map[string]bool) bool {
	pkg := javaImportPackage(importPath)
	if projectPackages[pkg] || projectPackages[importPath] {
		return true
	}

	for projectPkg := range projectPackages {
		if strings.HasPrefix(pkg, projectPkg+".") || pkg == projectPkg {
			return true
		}
		if strings.HasPrefix(importPath, projectPkg+".") {
			return true
		}
	}

	return false
}

func javaImportPackage(path string) string {
	trimmed := strings.TrimSuffix(path, ".*")
	parts := strings.Split(trimmed, ".")
	if len(parts) <= 1 {
		return trimmed
	}

	last := parts[len(parts)-1]
	if len(last) > 0 {
		first := last[0]
		if first >= 'A' && first <= 'Z' {
			return strings.Join(parts[:len(parts)-1], ".")
		}
	}

	return trimmed
}

func simpleTypeName(path string) string {
	trimmed := strings.TrimSuffix(path, ".*")
	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// ExtractTypeIdentifiers returns referenced type-like identifiers in Java source.
func ExtractTypeIdentifiers(sourceCode []byte) []string {
	tree, err := parseJava(sourceCode)
	if err != nil {
		return []string{}
	}
	defer tree.Close()

	seen := make(map[string]bool)
	result := []string{}

	ensureJavaQueries()
	if javaCompiledTypeIdentifierQuery != nil {
		cursor := sitter.NewQueryCursor()
		cursor.Exec(javaCompiledTypeIdentifierQuery, tree.RootNode())

		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}

			match = cursor.FilterPredicates(match, sourceCode)
			for _, capture := range match.Captures {
				name := strings.TrimSpace(capture.Node.Content(sourceCode))
				name = simpleTypeName(name)
				if name == "" || seen[name] {
					continue
				}
				seen[name] = true
				result = append(result, name)
			}
		}

		cursor.Close()
	}

	for _, name := range ParseTopLevelTypeNames(sourceCode) {
		if name != "" && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}

	return result
}

// ExtractQualifiedTypeReferences returns dotted Java references that may point at project types.
func ExtractQualifiedTypeReferences(sourceCode []byte) []string {
	tree, err := parseJava(sourceCode)
	if err != nil {
		return []string{}
	}
	defer tree.Close()

	seen := make(map[string]bool)
	result := []string{}
	for _, nodeType := range []string{"scoped_type_identifier", "scoped_identifier"} {
		for _, node := range findNodesOfType(tree.RootNode(), nodeType) {
			name := strings.TrimSpace(node.Content(sourceCode))
			if !strings.Contains(name, ".") || !hasUppercaseSegment(name) || seen[name] {
				continue
			}
			seen[name] = true
			result = append(result, name)
		}
	}

	return result
}

func hasUppercaseSegment(name string) bool {
	for _, part := range strings.Split(name, ".") {
		if startsWithASCIIUppercase(part) {
			return true
		}
	}
	return false
}

func startsWithASCIIUppercase(name string) bool {
	if name == "" {
		return false
	}
	first := name[0]
	return first >= 'A' && first <= 'Z'
}

const javaTypeIdentifierQuery = `
((type_identifier) @type.name)
((scoped_type_identifier) @type.name)
((field_access object: (identifier) @type.name))
((method_invocation object: (identifier) @type.name))
((marker_annotation name: (identifier) @type.name))
((annotation name: (identifier) @type.name))
((marker_annotation name: (scoped_identifier) @type.name))
((annotation name: (scoped_identifier) @type.name))
`

func parseJava(sourceCode []byte) (*sitter.Tree, error) {
	parser, _ := javaParserPool.Get().(*sitter.Parser)
	if parser == nil {
		parser = sitter.NewParser()
		parser.SetLanguage(javaLanguage)
	}
	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	javaParserPool.Put(parser)
	return tree, err
}

func isTopLevelDeclaration(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	parent := node.Parent()
	return parent != nil && parent.Type() == "program"
}

func extractDeclarationName(node *sitter.Node, sourceCode []byte) string {
	if node == nil {
		return ""
	}
	if name := node.ChildByFieldName("name"); name != nil {
		return strings.TrimSpace(name.Content(sourceCode))
	}
	if name := findFirstChildOfType(node, "type_identifier", "identifier"); name != nil {
		return strings.TrimSpace(name.Content(sourceCode))
	}
	return ""
}

func extractImportPath(node *sitter.Node, sourceCode []byte) (string, bool) {
	if node == nil {
		return "", false
	}

	if name := node.ChildByFieldName("name"); name != nil {
		return strings.TrimSpace(name.Content(sourceCode)), hasChildOfType(node, "asterisk")
	}

	nameNode := findFirstChildOfType(node, "scoped_identifier", "identifier")
	if nameNode == nil {
		return "", false
	}

	return strings.TrimSpace(nameNode.Content(sourceCode)), hasChildOfType(node, "asterisk")
}

func hasChildOfType(node *sitter.Node, nodeType string) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == nodeType {
			return true
		}
		if hasChildOfType(child, nodeType) {
			return true
		}
	}
	return false
}

func findFirstChildOfType(node *sitter.Node, types ...string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		for _, t := range types {
			if child.Type() == t {
				return child
			}
		}
	}
	return nil
}

func findFirstNodeOfType(node *sitter.Node, nodeType string) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Type() == nodeType {
		return node
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		found := findFirstNodeOfType(node.NamedChild(i), nodeType)
		if found != nil {
			return found
		}
	}
	return nil
}

func findNodesOfType(node *sitter.Node, nodeType string) []*sitter.Node {
	if node == nil {
		return nil
	}
	nodes := []*sitter.Node{}
	if node.Type() == nodeType {
		nodes = append(nodes, node)
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		nodes = append(nodes, findNodesOfType(node.NamedChild(i), nodeType)...)
	}
	return nodes
}
