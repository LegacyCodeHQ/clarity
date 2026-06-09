package kotlin

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// KotlinImport represents an import in a Kotlin file
type KotlinImport interface {
	Path() string
	IsWildcard() bool
	Package() string
}

// StandardLibraryImport represents a Kotlin/Java/Android standard library import
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
	return extractPackageFromPath(s.path)
}

// ExternalImport represents an external library import
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
	return extractPackageFromPath(e.path)
}

// InternalImport represents an internal project import
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
	return extractPackageFromPath(i.path)
}

// extractPackageFromPath extracts the package name from an import path
// For wildcard imports, it's already the package name
// For specific imports, we need to remove the class name (typically starts with uppercase)
func extractPackageFromPath(importPath string) string {
	parts := strings.Split(importPath, ".")

	// Find the last lowercase segment (package boundary)
	// Class names typically start with uppercase in Kotlin
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && len(parts[i]) > 0 && unicode.IsLower(rune(parts[i][0])) {
			return strings.Join(parts[:i+1], ".")
		}
	}

	// If all lowercase (package only), return full path
	return importPath
}

// classifyKotlinImport classifies a Kotlin import path
func classifyKotlinImport(importPath string, isWildcard bool, projectPackages map[string]bool) KotlinImport {
	// Check for standard library prefixes
	if isStandardLibrary(importPath) {
		return StandardLibraryImport{path: importPath, isWildcard: isWildcard}
	}

	// Check if import matches project package structure
	if isInternalPackage(importPath, projectPackages) {
		return InternalImport{path: importPath, isWildcard: isWildcard}
	}

	// Everything else is external
	return ExternalImport{path: importPath, isWildcard: isWildcard}
}

// isStandardLibrary checks if an import path is from the Kotlin/Java/Android standard library
func isStandardLibrary(path string) bool {
	stdLibPrefixes := []string{
		"kotlin.", "kotlinx.", // Kotlin stdlib and extensions
		"java.", "javax.", // Java stdlib
		"android.", // Android SDK
	}

	for _, prefix := range stdLibPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// isInternalPackage checks if an import path matches the project package structure
func isInternalPackage(importPath string, projectPackages map[string]bool) bool {
	// Extract package from import path
	pkg := extractPackageFromPath(importPath)

	// Check exact match
	if projectPackages[pkg] {
		return true
	}

	// Check if import path itself is in project packages (for class imports)
	if projectPackages[importPath] {
		return true
	}

	// Check if import is a sub-package of any project package
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

// KotlinImports parses a Kotlin file and returns its imports
func KotlinImports(filePath string) ([]KotlinImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseKotlinImports(sourceCode)
}

// ParseKotlinImports parses Kotlin source code and extracts imports
func ParseKotlinImports(sourceCode []byte) ([]KotlinImport, error) {
	ensureKotlinQueries()

	parser := kotlinParserPool.Get().(*sitter.Parser)
	defer kotlinParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Kotlin code: %w", err)
	}
	defer tree.Close()

	// Try primary query pattern
	imports, err := runQueryKotlinImports(tree.RootNode(), sourceCode, kotlinCompiledImportQuery, kotlinImportQueryPattern)
	if err == nil {
		return imports, nil
	}

	// If primary pattern fails, try fallback patterns
	for i, q := range kotlinCompiledFallbackQueries {
		imports, err = runQueryKotlinImports(tree.RootNode(), sourceCode, q, kotlinFallbackQueryPatterns[i])
		if err == nil {
			return imports, nil
		}
	}

	// If all tree-sitter queries fail, return empty slice (no imports found)
	return []KotlinImport{}, nil
}

// Primary query pattern for Kotlin imports
const kotlinImportQueryPattern = `
(import_header
  (identifier) @import.path)
`

// Fallback query patterns to try if primary fails
var kotlinFallbackQueryPatterns = []string{
	`(import_list (import_header (identifier) @import.path))`,
	`(import_header) @import.full`,
}

var kotlinParserPool = sync.Pool{
	New: func() interface{} {
		lang := kotlin.GetLanguage()
		p := sitter.NewParser()
		p.SetLanguage(lang)
		return p
	},
}

var (
	kotlinQueryOnce                sync.Once
	kotlinCompiledImportQuery      *sitter.Query
	kotlinCompiledFallbackQueries  []*sitter.Query
	kotlinCompiledPackageQuery     *sitter.Query
	kotlinCompiledTypeIdQuery      *sitter.Query
	kotlinCompiledConstructorQuery *sitter.Query
	kotlinCompiledSymbolQuery      *sitter.Query
)

func ensureKotlinQueries() {
	kotlinQueryOnce.Do(func() {
		lang := kotlin.GetLanguage()
		var err error

		kotlinCompiledImportQuery, err = sitter.NewQuery([]byte(kotlinImportQueryPattern), lang)
		if err != nil {
			panic(fmt.Sprintf("failed to compile kotlin import query: %v", err))
		}

		kotlinCompiledFallbackQueries = make([]*sitter.Query, 0, len(kotlinFallbackQueryPatterns))
		for _, pattern := range kotlinFallbackQueryPatterns {
			q, err := sitter.NewQuery([]byte(pattern), lang)
			if err != nil {
				panic(fmt.Sprintf("failed to compile kotlin fallback query: %v", err))
			}
			kotlinCompiledFallbackQueries = append(kotlinCompiledFallbackQueries, q)
		}

		kotlinCompiledPackageQuery, err = sitter.NewQuery([]byte(kotlinPackageQueryPattern), lang)
		if err != nil {
			panic(fmt.Sprintf("failed to compile kotlin package query: %v", err))
		}

		kotlinCompiledTypeIdQuery, err = sitter.NewQuery([]byte("(type_identifier) @type.name"), lang)
		if err != nil {
			panic(fmt.Sprintf("failed to compile kotlin type_identifier query: %v", err))
		}

		kotlinCompiledConstructorQuery, err = sitter.NewQuery([]byte("(call_expression (simple_identifier) @constructor.name)"), lang)
		if err != nil {
			panic(fmt.Sprintf("failed to compile kotlin constructor query: %v", err))
		}

		kotlinCompiledSymbolQuery, err = sitter.NewQuery([]byte("(navigation_expression (simple_identifier) @symbol.name)"), lang)
		if err != nil {
			panic(fmt.Sprintf("failed to compile kotlin symbol query: %v", err))
		}
	})
}

// runQueryKotlinImports executes a pre-compiled tree-sitter query and extracts import paths
func runQueryKotlinImports(rootNode *sitter.Node, sourceCode []byte, query *sitter.Query, pattern string) ([]KotlinImport, error) {
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, rootNode)

	imports := []KotlinImport{}
	projectPackages := make(map[string]bool) // Will be populated later during graph building

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		match = cursor.FilterPredicates(match, sourceCode)

		for _, capture := range match.Captures {
			content := capture.Node.Content(sourceCode)
			importPath := content

			// If this is a full import capture, extract the import path manually
			if strings.Contains(pattern, "@import.full") {
				importPath = extractImportFromFullText(content)
			}

			// Check if this is a wildcard import by looking at the parent import_header node
			isWildcard := false
			parent := capture.Node.Parent()
			if parent != nil && parent.Type() == "import_header" {
				// Check if any child is a wildcard_import node
				for i := 0; i < int(parent.ChildCount()); i++ {
					child := parent.Child(i)
					if child.Type() == "wildcard_import" {
						isWildcard = true
						break
					}
				}
			}

			// Clean the import path (remove "import" keyword if present)
			importPath = strings.TrimPrefix(importPath, "import")
			importPath = strings.TrimSpace(importPath)

			if importPath != "" {
				// For now, classify with empty project packages (will be reclassified during graph building)
				imports = append(imports, classifyKotlinImport(importPath, isWildcard, projectPackages))
			}
		}
	}

	return imports, nil
}

