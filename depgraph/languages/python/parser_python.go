package python

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

var (
	pythonLanguage   = python.GetLanguage()
	pythonParserPool = sync.Pool{
		New: func() any {
			parser := sitter.NewParser()
			parser.SetLanguage(pythonLanguage)
			return parser
		},
	}
)

// PythonImport represents an import in a Python file.
//
// FallbackPath is the path to try when Path does not resolve to a real file.
// It is only non-empty for `from X import name` statements: name might be a
// submodule of X (Path: "X.name") or an attribute defined inside X's own
// file/package (FallbackPath: "X") -- only a file-existence check can tell
// them apart, and the parser doesn't have that information, so both
// candidates are threaded through to the resolver.
type PythonImport interface {
	Path() string
	FallbackPath() string
	IsTypeOnly() bool
}

// ExternalImport represents an external module import.
type ExternalImport struct {
	path         string
	fallbackPath string
	isTypeOnly   bool
}

func (e ExternalImport) Path() string {
	return e.path
}

func (e ExternalImport) FallbackPath() string {
	return e.fallbackPath
}

func (e ExternalImport) IsTypeOnly() bool {
	return e.isTypeOnly
}

// InternalImport represents a relative module import.
type InternalImport struct {
	path         string
	fallbackPath string
	isTypeOnly   bool
}

func (i InternalImport) Path() string {
	return i.path
}

func (i InternalImport) FallbackPath() string {
	return i.fallbackPath
}

func (i InternalImport) IsTypeOnly() bool {
	return i.isTypeOnly
}

// classifyPythonImport classifies a Python import path.
func classifyPythonImport(importPath string, isTypeOnly bool) PythonImport {
	return classifyPythonImportWithFallback(importPath, "", isTypeOnly)
}

// classifyPythonImportWithFallback classifies a Python import path that has
// a fallback to try if importPath does not resolve to a real file.
func classifyPythonImportWithFallback(importPath, fallbackPath string, isTypeOnly bool) PythonImport {
	if strings.HasPrefix(importPath, ".") {
		return InternalImport{path: importPath, fallbackPath: fallbackPath, isTypeOnly: isTypeOnly}
	}

	return ExternalImport{path: importPath, fallbackPath: fallbackPath, isTypeOnly: isTypeOnly}
}

// parsePythonImportsFast extracts Python imports with a simple byte scanner, avoiding tree-sitter.
// Returns (imports, true) on success, (nil, false) when the file requires tree-sitter fallback.
func parsePythonImportsFast(src []byte) ([]PythonImport, bool) {
	// Triple-quoted strings can contain import-like text; bail to tree-sitter.
	if bytes.Contains(src, []byte(`"""`)) || bytes.Contains(src, []byte("'''")) {
		return nil, false
	}

	var imports []PythonImport
	remaining := src

	for len(remaining) > 0 {
		nl := bytes.IndexByte(remaining, '\n')
		var line []byte
		if nl < 0 {
			line = remaining
			remaining = nil
		} else {
			line = remaining[:nl]
			remaining = remaining[nl+1:]
		}

		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || trimmed[0] == '#' {
			continue
		}

		// Backslash continuation — bail to tree-sitter.
		if trimmed[len(trimmed)-1] == '\\' {
			return nil, false
		}

		if bytes.HasPrefix(trimmed, []byte("import ")) {
			mods := trimmed[7:]
			for _, part := range bytes.Split(mods, []byte(",")) {
				mod := bytes.TrimSpace(part)
				if idx := bytes.Index(mod, []byte(" as ")); idx >= 0 {
					mod = bytes.TrimSpace(mod[:idx])
				} else if idx := bytes.IndexByte(mod, ' '); idx >= 0 {
					mod = mod[:idx]
				}
				if len(mod) > 0 {
					imports = append(imports, classifyPythonImport(string(mod), false))
				}
			}
			continue
		}

		if bytes.HasPrefix(trimmed, []byte("from ")) {
			// Every `from X import name` needs submodule-first/module-fallback
			// resolution (name might be a submodule of X or an attribute
			// defined inside X itself — only a file-existence check downstream
			// can tell), so there is no shortcut left worth hand-rolling here.
			// Bail to tree-sitter for the whole file.
			return nil, false
		}
	}

	return imports, true
}

// PythonImports parses a Python file and returns its imports.
func PythonImports(filePath string) ([]PythonImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParsePythonImports(sourceCode)
}

// ParsePythonImports parses Python source code and extracts imports.
func ParsePythonImports(sourceCode []byte) ([]PythonImport, error) {
	if imports, ok := parsePythonImportsFast(sourceCode); ok {
		return imports, nil
	}

	parser, _ := pythonParserPool.Get().(*sitter.Parser)
	if parser == nil {
		parser = sitter.NewParser()
		parser.SetLanguage(pythonLanguage)
	}
	defer pythonParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Python code: %w", err)
	}
	defer tree.Close()

	return extractImportsFromTree(tree.RootNode(), sourceCode), nil
}

