package zig_test

import (
	"context"
	"testing"

	"github.com/LegacyCodeHQ/clarity/internal/tree_sitter/zig"
	sitter "github.com/smacker/go-tree-sitter"
)

func TestCanLoadGrammar(t *testing.T) {
	language := zig.GetLanguage()
	if language == nil {
		t.Errorf("Error loading Zig grammar")
	}
}

func TestCanParseZigSource(t *testing.T) {
	parser := sitter.NewParser()
	parser.SetLanguage(zig.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, []byte("const std = @import(\"std\");\npub fn main() void {}\n"))
	if err != nil {
		t.Fatalf("ParseCtx returned error: %v", err)
	}
	defer tree.Close()

	if tree.RootNode() == nil {
		t.Fatal("expected root node")
	}
	if tree.RootNode().HasError() {
		t.Fatalf("expected sample Zig source to parse without errors: %s", tree.RootNode().String())
	}
}