var kotlinImportFullTextPattern = regexp.MustCompile(`import\s+([\w.*]+)(?:\s+as\s+\w+)?`)

// extractImportFromFullText extracts the import path from a full import statement
func extractImportFromFullText(text string) string {
	matches := kotlinImportFullTextPattern.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractPackageDeclaration extracts the package declaration from Kotlin source code
func ExtractPackageDeclaration(sourceCode []byte) string {
	ensureKotlinQueries()

	parser := kotlinParserPool.Get().(*sitter.Parser)
	defer kotlinParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		// Fallback to regex if parsing fails
		return extractPackageWithRegex(sourceCode)
	}
	defer tree.Close()

	// Try tree-sitter query for package header
	pkg, err := runQueryPackageName(tree.RootNode(), sourceCode)
	if err == nil && pkg != "" {
		return pkg
	}

	// Fallback to regex
	return extractPackageWithRegex(sourceCode)
}

// Query pattern for package declaration
const kotlinPackageQueryPattern = `
(package_header
  (identifier) @package.name)
`

// runQueryPackageName executes a pre-compiled tree-sitter query to extract package name
func runQueryPackageName(rootNode *sitter.Node, sourceCode []byte) (string, error) {
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(kotlinCompiledPackageQuery, rootNode)

	match, ok := cursor.NextMatch()
	if !ok {
		return "", fmt.Errorf("no package declaration found")
	}

	match = cursor.FilterPredicates(match, sourceCode)

	if len(match.Captures) > 0 {
		pkg := match.Captures[0].Node.Content(sourceCode)
		return strings.TrimSpace(pkg), nil
	}

	return "", fmt.Errorf("no package name captured")
}

var kotlinPackagePattern = regexp.MustCompile(`package\s+([\w.]+)`)

