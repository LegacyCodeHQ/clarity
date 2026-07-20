package rust

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"
)

var (
	rustLanguage   = rust.GetLanguage()
	rustParserPool = sync.Pool{
		New: func() any {
			parser := sitter.NewParser()
			parser.SetLanguage(rustLanguage)
			return parser
		},
	}
	rustQualifiedPathPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)+`)
)

// RustImportKind describes the type of Rust import-like declaration.
type RustImportKind int

const (
	RustImportUse RustImportKind = iota
	RustImportExternCrate
	RustImportModDecl
)

// RustImport represents a Rust import statement or module declaration.
type RustImport struct {
	Path string
	Kind RustImportKind
	// Nested is true when the statement was written inside an inner `mod`
	// block or a function body rather than at file scope. Such an import is
	// still a real dependency, but it is private to that inner scope — it can
	// never be a `pub use` re-export of the file's own module.
	Nested bool
}

// RustImports parses a Rust file and returns its imports.
func RustImports(filePath string) ([]RustImport, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return ParseRustImports(sourceCode)
}

// ParseRustImports parses Rust source code and extracts imports.
func ParseRustImports(sourceCode []byte) ([]RustImport, error) {
	var imports []RustImport

	if os.Getenv("CLARITY_RUST_IMPORTS_PARSER") != "tree" {
		imports, _ = parseRustImportsFast(sourceCode)
	} else {
		parser, _ := rustParserPool.Get().(*sitter.Parser)
		if parser == nil {
			parser = sitter.NewParser()
			parser.SetLanguage(rustLanguage)
		}
		defer rustParserPool.Put(parser)

		tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Rust code: %w", err)
		}
		defer tree.Close()

		imports = extractImports(tree.RootNode(), sourceCode)
	}

	// Also capture crate-qualified path usage in expressions/types, not only `use` declarations.
	imports = append(imports, parseRustQualifiedPathRefsFast(sourceCode)...)

	return dedupeRustImports(imports), nil
}

func parseRustImportsFast(sourceCode []byte) ([]RustImport, bool) {
	imports := make([]RustImport, 0, 8)
	var stmt []byte

	depth := 0
	// Brace depth at which the pending statement started. Statements nested
	// inside an inner `mod` block or a function body still declare real
	// coupling, so we parse them too, but relative (`self::`/`super::`) paths
	// and `mod` declarations mean something different there — see
	// parseRustImportStatementBytes.
	stmtDepth := 0
	// True once we've entered a `{` whose enclosing statement is a `use`. We
	// then have to preserve the brace group's contents in `stmt` so
	// `parseRustImportStatementBytes` can expand them — unlike other brace
	// contexts (function bodies, struct literals) which start a fresh
	// statement.
	inUseBrace := false
	inLineComment := false
	inBlockComment := 0
	inString := false
	inChar := false
	escaped := false

	for i := 0; i < len(sourceCode); i++ {
		c := sourceCode[i]
		next := byte(0)
		if i+1 < len(sourceCode) {
			next = sourceCode[i+1]
		}

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment > 0 {
			if c == '/' && next == '*' {
				inBlockComment++
				i++
				continue
			}
			if c == '*' && next == '/' {
				inBlockComment--
				i++
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		if inChar {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '\'':
				inChar = false
			}
			continue
		}

		if c == '/' && next == '/' {
			inLineComment = true
			i++
			continue
		}
		if c == '/' && next == '*' {
			inBlockComment = 1
			i++
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '\'' {
			if isRustLifetimeStart(sourceCode, i) && !looksLikeRustCharLiteralStart(sourceCode, i) {
				continue
			}
			inChar = true
			continue
		}

		switch c {
		case '{':
			if !inUseBrace {
				if isLikelyUsePrefix(stmt) {
					inUseBrace = true
				} else {
					// A block that is not a `use` group — a `mod` body, a
					// function body, a struct literal. Descend into it and
					// begin a fresh statement at the deeper level.
					stmt = stmt[:0]
					stmtDepth = depth + 1
				}
			}
			if inUseBrace {
				stmt = append(stmt, c)
			}
			depth++
			continue
		case '}':
			if depth > 0 {
				depth--
			}
			if inUseBrace {
				stmt = append(stmt, c)
			} else {
				// Leaving a block; anything unterminated inside it is not a
				// statement of the enclosing scope.
				stmt = stmt[:0]
				stmtDepth = depth
			}
			continue
		}

		if c == ';' {
			if imps, ok := parseRustImportStatementBytes(stmt, stmtDepth); ok {
				imports = append(imports, imps...)
			}
			stmt = stmt[:0]
			stmtDepth = depth
			inUseBrace = false
			continue
		}
		if len(stmt) == 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
			continue
		}
		stmt = append(stmt, c)
	}

	if inString || inChar || inBlockComment > 0 {
		return nil, false
	}

	return imports, true
}

// parseRustImportStatementBytes interprets a single statement. depth is the
// brace depth the statement was written at: 0 is file scope, anything deeper is
// inside an inner `mod` block or a function body. Nested statements still
// declare real coupling, but two shapes are scope-sensitive and so are dropped
// rather than misattributed to the file:
//
//   - `self::`/`super::` are relative to the enclosing module. Inside
//     `mod status { … }`, `super::X` is the file itself, not its parent, so
//     resolving it as a file-scope path invents an edge to the wrong module.
//   - `mod foo;` inside an inner module names a file under that module's own
//     directory, not a sibling of the current file.
func parseRustImportStatementBytes(stmt []byte, depth int) ([]RustImport, bool) {
	s := trimSpaceBytes(stmt)
	if len(s) == 0 {
		return nil, false
	}
	s = stripLeadingRustAttributesBytes(s)
	if len(s) == 0 {
		return nil, false
	}
	s = stripRustVisibilityPrefixBytes(s)

	switch {
	case bytes.HasPrefix(s, []byte("use ")):
		paths := expandRustUsePathsBytes(trimSpaceBytes(s[len("use "):]))
		if len(paths) == 0 {
			return nil, false
		}
		result := make([]RustImport, 0, len(paths))
		for _, p := range paths {
			if len(p) == 0 {
				continue
			}
			if depth > 0 && isRustModuleRelativePath(p) {
				continue
			}
			result = append(result, RustImport{Path: string(p), Kind: RustImportUse, Nested: depth > 0})
		}
		if len(result) == 0 {
			return nil, false
		}
		return result, true
	case bytes.HasPrefix(s, []byte("extern crate ")):
		name := leadingRustIdentBytes(trimSpaceBytes(s[len("extern crate "):]))
		if len(name) == 0 {
			return nil, false
		}
		return []RustImport{{Path: string(name), Kind: RustImportExternCrate, Nested: depth > 0}}, true
	case bytes.HasPrefix(s, []byte("mod ")):
		if depth > 0 {
			return nil, false
		}
		name := leadingRustIdentBytes(trimSpaceBytes(s[len("mod "):]))
		if len(name) == 0 {
			return nil, false
		}
		return []RustImport{{Path: string(name), Kind: RustImportModDecl}}, true
	default:
		return nil, false
	}
}

// isRustModuleRelativePath reports whether a use path is resolved relative to
// the module it is written in (`self::…`, `super::…`, or a bare `self`/`super`)
// rather than to the crate root or an external crate.
func isRustModuleRelativePath(path []byte) bool {
	for _, prefix := range [][]byte{[]byte("self"), []byte("super")} {
		if !bytes.HasPrefix(path, prefix) {
			continue
		}
		rest := path[len(prefix):]
		if len(rest) == 0 || bytes.HasPrefix(rest, []byte("::")) {
			return true
		}
	}
	return false
}

func trimSpaceBytes(b []byte) []byte {
	return bytes.TrimSpace(b)
}

func stripLeadingRustAttributesBytes(s []byte) []byte {
	trimmed := trimSpaceBytes(s)
	for bytes.HasPrefix(trimmed, []byte("#[")) || bytes.HasPrefix(trimmed, []byte("#![")) {
		open := bytes.IndexByte(trimmed, '[')
		if open < 0 {
			return trimmed
		}
		level := 0
		end := -1
		for i := open; i < len(trimmed); i++ {
			switch trimmed[i] {
			case '[':
				level++
			case ']':
				level--
				if level == 0 {
					end = i
					break
				}
			}
		}
		if end < 0 {
			return trimmed
		}
		trimmed = trimSpaceBytes(trimmed[end+1:])
	}
	return trimmed
}

func stripRustVisibilityPrefixBytes(s []byte) []byte {
	trimmed := trimSpaceBytes(s)
	if bytes.HasPrefix(trimmed, []byte("pub ")) {
		return trimSpaceBytes(trimmed[len("pub "):])
	}
	if bytes.HasPrefix(trimmed, []byte("pub(")) {
		if idx := bytes.IndexByte(trimmed, ')'); idx >= 0 {
			return trimSpaceBytes(trimmed[idx+1:])
		}
	}
	return trimmed
}

// expandRustUsePathsBytes returns the individual import paths represented by
// the body of a `use` statement (everything after `use `, without the
// trailing `;`). For a simple path it returns one element; for brace groups
// like `super::{Git, Submodule}` it returns one element per item, recursing
// into nested groups. `self` inside a group references the prefix itself;
// `as alias` is stripped from each leaf since the underlying path is what
// the dependency resolver cares about.
func expandRustUsePathsBytes(expr []byte) [][]byte {
	expr = trimSpaceBytes(expr)
	if len(expr) == 0 {
		return nil
	}

	braceIdx := findTopLevelOpenBrace(expr)
	if braceIdx < 0 {
		if p := normalizeRustSimpleUsePathBytes(expr); len(p) > 0 {
			return [][]byte{p}
		}
		return nil
	}

	prefix := stripTrailingColonColonBytes(trimSpaceBytes(expr[:braceIdx]))
	end := matchingCloseBraceIndex(expr, braceIdx)
	if end < 0 {
		if p := normalizeRustSimpleUsePathBytes(prefix); len(p) > 0 {
			return [][]byte{p}
		}
		return nil
	}
	inner := expr[braceIdx+1 : end]

	var result [][]byte
	for _, item := range splitTopLevelCommasBytes(inner) {
		item = trimSpaceBytes(item)
		if len(item) == 0 {
			continue
		}
		if bytes.Equal(item, []byte("self")) {
			if len(prefix) > 0 {
				result = append(result, dupBytes(prefix))
			}
			continue
		}
		for _, sub := range expandRustUsePathsBytes(item) {
			result = append(result, joinRustPathBytes(prefix, sub))
		}
	}
	return result
}

func normalizeRustSimpleUsePathBytes(expr []byte) []byte {
	path := trimSpaceBytes(expr)
	if idx := bytes.Index(path, []byte(" as ")); idx >= 0 {
		path = trimSpaceBytes(path[:idx])
	}
	path = stripTrailingColonColonBytes(path)
	path = bytes.TrimPrefix(path, []byte("::"))
	return trimSpaceBytes(path)
}

func stripTrailingColonColonBytes(p []byte) []byte {
	for bytes.HasSuffix(p, []byte("::")) {
		p = trimSpaceBytes(p[:len(p)-2])
	}
	return p
}

// findTopLevelOpenBrace returns the index of the first `{` that opens a
// brace group in `expr`. We don't care about string/comment hygiene here:
// the bytes have already been filtered by `parseRustImportsFast`.
func findTopLevelOpenBrace(expr []byte) int {
	for i := 0; i < len(expr); i++ {
		if expr[i] == '{' {
			return i
		}
	}
	return -1
}

func matchingCloseBraceIndex(expr []byte, open int) int {
	if open < 0 || open >= len(expr) || expr[open] != '{' {
		return -1
	}
	depth := 0
	for i := open; i < len(expr); i++ {
		switch expr[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitTopLevelCommasBytes(inner []byte) [][]byte {
	var items [][]byte
	depth := 0
	start := 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				items = append(items, inner[start:i])
				start = i + 1
			}
		}
	}
	items = append(items, inner[start:])
	return items
}

func joinRustPathBytes(prefix, suffix []byte) []byte {
	if len(prefix) == 0 {
		return dupBytes(suffix)
	}
	combined := make([]byte, 0, len(prefix)+2+len(suffix))
	combined = append(combined, prefix...)
	combined = append(combined, "::"...)
	combined = append(combined, suffix...)
	return combined
}

func dupBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func leadingRustIdentBytes(s []byte) []byte {
	if len(s) == 0 {
		return nil
	}
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && c >= '0' && c <= '9') {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return nil
	}
	return s[:i]
}

func isLikelyUsePrefix(stmt []byte) bool {
	return bytes.Contains(stmt, []byte("use "))
}

func parseRustQualifiedPathRefsFast(sourceCode []byte) []RustImport {
	cleaned := sanitizeRustSourceForPathMatching(sourceCode)
	matches := rustQualifiedPathPattern.FindAllIndex(cleaned, -1)
	if len(matches) == 0 {
		return nil
	}

	refs := make([]RustImport, 0, len(matches))
	for _, m := range matches {
		start, end := m[0], m[1]
		if isUsePathReference(cleaned, start) {
			continue
		}
		path := strings.TrimPrefix(string(cleaned[start:end]), "::")
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		refs = append(refs, RustImport{Path: path, Kind: RustImportUse})
	}
	return refs
}

func sanitizeRustSourceForPathMatching(sourceCode []byte) []byte {
	cleaned := make([]byte, len(sourceCode))
	copy(cleaned, sourceCode)

	inLineComment := false
	inBlockComment := 0
	inString := false
	inChar := false
	escaped := false

	for i := 0; i < len(sourceCode); i++ {
		c := sourceCode[i]
		next := byte(0)
		if i+1 < len(sourceCode) {
			next = sourceCode[i+1]
		}

		if inLineComment {
			if c == '\n' {
				inLineComment = false
				cleaned[i] = '\n'
			} else {
				cleaned[i] = ' '
			}
			continue
		}

		if inBlockComment > 0 {
			cleaned[i] = ' '
			if c == '/' && next == '*' {
				cleaned[i+1] = ' '
				inBlockComment++
				i++
				continue
			}
			if c == '*' && next == '/' {
				cleaned[i+1] = ' '
				inBlockComment--
				i++
			}
			continue
		}

		if inString {
			cleaned[i] = ' '
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		if inChar {
			cleaned[i] = ' '
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '\'':
				inChar = false
			}
			continue
		}

		if c == '/' && next == '/' {
			cleaned[i] = ' '
			cleaned[i+1] = ' '
			inLineComment = true
			i++
			continue
		}
		if c == '/' && next == '*' {
			cleaned[i] = ' '
			cleaned[i+1] = ' '
			inBlockComment = 1
			i++
			continue
		}
		if c == '"' {
			cleaned[i] = ' '
			inString = true
			continue
		}
		if c == '\'' {
			if isRustLifetimeStart(sourceCode, i) && !looksLikeRustCharLiteralStart(sourceCode, i) {
				continue
			}
			cleaned[i] = ' '
			inChar = true
			continue
		}
	}

	return cleaned
}

func isUsePathReference(cleaned []byte, start int) bool {
	i := start - 1
	for i >= 0 && isRustWhitespace(cleaned[i]) {
		i--
	}
	if i < 0 {
		return false
	}

	if cleaned[i] == ')' {
		depth := 1
		i--
		for i >= 0 && depth > 0 {
			switch cleaned[i] {
			case ')':
				depth++
			case '(':
				depth--
			}
			i--
		}
		for i >= 0 && isRustWhitespace(cleaned[i]) {
			i--
		}
	}

	if i < 0 {
		return false
	}

	startTok := i
	for startTok >= 0 && isRustIdentChar(cleaned[startTok]) {
		startTok--
	}
	token := string(cleaned[startTok+1 : i+1])
	return token == "use"
}

func isRustWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isRustIdentChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func isRustLifetimeStart(source []byte, idx int) bool {
	if idx+1 >= len(source) || !isRustIdentChar(source[idx+1]) {
		return false
	}

	prev := previousNonWhitespaceByte(source, idx)
	switch prev {
	case '&', '<', '>', ',', ':', '+', '(':
		return true
	default:
		return false
	}
}

func previousNonWhitespaceByte(source []byte, idx int) byte {
	for i := idx - 1; i >= 0; i-- {
		if !isRustWhitespace(source[i]) {
			return source[i]
		}
	}
	return 0
}

func looksLikeRustCharLiteralStart(source []byte, idx int) bool {
	if idx+2 < len(source) && source[idx+2] == '\'' {
		return true
	}
	return idx+3 < len(source) && source[idx+1] == '\\' && source[idx+3] == '\''
}

func dedupeRustImports(imports []RustImport) []RustImport {
	if len(imports) == 0 {
		return nil
	}
	seen := make(map[RustImport]bool, len(imports))
	result := make([]RustImport, 0, len(imports))
	for _, imp := range imports {
		if imp.Path == "" {
			continue
		}
		if seen[imp] {
			continue
		}
		seen[imp] = true
		result = append(result, imp)
	}
	return result
}

func extractImports(rootNode *sitter.Node, sourceCode []byte) []RustImport {
	if rootNode == nil {
		return nil
	}

	// Rust imports/declarations that affect module dependencies live at file scope.
	// Restricting to top-level declarations avoids a full-tree walk and reduces cgo traversal overhead.
	childCount := int(rootNode.NamedChildCount())
	imports := make([]RustImport, 0, childCount)
	for i := 0; i < childCount; i++ {
		n := rootNode.NamedChild(i)
		if n == nil {
			continue
		}
		switch n.Type() {
		case "use_declaration":
			if path := extractUsePath(n, sourceCode); path != "" {
				imports = append(imports, RustImport{Path: path, Kind: RustImportUse})
			}
		case "extern_crate_declaration":
			if crate := extractExternCrate(n, sourceCode); crate != "" {
				imports = append(imports, RustImport{Path: crate, Kind: RustImportExternCrate})
			}
		case "mod_item":
			if modName := extractModDecl(n, sourceCode); modName != "" {
				imports = append(imports, RustImport{Path: modName, Kind: RustImportModDecl})
			}
		}
	}
	return imports
}

func extractUsePath(node *sitter.Node, sourceCode []byte) string {
	if node == nil {
		return ""
	}

	arg := node.ChildByFieldName("argument")
	if arg == nil {
		return ""
	}

	switch arg.Type() {
	case "use_as_clause", "scoped_use_list":
		if path := arg.ChildByFieldName("path"); path != nil {
			return path.Content(sourceCode)
		}
	case "scoped_identifier", "identifier", "crate", "self", "super":
		return arg.Content(sourceCode)
	}

	return arg.Content(sourceCode)
}

func namedChildCount(node *sitter.Node) int {
	if node == nil {
		return 0
	}
	return int(node.NamedChildCount())
}

func extractExternCrate(node *sitter.Node, sourceCode []byte) string {
	if node == nil {
		return ""
	}
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		return nameNode.Content(sourceCode)
	}
	childCount := namedChildCount(node)
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "identifier" {
			return child.Content(sourceCode)
		}
	}
	return ""
}

func extractModDecl(node *sitter.Node, sourceCode []byte) string {
	if node == nil {
		return ""
	}
	if modItemHasBody(node) {
		return ""
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		return nameNode.Content(sourceCode)
	}
	childCount := namedChildCount(node)
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type() == "identifier" {
			return child.Content(sourceCode)
		}
	}
	return ""
}

func modItemHasBody(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	if body := node.ChildByFieldName("body"); body != nil {
		return true
	}
	childCount := namedChildCount(node)
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "block", "declaration_list":
			return true
		}
	}
	return false
}