// extractImportsFromTree walks the AST and extracts imports.
func extractImportsFromTree(rootNode *sitter.Node, sourceCode []byte) []PythonImport {
	var imports []PythonImport

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		switch n.Type() {
		case "import_statement":
			modules := extractImportStatementModules(n, sourceCode)
			for _, module := range modules {
				if module != "" {
					imports = append(imports, classifyPythonImport(module, false))
				}
			}
		case "import_from_statement", "future_import_statement":
			for _, target := range extractImportFromModules(n, sourceCode) {
				if target.submodule != "" {
					imports = append(imports, classifyPythonImportWithFallback(target.submodule, target.fallback, false))
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(rootNode)
	return imports
}

func extractImportStatementModules(node *sitter.Node, sourceCode []byte) []string {
	var modules []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		module := extractModuleName(child, sourceCode)
		if module != "" {
			modules = append(modules, module)
		}
	}
	return modules
}

// fromImportTarget is one name imported by a `from X import ...` statement.
// Submodule is the path to try first, treating the name as a submodule of X
// (e.g. "_pytest.nodes" or ".packages"). Fallback is the plain module path
// (e.g. "_pytest" or ".") to try if Submodule does not resolve to a real
// file -- the name is then an attribute defined inside X's own file/package,
// not a submodule (e.g. `from . import logger` where logger is an instance
// defined in the package's __init__.py, not a logger.py file).
type fromImportTarget struct {
	submodule string
	fallback  string
}

// extractImportFromModules returns the import target(s) for a `from X import
// ...` statement. Each name after "import" gets its own submodule-shaped
// candidate plus a fallback to the bare module, since only a file-existence
// check (done downstream, where suppliedFiles is available) can tell whether
// a name is a submodule of X or an attribute defined inside X itself.
// `from X import *` is left as a single module-only target with no fallback:
// a wildcard pulls from X's own namespace, which is what the module path
// already resolves to.
func extractImportFromModules(node *sitter.Node, sourceCode []byte) []fromImportTarget {
	var module string
	importIdx := -1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "import" {
			importIdx = i
			break
		}
		switch child.Type() {
		case "relative_import", "dotted_name":
			module = strings.TrimSpace(child.Content(sourceCode))
		}
	}

	if module == "" || importIdx < 0 {
		return []fromImportTarget{{submodule: module}}
	}

	// Bare dots concatenate directly with the name ("." + "packages" =
	// ".packages"); a module that already has a name of its own needs a
	// literal separator (".models" + "." + "Response" = ".models.Response").
	sep := "."
	if strings.HasSuffix(module, ".") {
		sep = ""
	}

	var names []string
	for i := importIdx + 1; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "wildcard_import":
			return []fromImportTarget{{submodule: module}}
		case "dotted_name", "aliased_import":
			if name := extractModuleName(child, sourceCode); name != "" {
				names = append(names, name)
			}
		}
	}

	if len(names) == 0 {
		return []fromImportTarget{{submodule: module}}
	}

	targets := make([]fromImportTarget, len(names))
	for i, name := range names {
		targets[i] = fromImportTarget{submodule: module + sep + name, fallback: module}
	}
	return targets
}

func extractModuleName(node *sitter.Node, sourceCode []byte) string {
	switch node.Type() {
	case "dotted_name", "identifier", "relative_import":
		return strings.TrimSpace(node.Content(sourceCode))
	case "aliased_import":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			if child.Type() == "dotted_name" || child.Type() == "identifier" {
				return strings.TrimSpace(child.Content(sourceCode))
			}
		}
	}
	return ""
}

// ResolvePythonImportPath resolves a Python import path to possible file paths.
func ResolvePythonImportPath(sourceFile, importPath string, suppliedFiles map[string]bool) []string {
	if !strings.HasPrefix(importPath, ".") {
		return nil
	}

	sourceDir := filepath.Dir(sourceFile)
	dotCount := 0
	for i := 0; i < len(importPath); i++ {
		if importPath[i] != '.' {
			break
		}
		dotCount++
	}

	baseDir := sourceDir
	for i := 0; i < dotCount-1; i++ {
		baseDir = filepath.Dir(baseDir)
	}

	modulePath := strings.TrimLeft(importPath, ".")
	modulePath = strings.ReplaceAll(modulePath, ".", string(filepath.Separator))

	var resolvedPaths []string

	if modulePath == "" {
		candidate := filepath.Join(baseDir, "__init__.py")
		if suppliedFiles[candidate] {
			resolvedPaths = append(resolvedPaths, candidate)
		}
		return resolvedPaths
	}

	fileCandidate := filepath.Join(baseDir, modulePath) + ".py"
	if suppliedFiles[fileCandidate] {
		resolvedPaths = append(resolvedPaths, fileCandidate)
	}

	packageCandidate := filepath.Join(baseDir, modulePath, "__init__.py")
	if suppliedFiles[packageCandidate] {
		resolvedPaths = append(resolvedPaths, packageCandidate)
	}

	return resolvedPaths
}

// ResolvePythonAbsoluteImportPath resolves an absolute Python package import
// (e.g. "dexter.tools.finance.api") to matching project files.
func ResolvePythonAbsoluteImportPath(absSourcePath, importPath string, suppliedFiles map[string]bool) []string {
	if importPath == "" || strings.HasPrefix(importPath, ".") {
		return nil
	}

	modulePath := strings.ReplaceAll(importPath, ".", string(filepath.Separator))
	fileSuffix := string(filepath.Separator) + modulePath + ".py"
	packageSuffix := string(filepath.Separator) + filepath.Join(modulePath, "__init__.py")

	var resolvedPaths []string
	for suppliedPath := range suppliedFiles {
		if suppliedPath == absSourcePath {
			continue
		}
		if strings.HasSuffix(suppliedPath, fileSuffix) || strings.HasSuffix(suppliedPath, packageSuffix) {
			resolvedPaths = append(resolvedPaths, suppliedPath)
		}
	}

	sort.Strings(resolvedPaths)
	return resolvedPaths
}
