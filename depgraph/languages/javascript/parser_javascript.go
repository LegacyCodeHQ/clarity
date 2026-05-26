package javascript

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// JavaScriptImport represents an import in a JavaScript/JSX file
// (type-only imports are always false for standard JavaScript).
type JavaScriptImport interface {
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

// InternalImport represents an internal project file import (./, ../)
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

var (
	jsSideEffectRE = regexp.MustCompile(`(?m)^\s*import\s*['"]([^'"]+)['"]`)
	jsRequireRE    = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	jsFromPathRE   = regexp.MustCompile(`\bfrom\s*['"]([^'"]+)['"]`)
)

// extractJSImportsFast extracts JavaScript imports using a byte scanner and
// simple regexes without tree-sitter. The two formerly-expensive multiline
// patterns (jsImportFromRE / jsExportFromRE with [\s\S]*?) are replaced by a
// line-oriented scan so the regex engine never has to backtrack across the
// entire file.
func extractJSImportsFast(sourceCode []byte) []JavaScriptImport {
	if !bytes.Contains(sourceCode, []byte("import")) &&
		!bytes.Contains(sourceCode, []byte("export")) &&
		!bytes.Contains(sourceCode, []byte("require")) {
		return []JavaScriptImport{}
	}

	imports := make([]JavaScriptImport, 0, 8)

	// import/export … from 'path': scan line-by-line to avoid O(n²) multiline regex.
	imports = scanImportExportFrom(sourceCode, imports)

	// Side-effect imports: import 'path'
	for _, m := range jsSideEffectRE.FindAllSubmatch(sourceCode, -1) {
		if len(m) >= 2 && len(m[1]) > 0 {
			imports = append(imports, classifyJavaScriptImport(string(m[1]), false))
		}
	}

	// CommonJS require('path')
	for _, m := range jsRequireRE.FindAllSubmatch(sourceCode, -1) {
		if len(m) >= 2 && len(m[1]) > 0 {
			imports = append(imports, classifyJavaScriptImport(string(m[1]), false))
		}
	}

	return imports
}

// scanImportExportFrom walks src line by line. When a line starts with
// "import" or "export" it accumulates lines until it finds a from 'path'
// clause (up to maxStmtLines lookahead) and extracts the module path.
// This replaces the two (?ms)^\s*(import|export)\b[\s\S]*?\bfrom\s*['"]…
// patterns that caused quadratic backtracking on large files.
func scanImportExportFrom(src []byte, imports []JavaScriptImport) []JavaScriptImport {
	const maxStmtLines = 20

	i := 0
	for i < len(src) {
		// Find the end of the current line.
		nl := bytes.IndexByte(src[i:], '\n')
		var lineEnd int
		if nl < 0 {
			lineEnd = len(src)
		} else {
			lineEnd = i + nl
		}

		line := bytes.TrimSpace(src[i:lineEnd])

		startsImport := bytes.HasPrefix(line, []byte("import ")) || bytes.HasPrefix(line, []byte("import{"))
		startsExport := bytes.HasPrefix(line, []byte("export ")) || bytes.HasPrefix(line, []byte("export{"))

		if startsImport || startsExport {
			// Grow the statement window until we find 'from "…"' or run out of
			// lookahead lines.
			stmtEnd := lineEnd
			for k := 0; k < maxStmtLines; k++ {
				segment := src[i:stmtEnd]
				if m := jsFromPathRE.FindSubmatch(segment); m != nil && len(m[1]) > 0 {
					imports = append(imports, classifyJavaScriptImport(string(m[1]), false))
					break
				}
				// Advance to next line end.
				next := bytes.IndexByte(src[stmtEnd:], '\n')
				if next < 0 {
					break
				}
				stmtEnd += next + 1
			}
		}

		i = lineEnd + 1
	}
	return imports
}

// classifyJavaScriptImport classifies a JavaScript import path
func classifyJavaScriptImport(importPath string, isTypeOnly bool) JavaScriptImport {
	// Check for node: prefix (e.g., node:fs)
	if strings.HasPrefix(importPath, "node:") {
		return NodeBuiltinImport{path: importPath, isTypeOnly: isTypeOnly}
	}

	// Check if it's a known Node.js builtin
	if nodeBuiltins[importPath] {
		return NodeBuiltinImport{path: importPath, isTypeOnly: isTypeOnly}
	}

	// Check for relative imports (./ or ../)
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		return InternalImport{path: importPath, isTypeOnly: isTypeOnly}
	}

	// Everything else is an external npm package
	return ExternalImport{path: importPath, isTypeOnly: isTypeOnly}
}

// JavaScriptImports parses a JavaScript/JSX file and returns its imports
func JavaScriptImports(filePath string) ([]JavaScriptImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	isJSX := strings.HasSuffix(filePath, ".jsx")
	return ParseJavaScriptImports(sourceCode, isJSX)
}

