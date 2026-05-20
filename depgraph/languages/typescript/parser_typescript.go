package typescript

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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

const tsImportQueryPattern = `
(import_statement
  source: (string) @import.source)
`

const tsExportQueryPattern = `
(export_statement
  source: (string) @export.source)
`

var (
	tsTypescriptLang = typescript.GetLanguage()
	tsTSXLang        = tsx.GetLanguage()
	tsImportQueryTS  *sitter.Query
	tsExportQueryTS  *sitter.Query
	tsImportQueryTSX *sitter.Query
	tsExportQueryTSX *sitter.Query
	tsParserPoolTS   = sync.Pool{
		New: func() any {
			p := sitter.NewParser()
			p.SetLanguage(tsTypescriptLang)
			return p
		},
	}
	tsParserPoolTSX = sync.Pool{
		New: func() any {
			p := sitter.NewParser()
			p.SetLanguage(tsTSXLang)
			return p
		},
	}
)

func init() {
	var err error
	tsImportQueryTS, err = sitter.NewQuery([]byte(tsImportQueryPattern), tsTypescriptLang)
	if err != nil {
		panic("failed to compile ts import query: " + err.Error())
	}
	tsExportQueryTS, err = sitter.NewQuery([]byte(tsExportQueryPattern), tsTypescriptLang)
	if err != nil {
		panic("failed to compile ts export query: " + err.Error())
	}
	tsImportQueryTSX, err = sitter.NewQuery([]byte(tsImportQueryPattern), tsTSXLang)
	if err != nil {
		panic("failed to compile tsx import query: " + err.Error())
	}
	tsExportQueryTSX, err = sitter.NewQuery([]byte(tsExportQueryPattern), tsTSXLang)
	if err != nil {
		panic("failed to compile tsx export query: " + err.Error())
	}
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
	typeImportFromRE = regexp.MustCompile(`(?ms)^\s*import\s+type\b[\s\S]*?\bfrom\s*['"]([^'"]+)['"]`)
	importFromRE     = regexp.MustCompile(`(?ms)^\s*import\b[\s\S]*?\bfrom\s*['"]([^'"]+)['"]`)
	sideEffectRE     = regexp.MustCompile(`(?m)^\s*import\s*['"]([^'"]+)['"]`)
	exportFromRE     = regexp.MustCompile(`(?ms)^\s*export\b[\s\S]*?\bfrom\s*['"]([^'"]+)['"]`)
)

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
	if fast := extractImportsFast(sourceCode); fast != nil {
		return fast, nil
	}

	var pool *sync.Pool
	if isTSX {
		pool = &tsParserPoolTSX
	} else {
		pool = &tsParserPoolTS
	}

	parser, _ := pool.Get().(*sitter.Parser)
	if parser == nil {
		var lang *sitter.Language
		if isTSX {
			lang = tsTSXLang
		} else {
			lang = tsTypescriptLang
		}
		parser = sitter.NewParser()
		parser.SetLanguage(lang)
	}
	defer pool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TypeScript code: %w", err)
	}
	defer tree.Close()

	return extractImportsFromTree(tree.RootNode(), sourceCode, isTSX)
}

func extractImportsFast(sourceCode []byte) []TypeScriptImport {
	if !bytes.Contains(sourceCode, []byte("import")) && !bytes.Contains(sourceCode, []byte("export")) {
		return []TypeScriptImport{}
	}

	imports := make([]TypeScriptImport, 0, 8)

	for _, m := range typeImportFromRE.FindAllSubmatch(sourceCode, -1) {
		if len(m) < 2 {
			continue
		}
		importPath := cleanImportPath(string(m[1]))
		if importPath == "" {
			continue
		}
		imports = append(imports, classifyTypeScriptImport(importPath, true))
	}

	for _, m := range importFromRE.FindAllSubmatch(sourceCode, -1) {
		if len(m) < 2 {
			continue
		}
		if bytes.HasPrefix(bytes.TrimSpace(m[0]), []byte("import type")) {
			continue
		}
		importPath := cleanImportPath(string(m[1]))
		if importPath == "" {
			continue
		}
		imports = append(imports, classifyTypeScriptImport(importPath, false))
	}

	for _, m := range sideEffectRE.FindAllSubmatch(sourceCode, -1) {
		if len(m) < 2 {
			continue
		}
		importPath := cleanImportPath(string(m[1]))
		if importPath == "" {
			continue
		}
		imports = append(imports, classifyTypeScriptImport(importPath, false))
	}

	for _, m := range exportFromRE.FindAllSubmatch(sourceCode, -1) {
		if len(m) < 2 {
			continue
		}
		importPath := cleanImportPath(string(m[1]))
		if importPath == "" {
			continue
		}
		imports = append(imports, classifyTypeScriptImport(importPath, false))
	}

	return imports
}