// extractPackageWithRegex extracts package declaration using regex as fallback
func extractPackageWithRegex(sourceCode []byte) string {
	matches := kotlinPackagePattern.FindSubmatch(sourceCode)
	if len(matches) > 1 {
		return string(matches[1])
	}
	return ""
}

// ClassifyWithProjectPackages reclassifies imports with knowledge of project packages
// This is used during graph building when we know all project packages
func ClassifyWithProjectPackages(imports []KotlinImport, projectPackages map[string]bool) []KotlinImport {
	reclassified := make([]KotlinImport, 0, len(imports))

	for _, imp := range imports {
		path := imp.Path()
		isWildcard := imp.IsWildcard()
		reclassified = append(reclassified, classifyKotlinImport(path, isWildcard, projectPackages))
	}

	return reclassified
}

// ExtractTopLevelTypeNames returns the class/object/interface/typealias names declared at the top level of the file
func ExtractTopLevelTypeNames(sourceCode []byte) []string {
	ensureKotlinQueries()

	parser := kotlinParserPool.Get().(*sitter.Parser)
	defer kotlinParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	var names []string
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "class_declaration", "object_declaration", "interface_declaration", "type_alias":
			if isTopLevelDeclaration(node) {
				if name := extractDeclarationIdentifier(node, sourceCode); name != "" {
					names = append(names, name)
				}
			}
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}

	walk(tree.RootNode())
	return names
}

// ExtractCalledFunctionNames returns the callee names of unqualified call
// expressions (e.g. `foo()` but not `x.foo()`). Used to resolve same-package
// top-level function calls, which ExtractTypeIdentifiers deliberately ignores
// (it captures only type references). Resolution stays precise: a name only
// produces an edge when the package index holds exactly one declaring file.
func ExtractCalledFunctionNames(sourceCode []byte) []string {
	ensureKotlinQueries()

	parser := kotlinParserPool.Get().(*sitter.Parser)
	defer kotlinParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(kotlinCompiledConstructorQuery, tree.RootNode())

	seen := make(map[string]bool)
	var names []string
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			name := strings.TrimSpace(capture.Node.Content(sourceCode))
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

// ExtractTopLevelFunctionNames returns the names of functions declared at the
// top level of the file. Kotlin same-package free-function calls carry no
// import, so these must be indexed for an unqualified call to resolve to its
// defining file.
func ExtractTopLevelFunctionNames(sourceCode []byte) []string {
	ensureKotlinQueries()

	parser := kotlinParserPool.Get().(*sitter.Parser)
	defer kotlinParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	var names []string
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		if node.Type() == "function_declaration" && isTopLevelDeclaration(node) {
			if name := extractDeclarationIdentifier(node, sourceCode); name != "" {
				names = append(names, name)
			}
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}

	walk(tree.RootNode())
	return names
}

// ExtractTypeIdentifiers returns all type identifiers referenced within the file
func ExtractTypeIdentifiers(sourceCode []byte) []string {
	ensureKotlinQueries()

	parser := kotlinParserPool.Get().(*sitter.Parser)
	defer kotlinParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(kotlinCompiledTypeIdQuery, tree.RootNode())

	seen := make(map[string]bool)
	var identifiers []string
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			name := strings.TrimSpace(capture.Node.Content(sourceCode))
			if name != "" && !seen[name] {
				seen[name] = true
				identifiers = append(identifiers, name)
			}
		}
	}

	constructorCursor := sitter.NewQueryCursor()
	defer constructorCursor.Close()
	constructorCursor.Exec(kotlinCompiledConstructorQuery, tree.RootNode())

	for {
		match, ok := constructorCursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			name := strings.TrimSpace(capture.Node.Content(sourceCode))
			if isUpperCamelIdentifier(name) && !seen[name] {
				seen[name] = true
				identifiers = append(identifiers, name)
			}
		}
	}

	symbolCursor := sitter.NewQueryCursor()
	defer symbolCursor.Close()
	symbolCursor.Exec(kotlinCompiledSymbolQuery, tree.RootNode())

	for {
		match, ok := symbolCursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			name := strings.TrimSpace(capture.Node.Content(sourceCode))
			if isUpperCamelIdentifier(name) && !seen[name] {
				seen[name] = true
				identifiers = append(identifiers, name)
			}
		}
	}

	return identifiers
}

func isUpperCamelIdentifier(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// isTopLevelDeclaration checks if a node is declared directly in the source file
func isTopLevelDeclaration(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	switch parent.Type() {
	case "source_file", "script":
		return true
	default:
		return false
	}
}

// extractDeclarationIdentifier returns the identifier for a declaration node
func extractDeclarationIdentifier(node *sitter.Node, sourceCode []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		switch child.Type() {
		case "type_identifier", "simple_identifier":
			return strings.TrimSpace(child.Content(sourceCode))
		}
	}
	return ""
}
