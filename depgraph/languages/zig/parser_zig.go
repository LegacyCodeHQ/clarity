package zig

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	tree_sitter_zig "github.com/LegacyCodeHQ/clarity/internal/tree_sitter/zig"
	sitter "github.com/smacker/go-tree-sitter"
)

type Import struct {
	Path string
}

var (
	zigLanguage   = tree_sitter_zig.GetLanguage()
	zigParserPool = sync.Pool{
		New: func() any {
			parser := sitter.NewParser()
			parser.SetLanguage(zigLanguage)
			return parser
		},
	}
)

func Imports(filePath string) ([]Import, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseImports(sourceCode)
}

func ParseImports(sourceCode []byte) ([]Import, error) {
	parser, _ := zigParserPool.Get().(*sitter.Parser)
	if parser == nil {
		parser = sitter.NewParser()
		parser.SetLanguage(zigLanguage)
	}
	defer zigParserPool.Put(parser)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Zig code: %w", err)
	}
	defer tree.Close()

	return dedupeImports(extractImports(tree.RootNode(), sourceCode)), nil
}

func extractImports(rootNode *sitter.Node, sourceCode []byte) []Import {
	var imports []Import

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Type() == "builtin_function" {
			if path, ok := zigImportNodePath(node, sourceCode); ok {
				imports = append(imports, Import{Path: path})
			}
			return
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}

	walk(rootNode)
	return imports
}

func zigImportNodePath(node *sitter.Node, sourceCode []byte) (string, bool) {
	if node == nil || node.Type() != "builtin_function" || node.NamedChildCount() < 2 {
		return "", false
	}

	builtin := node.NamedChild(0)
	if builtin == nil || builtin.Type() != "builtin_identifier" || builtin.Content(sourceCode) != "@import" {
		return "", false
	}

	args := node.NamedChild(1)
	if args == nil || args.Type() != "arguments" || args.NamedChildCount() != 1 {
		return "", false
	}

	arg := args.NamedChild(0)
	if arg == nil || arg.Type() != "string" {
		return "", false
	}

	path, err := strconv.Unquote(arg.Content(sourceCode))
	if err != nil || path == "" {
		return "", false
	}
	return path, true
}

func dedupeImports(imports []Import) []Import {
	seen := make(map[string]bool, len(imports))
	deduped := make([]Import, 0, len(imports))
	for _, imp := range imports {
		if seen[imp.Path] {
			continue
		}
		seen[imp.Path] = true
		deduped = append(deduped, imp)
	}
	return deduped
}