// extractImportsFromTree walks the AST and extracts imports
func extractImportsFromTree(rootNode *sitter.Node, sourceCode []byte, isTSX bool) ([]TypeScriptImport, error) {
	var imports []TypeScriptImport

	var importQ, exportQ *sitter.Query
	var lang *sitter.Language
	if isTSX {
		importQ = tsImportQueryTSX
		exportQ = tsExportQueryTSX
		lang = tsTSXLang
	} else {
		importQ = tsImportQueryTS
		exportQ = tsExportQueryTS
		lang = tsTypescriptLang
	}

	// Execute import query
	importResults, err := executeQuery(rootNode, sourceCode, lang, importQ)
	if err == nil {
		imports = append(imports, importResults...)
	}

	// Execute export query
	exportResults, err := executeQuery(rootNode, sourceCode, lang, exportQ)
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
func executeQuery(rootNode *sitter.Node, sourceCode []byte, lang *sitter.Language, query *sitter.Query) ([]TypeScriptImport, error) {
	if query == nil {
		return nil, fmt.Errorf("query is nil")
	}

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
	basePaths := resolveTypeScriptBasePaths(sourceFile, importPath)
	if len(basePaths) == 0 {
		return nil
	}

	var resolvedPaths []string
	seen := make(map[string]bool)
	add := func(p string) {
		if suppliedFiles[p] && !seen[p] {
			seen[p] = true
			resolvedPaths = append(resolvedPaths, p)
		}
	}

	extensions := []string{".ts", ".tsx", ".js", ".jsx"}

	for _, basePath := range basePaths {
		// In TS/TSX source, explicit runtime .js/.jsx specifiers often point to .ts/.tsx files.
		for _, candidate := range sourceCandidatesForJSImport(basePath) {
			add(candidate)
		}

		for _, ext := range extensions {
			add(basePath + ext)
		}

		// Index file resolution (./utils -> ./utils/index.ts)
		add(filepath.Join(basePath, "index.ts"))
		add(filepath.Join(basePath, "index.tsx"))
		add(filepath.Join(basePath, "index.js"))
		add(filepath.Join(basePath, "index.jsx"))

		if hasTypeScriptExtension(importPath) {
			add(basePath)
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

// resolveTypeScriptBasePaths returns candidate base paths (no extension) for
// an import. It returns multiple candidates because alias resolution depends
// on project layout (src-rooted vs project-root, e.g. Next.js App Router) and
// optionally on tsconfig.json paths. The caller filters candidates against
// the set of supplied project files.
func resolveTypeScriptBasePaths(sourceFile, importPath string) []string {
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		sourceDir := filepath.Dir(sourceFile)
		return []string{filepath.Clean(filepath.Join(sourceDir, importPath))}
	}

	if !strings.HasPrefix(importPath, "@/") {
		// Could be an npm workspace package (e.g. "@tanstack/query-core" or
		// "lodash-es" inside a yarn/pnpm workspace).
		if bases := resolveWorkspaceBasePaths(sourceFile, importPath); len(bases) > 0 {
			return bases
		}
		// Fall back to the legacy behaviour of joining against the source
		// dir. Harmless if nothing resolves; preserves any external-but-vendored
		// patterns that happened to work before this change.
		sourceDir := filepath.Dir(sourceFile)
		return []string{filepath.Clean(filepath.Join(sourceDir, importPath))}
	}

	var bases []string
	seen := make(map[string]bool)
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			bases = append(bases, p)
		}
	}

	// 1. tsconfig.json / jsconfig.json paths (canonical mapping).
	if cfg := loadTsConfigFor(sourceFile); cfg != nil {
		for _, p := range cfg.resolveAlias(importPath) {
			add(p)
		}
	}

	rest := strings.TrimPrefix(importPath, "@/")

	// 2. Legacy heuristic: project's "src/" directory.
	if srcRoot, ok := projectSrcRootFromSourceFile(sourceFile); ok {
		add(filepath.Clean(filepath.Join(srcRoot, rest)))
	}

	// 3. Each ancestor of the source file as a potential project root.
	// Covers Next.js / Vite layouts where "@/" maps to the repo root and
	// no tsconfig is reachable on disk (e.g. unit tests with synthetic paths).
	dir := filepath.Dir(sourceFile)
	for {
		add(filepath.Clean(filepath.Join(dir, rest)))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return bases
}

// resolveWorkspaceBasePaths returns candidate base paths (no extension) when
// importPath names a sibling workspace package, e.g. "@tanstack/query-core"
// in a pnpm workspace. Returns nil if importPath is not a workspace
// package or no workspace root is reachable from sourceFile.
//
// For a bare-package import (e.g. "@tanstack/query-core"), candidates are:
//   - <pkgDir>/<main-without-extension>  (when package.json declares main)
//   - <pkgDir>/src/index
//   - <pkgDir>/index
//
// For a sub-path import (e.g. "@tanstack/query-core/devtools"), candidates are:
//   - <pkgDir>/<subpath>
//   - <pkgDir>/src/<subpath>          (common in JS monorepos that ship src/)
//   - <pkgDir>/dist/<subpath>         (common compiled-output layout)
//
// Extension probing + index resolution is layered on top by
// ResolveTypeScriptImportPath.
func resolveWorkspaceBasePaths(sourceFile, importPath string) []string {
	ws := loadNpmWorkspaceFor(sourceFile)
	if ws == nil {
		return nil
	}
	pkgDir, subpath, ok := ws.resolveWorkspacePackage(importPath)
	if !ok {
		return nil
	}

	var bases []string
	seen := make(map[string]bool)
	add := func(p string) {
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			bases = append(bases, p)
		}
	}

	if subpath == "" {
		// Bare-package import. Try the package.json main field first, then
		// common JS / TS conventions.
		if main := readPackageMain(pkgDir); main != "" {
			add(filepath.Join(pkgDir, stripKnownExtension(main)))
		}
		add(filepath.Join(pkgDir, "src", "index"))
		add(filepath.Join(pkgDir, "index"))
		return bases
	}

	// Subpath import. The most common layouts in JS monorepos are
	// pkg/<subpath> (subpath maps directly into src/), pkg/src/<subpath>
	// (subpath is declared without the src/ prefix), or pkg/dist/<subpath>.
	add(filepath.Join(pkgDir, subpath))
	add(filepath.Join(pkgDir, "src", subpath))
	add(filepath.Join(pkgDir, "dist", subpath))
	return bases
}

// readPackageMain returns the value of the "main" field from a package.json
// living in pkgDir, or "" if absent.
func readPackageMain(pkgDir string) string {
	data, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err != nil {
		return ""
	}
	var raw struct {
		Main string `json:"main"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return strings.TrimSpace(raw.Main)
}

// stripKnownExtension removes a trailing .js/.jsx/.ts/.tsx/.mjs/.cjs from a
// path so that the resolver's extension-probing logic can append fresh
// candidates without duplicating work.
func stripKnownExtension(path string) string {
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
		if strings.HasSuffix(path, ext) {
			return strings.TrimSuffix(path, ext)
		}
	}
	return path
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
