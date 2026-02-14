package typescript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TypeScriptImport represents an import in a TypeScript/TSX file
type TypeScriptImport interface {
	Path() string
	IsTypeOnly() bool
}

// NodeBuiltinImport represents a Node.js built-in module import (fs, path, http, node:fs)
type NodeBuiltinImport struct {
	path       string
	isTypeOnly bool
}

func (n NodeBuiltinImport) Path() string {
	return n.path
}

func (n NodeBuiltinImport) IsTypeOnly() bool {
	return n.isTypeOnly
}

// ExternalImport represents an external npm package import
type ExternalImport struct {
	path       string
	isTypeOnly bool
}

func (e ExternalImport) Path() string {
	return e.path
}

func (e ExternalImport) IsTypeOnly() bool {
	return e.isTypeOnly
}

// InternalImport represents an internal project file import (./, ../, @/)
type InternalImport struct {
	path       string
	isTypeOnly bool
}

func (i InternalImport) Path() string {
	return i.path
}

func (i InternalImport) IsTypeOnly() bool {
	return i.isTypeOnly
}

// nodeBuiltins contains known Node.js built-in module names
var nodeBuiltins = map[string]bool{
	"assert":         true,
	"buffer":         true,
	"child_process":  true,
	"cluster":        true,
	"crypto":         true,
	"dgram":          true,
	"dns":            true,
	"events":         true,
	"fs":             true,
	"http":           true,
	"https":          true,
	"net":            true,
	"os":             true,
	"path":           true,
	"querystring":    true,
	"readline":       true,
	"stream":         true,
	"string_decoder": true,
	"timers":         true,
	"tls":            true,
	"tty":            true,
	"url":            true,
	"util":           true,
	"v8":             true,
	"vm":             true,
	"zlib":           true,
	"worker_threads": true,
	"perf_hooks":     true,
	"async_hooks":    true,
	"fs/promises":    true,
	"path/posix":     true,
	"path/win32":     true,
}

// classifyTypeScriptImport classifies a TypeScript import path
func classifyTypeScriptImport(importPath string, isTypeOnly bool) TypeScriptImport {
	// Check for node: prefix (e.g., node:fs)
	if strings.HasPrefix(importPath, "node:") {
		return NodeBuiltinImport{path: importPath, isTypeOnly: isTypeOnly}
	}

	// Check if it's a known Node.js builtin
	if nodeBuiltins[importPath] {
		return NodeBuiltinImport{path: importPath, isTypeOnly: isTypeOnly}
	}

	// Check for relative imports (./ or ../) and common TS alias imports (@/)
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") || strings.HasPrefix(importPath, "@/") {
		return InternalImport{path: importPath, isTypeOnly: isTypeOnly}
	}

	// Everything else is an external npm package
	return ExternalImport{path: importPath, isTypeOnly: isTypeOnly}
}

// TypeScriptImports parses a TypeScript/TSX file and returns its imports
func TypeScriptImports(filePath string) ([]TypeScriptImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	isTSX := strings.HasSuffix(filePath, ".tsx")
	return ParseTypeScriptImports(sourceCode, isTSX)
}

// ParseTypeScriptImports parses TypeScript source code and extracts imports
func ParseTypeScriptImports(sourceCode []byte, isTSX bool) ([]TypeScriptImport, error) {
	var lang *sitter.Language
	if isTSX {
		lang = tsx.GetLanguage()
	} else {
		lang = typescript.GetLanguage()
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TypeScript code: %w", err)
	}
	defer tree.Close()

	return extractImportsFromTree(tree.RootNode(), sourceCode, lang)
}

// extractImportsFromTree walks the AST and extracts imports
func extractImportsFromTree(rootNode *sitter.Node, sourceCode []byte, lang *sitter.Language) ([]TypeScriptImport, error) {
	var imports []TypeScriptImport

	// Query for import statements: import ... from 'module'
	importQuery := `
(import_statement
  source: (string) @import.source)
`

	// Query for export statements with source: export ... from 'module'
	exportQuery := `
(export_statement
  source: (string) @export.source)
`

	// Execute import query
	importResults, err := executeQuery(rootNode, sourceCode, lang, importQuery)
	if err == nil {
		imports = append(imports, importResults...)
	}

	// Execute export query
	exportResults, err := executeQuery(rootNode, sourceCode, lang, exportQuery)
	if err == nil {
		imports = append(imports, exportResults...)
	}

	// If queries fail, fall back to manual tree traversal
	if len(imports) == 0 {
		imports = extractImportsManually(rootNode, sourceCode)
	}

	return imports, nil
}

// executeQuery runs a tree-sitter query and extracts imports
func executeQuery(rootNode *sitter.Node, sourceCode []byte, lang *sitter.Language, pattern string) ([]TypeScriptImport, error) {
	query, err := sitter.NewQuery([]byte(pattern), lang)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, rootNode)

	var imports []TypeScriptImport

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		match = cursor.FilterPredicates(match, sourceCode)

		for _, capture := range match.Captures {
			content := capture.Node.Content(sourceCode)
			importPath := cleanImportPath(content)

			if importPath != "" {
				// Check if this is a type-only import by looking at the parent
				isTypeOnly := isTypeOnlyImport(capture.Node, sourceCode)
				imports = append(imports, classifyTypeScriptImport(importPath, isTypeOnly))
			}
		}
	}

	return imports, nil
}

