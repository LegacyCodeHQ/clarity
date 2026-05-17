# tree-sitter Zig

This package contains generated parser sources from
[tree-sitter-grammars/tree-sitter-zig](https://github.com/tree-sitter-grammars/tree-sitter-zig).

- Upstream commit: `976140ed1fc828c8fb2ff2bcfbe6853f1ae9f183`
- License: MIT, copied in `LICENSE`
- Files copied from upstream:
  - `src/parser.c` -> `parser.c`
  - `src/tree_sitter/*.h` -> `tree_sitter/*.h`

The parser files are checked in so normal Go builds and tests do not need the
tree-sitter CLI or network access. This revision is generated with tree-sitter
ABI 14, which matches the current Go tree-sitter runtime dependency.
