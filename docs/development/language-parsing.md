# Language Parsing

The project uses [go-tree-sitter](https://github.com/smacker/go-tree-sitter) to integrate [tree-sitter](https://tree-sitter.github.io/tree-sitter/) for parsing and extracting dependency information from source files.

## Available Languages

**go-tree-sitter** provides built-in support for parsing various programming languages, with each language binding in its own directory within the [project's source](https://github.com/smacker/go-tree-sitter).

## Unsupported Languages

### Dart

Dart support is not available natively in go-tree-sitter. To add support for Dart:

1. The [tree-sitter-dart](https://github.com/UserNobody14/tree-sitter-dart) repository was cloned and the parser was built using the [tree-sitter-cli](https://github.com/tree-sitter/tree-sitter/blob/master/crates/cli/README.md) tool.
2. The parser files (`parser.c`, `parser.h`, `scanner.c`) were copied into the `internal/tree_sitter/dart/` directory.
3. Go bindings were created inside the same directory to interface with the C-based tree-sitter parser using CGo.

### Zig

Zig support uses generated parser files from [tree-sitter-grammars/tree-sitter-zig](https://github.com/tree-sitter-grammars/tree-sitter-zig), stored under `internal/tree_sitter/zig/`.

The copied source is pinned to commit `976140ed1fc828c8fb2ff2bcfbe6853f1ae9f183`. See `internal/tree_sitter/zig/README.md` for the copied files and license details.

### Supporting Additional Languages

1. Create a new language directory under `internal/tree_sitter/` and build and copy the appropriate tree-sitter files.
2. Check the `internal/tree_sitter/dart` directory for a reference implementation of the Go bindings.

## Resources

- [go-tree-sitter](https://github.com/smacker/go-tree-sitter)
- [tree-sitter-dart](https://github.com/UserNobody14/tree-sitter-dart)
- [tree-sitter-zig](https://github.com/tree-sitter-grammars/tree-sitter-zig)
