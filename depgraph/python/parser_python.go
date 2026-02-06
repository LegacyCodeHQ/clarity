package python

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// PythonImport represents an import in a Python file.
type PythonImport interface {
	Path() string
	IsTypeOnly() bool
}

// ExternalImport represents an external module import.
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

// InternalImport represents a relative module import.
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

// classifyPythonImport classifies a Python import path.
func classifyPythonImport(importPath string, isTypeOnly bool) PythonImport {
	if strings.HasPrefix(importPath, ".") {
		return InternalImport{path: importPath, isTypeOnly: isTypeOnly}
	}

	return ExternalImport{path: importPath, isTypeOnly: isTypeOnly}
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
	lang := python.GetLanguage()

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

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
			module := extractImportFromModule(n, sourceCode)
			if module != "" {
				imports = append(imports, classifyPythonImport(module, false))
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

func extractImportFromModule(node *sitter.Node, sourceCode []byte) string {
	var module string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "import" {
			break
		}
		switch child.Type() {
		case "relative_import", "dotted_name":
			module = strings.TrimSpace(child.Content(sourceCode))
		}
	}

	return module
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
