package zig

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
	sitter "github.com/smacker/go-tree-sitter"
)

func IsTestFile(filePath string) bool {
	if filepath.Ext(filepath.Base(filePath)) != ".zig" {
		return false
	}

	base := filepath.Base(filePath)
	path := filepath.ToSlash(filePath)
	return strings.HasSuffix(base, "_test.zig") || strings.Contains(path, "/test/") || strings.Contains(path, "/tests/")
}

func IsTestFileWithContent(filePath string, contentReader vcs.ContentReader) bool {
	if IsTestFile(filePath) {
		return true
	}
	if filepath.Base(filePath) != "test.zig" || contentReader == nil {
		return false
	}

	content, err := contentReader(filePath)
	if err != nil {
		return false
	}
	return hasZigTestDeclaration(content)
}

func hasZigTestDeclaration(sourceCode []byte) bool {
	parser := sitter.NewParser()
	parser.SetLanguage(zigLanguage)

	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return false
	}
	defer tree.Close()

	var walk func(*sitter.Node) bool
	walk = func(node *sitter.Node) bool {
		if node == nil {
			return false
		}
		if node.Type() == "test_declaration" {
			return true
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if walk(node.NamedChild(i)) {
				return true
			}
		}
		return false
	}

	return walk(tree.RootNode())
}
