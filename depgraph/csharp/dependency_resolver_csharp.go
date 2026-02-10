package csharp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

func BuildCSharpIndices(
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) (map[string][]string, map[string]map[string][]string, map[string]string, map[string]string) {
	namespaceToFiles := make(map[string][]string)
	namespaceToTypes := make(map[string]map[string][]string)
	fileToNamespace := make(map[string]string)
	fileToScope := make(map[string]string)

	for filePath := range suppliedFiles {
		if filepath.Ext(filePath) != ".cs" {
			continue
		}

		content, err := contentReader(filePath)
		if err != nil {
			continue
		}

		source := string(content)
		namespace := ParseCSharpNamespace(source)
		fileToNamespace[filePath] = namespace
		scope := inferCSharpFileScope(filePath)
		fileToScope[filePath] = scope
		scopedNamespace := scopeKey(scope, namespace)
		namespaceToFiles[scopedNamespace] = append(namespaceToFiles[scopedNamespace], filePath)

		typeNames := ParseTopLevelCSharpTypeNames(source)
		if len(typeNames) == 0 {
			continue
		}

		typeMap, ok := namespaceToTypes[scopedNamespace]
		if !ok {
			typeMap = make(map[string][]string)
			namespaceToTypes[scopedNamespace] = typeMap
		}
		for _, typeName := range typeNames {
			typeMap[typeName] = append(typeMap[typeName], filePath)
		}
	}

	return namespaceToFiles, namespaceToTypes, fileToNamespace, fileToScope
}

func ResolveCSharpProjectImports(
	absPath string,
	_ string,
	namespaceToFiles map[string][]string,
	namespaceToTypes map[string]map[string][]string,
	fileToNamespace map[string]string,
	fileToScope map[string]string,
	suppliedFiles map[string]bool,
	contentReader vcs.ContentReader,
) ([]string, error) {
	content, err := contentReader(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absPath, err)
	}

	source := string(content)
	imports := ParseCSharpImports(source)
	referencedTypes := ExtractCSharpTypeIdentifiers(source)
	declaredTypes := make(map[string]bool)
	for _, name := range ParseTopLevelCSharpTypeNames(source) {
		declaredTypes[name] = true
	}

	resolved := make([]string, 0, len(imports))
	seen := make(map[string]bool)
	addDep := func(path string) {
		if path == absPath || !suppliedFiles[path] || seen[path] {
			return
		}
		seen[path] = true
		resolved = append(resolved, path)
	}

	importedTypeNames := make(map[string]bool)
	resolvedTypes := make(map[string]bool)
	scope := fileToScope[absPath]
	for _, imp := range imports {
		path := imp.Path
		if path == "" {
			continue
		}

		// "using A.B;" form imports a namespace.
		if typeMap, ok := namespaceToTypes[scopeKey(scope, path)]; ok {
			for _, ref := range referencedTypes {
				if declaredTypes[ref] {
					continue
				}
				files := typeMap[ref]
				if len(files) != 1 {
					continue
				}
				addDep(files[0])
				resolvedTypes[ref] = true
			}
			continue
		}

		// "using A.B.TypeName;" can import a specific type.
		lastDot := strings.LastIndex(path, ".")
		if lastDot <= 0 || lastDot >= len(path)-1 {
			continue
		}
		pkg := path[:lastDot]
		typeName := path[lastDot+1:]
		importedTypeNames[typeName] = true
		if !containsString(referencedTypes, typeName) {
			continue
		}
		typeMap := namespaceToTypes[scopeKey(scope, pkg)]
		files := typeMap[typeName]
		if len(files) != 1 {
			continue
		}
		addDep(files[0])
		resolvedTypes[typeName] = true
	}

	// Same-namespace references do not require using directives in C#.
	if namespace, ok := fileToNamespace[absPath]; ok {
		if typeMap, ok := namespaceToTypes[scopeKey(scope, namespace)]; ok {
			for _, ref := range referencedTypes {
				if declaredTypes[ref] || importedTypeNames[ref] {
					continue
				}
				files := typeMap[ref]
				if len(files) != 1 {
					continue
				}
				addDep(files[0])
				resolvedTypes[ref] = true
			}
		}
	}

	// Cross-scope fallback: if a referenced type resolves to exactly one changed file
	// across all scopes, link it. This captures project-reference/global-using flows
	// while still avoiding fan-out from duplicate type names (e.g. start vs finished).
	globalTypeMatches := make(map[string][]string)
	for _, typeMap := range namespaceToTypes {
		for typeName, files := range typeMap {
			globalTypeMatches[typeName] = append(globalTypeMatches[typeName], files...)
		}
	}
	for _, ref := range referencedTypes {
		if declaredTypes[ref] || importedTypeNames[ref] || resolvedTypes[ref] {
			continue
		}
		files := uniqueStrings(globalTypeMatches[ref])
		if len(files) != 1 {
			continue
		}
		addDep(files[0])
	}

	_ = namespaceToFiles // retained for future namespace-wide heuristics.
	return resolved, nil
}

func inferCSharpFileScope(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if strings.HasSuffix(entry.Name(), ".csproj") {
					return filepath.Join(dir, entry.Name())
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(filePath)
}

func scopeKey(scope, namespace string) string {
	return scope + "::" + namespace
}

func uniqueStrings(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
