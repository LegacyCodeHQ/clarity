package typescript

import (
	"regexp"
	"strings"
)

// ECMAScriptDeclarationLocation is one JavaScript/TypeScript declaration.
type ECMAScriptDeclarationLocation struct {
	Name string
	Kind string
	Line int
}

// ECMAScriptImportLocation is one imported or re-exported binding.
type ECMAScriptImportLocation struct {
	Path     string
	Imported string
	Local    string
	Kind     string
	Line     int
}

// ECMAScriptReferenceLocation is one use of an imported binding.
type ECMAScriptReferenceLocation struct {
	Name string
	Path string
	Kind string
	Line int
}

var (
	ecmaDeclarationPattern = regexp.MustCompile(
		`^(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:async\s+)?` +
			`(function|class|interface|type|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	ecmaValueDeclarationPattern = regexp.MustCompile(
		`^(?:export\s+)?(?:declare\s+)?(const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	ecmaImportPattern = regexp.MustCompile(
		`(?m)^[\t ]*import[\t ]+(type[\t ]+)?(?s:(.*?))[\t ]+from[\t ]*['"]([^'"]+)['"]`)
	ecmaReExportPattern = regexp.MustCompile(
		`(?m)^[\t ]*export[\t ]+(?:type[\t ]+)?(?s:(\{.*?\}|\*))[\t ]+from[\t ]*['"]([^'"]+)['"]`)
	ecmaSideEffectImportPattern = regexp.MustCompile(
		`(?m)^[\t ]*import[\t ]*['"]([^'"]+)['"]`)
	ecmaIdentifierPattern = regexp.MustCompile(`[A-Za-z_$][A-Za-z0-9_$]*`)
)

// ParseECMAScriptDeclarationLocations returns declarations with 1-based lines.
func ParseECMAScriptDeclarationLocations(source []byte) []ECMAScriptDeclarationLocation {
	result := []ECMAScriptDeclarationLocation{}
	for index, rawLine := range strings.Split(string(source), "\n") {
		line := strings.TrimSpace(stripECMAScriptLineComment(rawLine))
		if match := ecmaDeclarationPattern.FindStringSubmatch(line); len(match) == 3 {
			result = append(result, ECMAScriptDeclarationLocation{
				Name: match[2], Kind: match[1], Line: index + 1,
			})
			continue
		}
		if match := ecmaValueDeclarationPattern.FindStringSubmatch(line); len(match) == 3 {
			result = append(result, ECMAScriptDeclarationLocation{
				Name: match[2], Kind: match[1], Line: index + 1,
			})
		}
	}
	return result
}

// ParseECMAScriptImportLocations returns imported and re-exported bindings.
func ParseECMAScriptImportLocations(source []byte) []ECMAScriptImportLocation {
	text := string(source)
	result := []ECMAScriptImportLocation{}
	covered := [][2]int{}
	for _, match := range ecmaImportPattern.FindAllStringSubmatchIndex(text, -1) {
		if len(match) != 8 {
			continue
		}
		kind := "import"
		if match[2] >= 0 {
			kind = "type-import"
		}
		line := strings.Count(text[:match[0]], "\n") + 1
		result = append(result, parseECMAScriptBindings(
			text[match[4]:match[5]], text[match[6]:match[7]], kind, line)...)
		covered = append(covered, [2]int{match[0], match[1]})
	}
	for _, match := range ecmaReExportPattern.FindAllStringSubmatchIndex(text, -1) {
		if len(match) != 6 {
			continue
		}
		line := strings.Count(text[:match[0]], "\n") + 1
		result = append(result, parseECMAScriptBindings(
			text[match[2]:match[3]], text[match[4]:match[5]], "re-export", line)...)
		covered = append(covered, [2]int{match[0], match[1]})
	}
	for _, match := range ecmaSideEffectImportPattern.FindAllStringSubmatchIndex(text, -1) {
		if len(match) != 4 || indexCovered(match[0], covered) {
			continue
		}
		result = append(result, ECMAScriptImportLocation{
			Path: text[match[2]:match[3]], Imported: "*", Kind: "import",
			Line: strings.Count(text[:match[0]], "\n") + 1,
		})
	}
	return result
}

// ExtractECMAScriptReferenceLocations returns uses of imported bindings.
func ExtractECMAScriptReferenceLocations(
	source []byte,
	imports []ECMAScriptImportLocation,
) []ECMAScriptReferenceLocation {
	result := []ECMAScriptReferenceLocation{}
	lines := strings.Split(string(source), "\n")
	for _, item := range imports {
		if item.Local == "" || item.Kind == "re-export" {
			continue
		}
		for index, rawLine := range lines {
			if index+1 == item.Line {
				continue
			}
			line := stripECMAScriptLineComment(rawLine)
			for _, location := range ecmaIdentifierPattern.FindAllStringIndex(line, -1) {
				if line[location[0]:location[1]] != item.Local {
					continue
				}
				kind := "symbol-reference"
				before := strings.TrimSpace(line[:location[0]])
				after := strings.TrimSpace(line[location[1]:])
				if strings.HasSuffix(before, "extends") || strings.HasSuffix(before, "implements") {
					kind = "inheritance"
				} else if strings.HasPrefix(after, "(") || strings.HasPrefix(after, "?.(") {
					kind = "call"
				} else if item.Kind == "type-import" ||
					strings.HasSuffix(before, ":") || strings.HasSuffix(before, "<") {
					kind = "type-reference"
				}
				result = append(result, ECMAScriptReferenceLocation{
					Name: item.Imported, Path: item.Path, Kind: kind, Line: index + 1,
				})
			}
		}
	}
	return result
}

func parseECMAScriptBindings(
	clause, path, kind string,
	line int,
) []ECMAScriptImportLocation {
	clause = strings.TrimSpace(clause)
	if clause == "*" {
		return []ECMAScriptImportLocation{{
			Path: path, Imported: "*", Kind: kind, Line: line,
		}}
	}
	result := []ECMAScriptImportLocation{}
	if braceStart := strings.Index(clause, "{"); braceStart >= 0 {
		if braceEnd := strings.LastIndex(clause, "}"); braceEnd > braceStart {
			for _, rawBinding := range strings.Split(clause[braceStart+1:braceEnd], ",") {
				fields := strings.Fields(strings.TrimSpace(rawBinding))
				if len(fields) == 0 {
					continue
				}
				imported := fields[0]
				local := imported
				if len(fields) >= 3 && fields[1] == "as" {
					local = fields[2]
				}
				result = append(result, ECMAScriptImportLocation{
					Path: path, Imported: imported, Local: local, Kind: kind, Line: line,
				})
			}
		}
	}
	if star := strings.Index(clause, "* as "); star >= 0 {
		local := strings.Fields(clause[star+len("* as "):])
		if len(local) > 0 {
			result = append(result, ECMAScriptImportLocation{
				Path: path, Imported: "*", Local: local[0], Kind: kind, Line: line,
			})
		}
	}
	prefix := clause
	if comma := strings.Index(prefix, ","); comma >= 0 {
		prefix = prefix[:comma]
	}
	prefix = strings.TrimSpace(prefix)
	if prefix != "" && !strings.HasPrefix(prefix, "{") && !strings.HasPrefix(prefix, "*") {
		result = append(result, ECMAScriptImportLocation{
			Path: path, Imported: "default", Local: prefix, Kind: kind, Line: line,
		})
	}
	return result
}

func indexCovered(index int, ranges [][2]int) bool {
	for _, item := range ranges {
		if index >= item[0] && index < item[1] {
			return true
		}
	}
	return false
}

func stripECMAScriptLineComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}