// ParseJavaScriptImports parses JavaScript source code and extracts imports
func ParseJavaScriptImports(sourceCode []byte, isJSX bool) ([]JavaScriptImport, error) {
	if fast := extractJSImportsFast(sourceCode); fast != nil {
		return fast, nil
	}

	// tree-sitter-javascript supports JSX; keep explicit mode for clarity.
	var lang *sitter.Language
	if isJSX {
		lang = javascript.GetLanguage()
	} else {
		lang = javascript.GetLanguage()
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JavaScript code: %w", err)
	}
	defer tree.Close()

	return extractImportsFromTree(tree.RootNode(), sourceCode, lang)
}

// extractImportsFromTree walks the AST and extracts imports
func extractImportsFromTree(rootNode *sitter.Node, sourceCode []byte, lang *sitter.Language) ([]JavaScriptImport, error) {
	var imports []JavaScriptImport

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

	// Query for CommonJS require calls: require('module')
	requireQuery := `
(call_expression
  function: (identifier) @require.fn
  arguments: (arguments (string) @require.source)
  (#eq? @require.fn "require"))
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

	// Execute CommonJS require query
	requireResults, err := executeQuery(rootNode, sourceCode, lang, requireQuery)
	if err == nil {
		imports = append(imports, requireResults...)
	}

	// If queries fail, fall back to manual tree traversal
	if len(imports) == 0 {
		imports = extractImportsManually(rootNode, sourceCode)
	}

	return imports, nil
}

// executeQuery runs a tree-sitter query and extracts imports
func executeQuery(rootNode *sitter.Node, sourceCode []byte, lang *sitter.Language, pattern string) ([]JavaScriptImport, error) {
	query, err := sitter.NewQuery([]byte(pattern), lang)
	if err != nil {
		return nil, fmt.Errorf("failed to create query: %w", err)
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	cursor.Exec(query, rootNode)

	var imports []JavaScriptImport

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}

		match = cursor.FilterPredicates(match, sourceCode)

		for _, capture := range match.Captures {
			captureName := query.CaptureNameForId(capture.Index)
			if !strings.HasSuffix(captureName, ".source") {
				continue
			}

			content := capture.Node.Content(sourceCode)
			importPath := cleanImportPath(content)

			if importPath != "" {
				// JavaScript doesn't have type-only imports, but keep the shape consistent.
				isTypeOnly := isTypeOnlyImport(capture.Node, sourceCode)
				imports = append(imports, classifyJavaScriptImport(importPath, isTypeOnly))
			}
		}
	}

	return imports, nil
}

// extractImportsManually walks the AST manually to extract imports
func extractImportsManually(node *sitter.Node, sourceCode []byte) []JavaScriptImport {
	var imports []JavaScriptImport

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Handle import_statement and export_statement
		if nodeType == "import_statement" || nodeType == "export_statement" {
			isTypeOnly := false

			// Check for type-only imports (Flow/TS syntax) if present
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
						imports = append(imports, classifyJavaScriptImport(importPath, isTypeOnly))
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

// ResolveJavaScriptImportPath resolves a JavaScript import path to possible file paths
func ResolveJavaScriptImportPath(sourceFile, importPath string, suppliedFiles map[string]bool) []string {
	sourceDir := filepath.Dir(sourceFile)

	// Resolve the import path relative to the source file
	basePath := filepath.Join(sourceDir, importPath)
	basePath = filepath.Clean(basePath)

	var resolvedPaths []string

	// JavaScript extension resolution order
	extensions := []string{".js", ".jsx", ".mjs", ".cjs"}

	// Try direct path with extensions
	for _, ext := range extensions {
		candidate := basePath + ext
		if suppliedFiles[candidate] {
			resolvedPaths = append(resolvedPaths, candidate)
		}
	}

	// Try index file resolution (./utils -> ./utils/index.js)
	indexPaths := []string{
		filepath.Join(basePath, "index.js"),
		filepath.Join(basePath, "index.jsx"),
		filepath.Join(basePath, "index.mjs"),
		filepath.Join(basePath, "index.cjs"),
	}

	for _, indexPath := range indexPaths {
		if suppliedFiles[indexPath] {
			resolvedPaths = append(resolvedPaths, indexPath)
		}
	}

	// Honour any explicit extension (e.g. ".js", ".css", ".json"). The
	// suppliedFiles filter is the source of truth for what's actually in the
	// project, so resolving non-JS imports such as CSS side-effect imports is
	// safe and produces the expected edges in the graph.
	if filepath.Ext(importPath) != "" {
		if suppliedFiles[basePath] {
			resolvedPaths = append(resolvedPaths, basePath)
		}
	}

	return resolvedPaths
}
