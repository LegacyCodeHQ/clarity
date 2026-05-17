package zig

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	zigImportCallPattern = regexp.MustCompile(`^@import\s*\(\s*("(?:\\.|[^"\\])*")\s*\)$`)
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
			if imp, ok := parseImportCall(node.Content(sourceCode)); ok {
				imports = append(imports, imp)
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

func parseImportCall(content string) (Import, bool) {
	matches := zigImportCallPattern.FindStringSubmatch(strings.TrimSpace(content))
	if len(matches) != 2 {
		return Import{}, false
	}

	path, err := strconv.Unquote(matches[1])
	if err != nil || path == "" {
		return Import{}, false
	}

	return Import{Path: path}, true
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