// extractImportsManually walks the AST manually to extract imports
func extractImportsManually(node *sitter.Node, sourceCode []byte) []TypeScriptImport {
	var imports []TypeScriptImport

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Handle import_statement and export_statement
		if nodeType == "import_statement" || nodeType == "export_statement" {
			isTypeOnly := false

			// Check for type-only imports
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child != nil && child.Type() == "type" {
					isTypeOnly = true
					break
				}
			}

			// Find the source string
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child != nil && child.Type() == "string" {
					content := child.Content(sourceCode)
					importPath := cleanImportPath(content)
					if importPath != "" {
						imports = append(imports, classifyTypeScriptImport(importPath, isTypeOnly))
					}
					break
				}
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return imports
}

// isTypeOnlyImport checks if an import is a type-only import
func isTypeOnlyImport(node *sitter.Node, sourceCode []byte) bool {
	// Walk up to find the import_statement parent
	parent := node.Parent()
	for parent != nil {
		if parent.Type() == "import_statement" {
			// Check if there's a "type" keyword child
			for i := 0; i < int(parent.ChildCount()); i++ {
				child := parent.Child(i)
				if child != nil {
					content := child.Content(sourceCode)
					if content == "type" {
						return true
					}
				}
			}
			return false
		}
		parent = parent.Parent()
	}
	return false
}

// cleanImportPath removes quotes from import path strings
func cleanImportPath(raw string) string {
	// Remove single or double quotes
	cleaned := strings.Trim(raw, "'\"")
	return strings.TrimSpace(cleaned)
}

// ResolveTypeScriptImportPath resolves a TypeScript import path to possible file paths
func ResolveTypeScriptImportPath(sourceFile, importPath string, suppliedFiles map[string]bool) []string {
	basePath, ok := resolveTypeScriptBasePath(sourceFile, importPath)
	if !ok {
		return nil
	}

	var resolvedPaths []string

	// TypeScript extension resolution order
	extensions := []string{".ts", ".tsx", ".js", ".jsx"}

	// In TS/TSX source, explicit runtime .js/.jsx specifiers often point to .ts/.tsx files.
	// Try source-file variants first to avoid missing project edges in mixed module setups.
	if sourceCandidates := sourceCandidatesForJSImport(basePath); len(sourceCandidates) > 0 {
		for _, candidate := range sourceCandidates {
			if suppliedFiles[candidate] {
				resolvedPaths = append(resolvedPaths, candidate)
			}
		}
	}

	// Try direct path with extensions
	for _, ext := range extensions {
		candidate := basePath + ext
		if suppliedFiles[candidate] {
			resolvedPaths = append(resolvedPaths, candidate)
		}
	}

	// Try index file resolution (./utils -> ./utils/index.ts)
	indexPaths := []string{
		filepath.Join(basePath, "index.ts"),
		filepath.Join(basePath, "index.tsx"),
		filepath.Join(basePath, "index.js"),
		filepath.Join(basePath, "index.jsx"),
	}

	for _, indexPath := range indexPaths {
		if suppliedFiles[indexPath] {
			resolvedPaths = append(resolvedPaths, indexPath)
		}
	}

	// If import already has an extension, try the exact path
	if hasTypeScriptExtension(importPath) {
		exactPath := basePath
		if suppliedFiles[exactPath] {
			resolvedPaths = append(resolvedPaths, exactPath)
		}
	}

	return resolvedPaths
}

// hasTypeScriptExtension checks if a path already has a TypeScript/JavaScript extension
func hasTypeScriptExtension(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx"
}

func sourceCandidatesForJSImport(basePath string) []string {
	ext := filepath.Ext(basePath)
	stem := strings.TrimSuffix(basePath, ext)
	switch ext {
	case ".js":
		return []string{stem + ".ts", stem + ".tsx"}
	case ".jsx":
		return []string{stem + ".tsx", stem + ".ts"}
	default:
		return nil
	}
}

func resolveTypeScriptBasePath(sourceFile, importPath string) (string, bool) {
	// Resolve common alias format "@/..." to "<repo>/src/..."
	if strings.HasPrefix(importPath, "@/") {
		srcRoot, ok := projectSrcRootFromSourceFile(sourceFile)
		if !ok {
			return "", false
		}
		return filepath.Clean(filepath.Join(srcRoot, strings.TrimPrefix(importPath, "@/"))), true
	}

	sourceDir := filepath.Dir(sourceFile)
	return filepath.Clean(filepath.Join(sourceDir, importPath)), true
}

func projectSrcRootFromSourceFile(sourceFile string) (string, bool) {
	sourceDir := filepath.Clean(filepath.Dir(sourceFile))
	marker := string(filepath.Separator) + "src" + string(filepath.Separator)

	if idx := strings.LastIndex(sourceDir, marker); idx >= 0 {
		return sourceDir[:idx+len(marker)-1], true
	}

	if strings.HasSuffix(sourceDir, string(filepath.Separator)+"src") {
		return sourceDir, true
	}

	return "", false
}
