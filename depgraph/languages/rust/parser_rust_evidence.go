package rust

import (
	"regexp"
	"strings"
)

// RustDeclarationLocation is one file-visible Rust declaration.
type RustDeclarationLocation struct {
	Name string
	Kind string
	Line int
}

// RustReferenceLocation is one dependency-producing Rust reference.
type RustReferenceLocation struct {
	Name string
	Kind string
	Path string
	Line int
}

var (
	rustEvidenceDeclarationPattern = regexp.MustCompile(
		`^(?:pub(?:\s*\([^)]*\))?\s+)?(?:async\s+|unsafe\s+|const\s+)*` +
			`(struct|enum|trait|type|fn|const|static|union)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	rustEvidenceModPattern = regexp.MustCompile(
		`^(?:pub(?:\s*\([^)]*\))?\s+)?mod\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	rustEvidenceUsePattern = regexp.MustCompile(
		`^(pub(?:\s*\([^)]*\))?\s+)?use\s+(.+);`)
	rustEvidenceMultilineUsePattern = regexp.MustCompile(
		`(?m)^[\t ]*(pub(?:[\t ]*\([^)]*\))?[\t ]+)?use[\t ]+([^;]+);`)
	rustEvidenceIdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// ParseRustDeclarationLocations returns file-visible declarations with their
// 1-based source lines.
func ParseRustDeclarationLocations(source []byte) []RustDeclarationLocation {
	lines := strings.Split(string(source), "\n")
	result := []RustDeclarationLocation{}
	depth := 0
	for lineIndex, rawLine := range lines {
		line := strings.TrimSpace(stripRustLineComment(rawLine))
		if depth == 0 {
			match := rustEvidenceDeclarationPattern.FindStringSubmatch(line)
			if len(match) == 3 {
				result = append(result, RustDeclarationLocation{
					Name: match[2],
					Kind: match[1],
					Line: lineIndex + 1,
				})
			}
		}
		depth += strings.Count(line, "{")
		depth -= strings.Count(line, "}")
		if depth < 0 {
			depth = 0
		}
	}
	return result
}

// ExtractRustReferenceLocations returns explicit module declarations, use
// paths, and lexical symbol references with their 1-based source lines.
func ExtractRustReferenceLocations(source []byte) []RustReferenceLocation {
	text := string(source)
	lines := strings.Split(text, "\n")
	result := []RustReferenceLocation{}
	useLines := make(map[int]bool)
	for _, match := range rustEvidenceMultilineUsePattern.FindAllStringSubmatchIndex(text, -1) {
		if len(match) != 6 {
			continue
		}
		kind := "import"
		if match[2] >= 0 {
			kind = "re-export"
		}
		line := strings.Count(text[:match[0]], "\n") + 1
		endLine := strings.Count(text[:match[1]], "\n") + 1
		for current := line; current <= endLine; current++ {
			useLines[current] = true
		}
		for _, name := range rustEvidenceIdentifierPattern.FindAllString(text[match[4]:match[5]], -1) {
			if name == "as" || name == "self" || name == "super" || name == "crate" {
				continue
			}
			result = append(result, RustReferenceLocation{
				Name: name,
				Kind: kind,
				Line: line,
			})
		}
	}
	for lineIndex, rawLine := range lines {
		if useLines[lineIndex+1] {
			continue
		}
		line := strings.TrimSpace(stripRustLineComment(rawLine))
		if line == "" {
			continue
		}
		if match := rustEvidenceModPattern.FindStringSubmatch(line); len(match) == 2 {
			result = append(result, RustReferenceLocation{
				Name: match[1],
				Kind: "module-declaration",
				Line: lineIndex + 1,
			})
			continue
		}
		if match := rustEvidenceUsePattern.FindStringSubmatch(line); len(match) == 3 {
			continue
		}
		for _, path := range rustQualifiedPathPattern.FindAllString(line, -1) {
			parts := strings.Split(path, "::")
			name := parts[len(parts)-1]
			kind := "symbol-reference"
			if startsUpperRustIdentifier(name) {
				kind = "type-reference"
			} else if matchOffset := strings.Index(line, path); matchOffset >= 0 {
				rest := strings.TrimSpace(line[matchOffset+len(path):])
				if strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "::<") {
					kind = "call"
				}
			}
			result = append(result, RustReferenceLocation{
				Name: name,
				Kind: kind,
				Path: path,
				Line: lineIndex + 1,
			})
		}
		for _, location := range rustEvidenceIdentifierPattern.FindAllStringIndex(line, -1) {
			name := line[location[0]:location[1]]
			kind := "symbol-reference"
			rest := strings.TrimSpace(line[location[1]:])
			if strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "::<") {
				kind = "call"
			} else if startsUpperRustIdentifier(name) {
				kind = "type-reference"
			}
			result = append(result, RustReferenceLocation{
				Name: name,
				Kind: kind,
				Line: lineIndex + 1,
			})
		}
	}
	return result
}

func stripRustLineComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}

func startsUpperRustIdentifier(value string) bool {
	if value == "" {
		return false
	}
	first := value[0]
	return first >= 'A' && first <= 'Z'
}
