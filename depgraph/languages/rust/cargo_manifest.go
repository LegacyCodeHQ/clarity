package rust

import (
	"bufio"
	"strings"
)

func parseRustCrateNamesFromCargoToml(content string) map[string]bool {
	names := make(map[string]bool)
	section := ""
	packageName := ""
	libName := ""

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}

		if !strings.HasPrefix(line, "name") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")
		if value == "" {
			continue
		}
		switch section {
		case "package":
			packageName = value
		case "lib":
			libName = value
		}
	}

	if libName != "" {
		names[libName] = true
	}
	if packageName != "" {
		names[normalizeCargoCrateName(packageName)] = true
	}
	return names
}

type rustPathDependencyEntry struct {
	importNames []string
	path        string
}

func parseRustPathDependencyEntries(content string) []rustPathDependencyEntry {
	scanner := bufio.NewScanner(strings.NewReader(content))
	section := ""
	entries := []rustPathDependencyEntry{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		if !isRustDependencySection(section) {
			continue
		}

		key, value, ok := parseTomlKeyValue(line)
		if !ok {
			continue
		}
		if !strings.HasPrefix(value, "{") {
			continue
		}

		path := parseTomlInlineString(value, "path")
		if path == "" {
			continue
		}

		importNames := []string{normalizeCargoCrateName(trimQuotes(key))}
		if pkg := parseTomlInlineString(value, "package"); pkg != "" {
			importNames = append(importNames, normalizeCargoCrateName(pkg))
		}

		entries = append(entries, rustPathDependencyEntry{
			importNames: dedupeNonEmptyStrings(importNames),
			path:        path,
		})
	}

	return entries
}

func isRustDependencySection(section string) bool {
	if section == "dependencies" || section == "dev-dependencies" || section == "build-dependencies" {
		return true
	}
	return strings.HasPrefix(section, "target.") && strings.HasSuffix(section, ".dependencies")
}

func parseTomlKeyValue(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func parseTomlInlineString(value, field string) string {
	idx := strings.Index(value, field)
	if idx < 0 {
		return ""
	}
	remainder := value[idx+len(field):]
	eqIdx := strings.Index(remainder, "=")
	if eqIdx < 0 {
		return ""
	}
	remainder = strings.TrimSpace(remainder[eqIdx+1:])
	return trimQuotes(remainder)
}

func trimQuotes(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ",")
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "\"") && strings.Contains(trimmed[1:], "\"") {
		trimmed = trimmed[1:]
		if end := strings.Index(trimmed, "\""); end >= 0 {
			return trimmed[:end]
		}
	}
	return strings.Trim(trimmed, "\"")
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func normalizeCargoCrateName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}
