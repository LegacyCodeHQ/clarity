package kotlin

import (
	"regexp"
	"strings"
)

// KotlinDeclarationLocation is one top-level declaration.
type KotlinDeclarationLocation struct {
	Name    string
	Kind    string
	Package string
	Line    int
}

// KotlinImportLocation is one imported binding.
type KotlinImportLocation struct {
	Path     string
	Imported string
	Local    string
	Wildcard bool
	Line     int
}

// KotlinReferenceLocation is one lexical use of a declaration.
type KotlinReferenceLocation struct {
	Name string
	Kind string
	Line int
}

var (
	kotlinEvidenceDeclarationPattern = regexp.MustCompile(
		`^(?:(?:public|internal|protected|private|expect|actual|open|abstract|sealed|data|enum|annotation|value|inline|tailrec|suspend|operator|infix|external|const|lateinit)\s+)*` +
			`(class|interface|object|typealias|fun|val|var)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	kotlinEvidenceImportPattern = regexp.MustCompile(
		`^import\s+([A-Za-z_][A-Za-z0-9_.]*|\*)(?:\s+as\s+([A-Za-z_][A-Za-z0-9_]*))?`)
	kotlinEvidenceIdentifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// ParseKotlinDeclarationLocations returns top-level declarations with lines.
func ParseKotlinDeclarationLocations(source []byte) []KotlinDeclarationLocation {
	pkg := ExtractPackageDeclaration(source)
	result := []KotlinDeclarationLocation{}
	braceDepth := 0
	parenDepth := 0
	for index, rawLine := range strings.Split(string(source), "\n") {
		line := strings.TrimSpace(stripKotlinLineComment(rawLine))
		if braceDepth == 0 && parenDepth == 0 {
			if match := kotlinEvidenceDeclarationPattern.FindStringSubmatch(line); len(match) == 3 {
				result = append(result, KotlinDeclarationLocation{
					Name: match[2], Kind: match[1], Package: pkg, Line: index + 1,
				})
			}
		}
		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")
		parenDepth += strings.Count(line, "(")
		parenDepth -= strings.Count(line, ")")
		if braceDepth < 0 {
			braceDepth = 0
		}
		if parenDepth < 0 {
			parenDepth = 0
		}
	}
	return result
}

// ParseKotlinImportLocations returns imports and aliases with source lines.
func ParseKotlinImportLocations(source []byte) []KotlinImportLocation {
	result := []KotlinImportLocation{}
	for index, rawLine := range strings.Split(string(source), "\n") {
		line := strings.TrimSpace(stripKotlinLineComment(rawLine))
		match := kotlinEvidenceImportPattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		path := match[1]
		imported := path
		if dot := strings.LastIndex(path, "."); dot >= 0 {
			imported = path[dot+1:]
		}
		local := imported
		if match[2] != "" {
			local = match[2]
		}
		result = append(result, KotlinImportLocation{
			Path: path, Imported: imported, Local: local,
			Wildcard: imported == "*", Line: index + 1,
		})
	}
	return result
}

// ExtractKotlinReferenceLocations returns references to candidate symbols.
func ExtractKotlinReferenceLocations(source []byte) []KotlinReferenceLocation {
	result := []KotlinReferenceLocation{}
	for index, rawLine := range strings.Split(string(source), "\n") {
		line := stripKotlinLineComment(rawLine)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") ||
			strings.HasPrefix(trimmed, "package ") {
			continue
		}
		for _, location := range kotlinEvidenceIdentifierPattern.FindAllStringIndex(line, -1) {
			name := line[location[0]:location[1]]
			before := strings.TrimSpace(line[:location[0]])
			after := strings.TrimSpace(line[location[1]:])
			kind := "symbol-reference"
			if kotlinInheritanceContext(before) {
				kind = "inheritance"
			} else if strings.HasPrefix(after, ".") && startsUpperKotlinIdentifier(name) {
				kind = "companion-member"
			} else if strings.HasPrefix(after, "(") {
				kind = "call"
			} else if startsUpperKotlinIdentifier(name) {
				kind = "type-reference"
			}
			result = append(result, KotlinReferenceLocation{
				Name: name, Kind: kind, Line: index + 1,
			})
		}
	}
	return result
}

func kotlinInheritanceContext(before string) bool {
	colon := strings.LastIndex(before, ":")
	if colon < 0 {
		return false
	}
	prefix := before[:colon]
	if strings.Count(prefix, "(") > strings.Count(prefix, ")") {
		return false
	}
	return strings.Contains(prefix, "class ") ||
		strings.Contains(prefix, "interface ") ||
		strings.Contains(prefix, "object ")
}

func stripKotlinLineComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}

func startsUpperKotlinIdentifier(value string) bool {
	if value == "" {
		return false
	}
	return value[0] >= 'A' && value[0] <= 'Z'
}
