package formatters

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/depgraph"
)

// BuildNodeNames returns stable, distinct display names for file paths.
// Paths that share the same base name are disambiguated by increasing path suffix depth.
func BuildNodeNames(paths []string) map[string]string {
	names := make(map[string]string, len(paths))
	groupedByBase := make(map[string][]string, len(paths))
	for _, path := range paths {
		base := filepath.Base(path)
		groupedByBase[base] = append(groupedByBase[base], path)
	}

	for base, groupedPaths := range groupedByBase {
		if len(groupedPaths) == 1 {
			names[groupedPaths[0]] = base
			continue
		}

		for depth := 2; ; depth++ {
			suffixCounts := make(map[string]int, len(groupedPaths))
			for _, path := range groupedPaths {
				suffixCounts[pathSuffix(path, depth)]++
			}

			allDistinct := true
			for _, path := range groupedPaths {
				suffix := pathSuffix(path, depth)
				if suffixCounts[suffix] > 1 {
					allDistinct = false
					break
				}
			}
			if !allDistinct {
				continue
			}

			for _, path := range groupedPaths {
				names[path] = pathSuffix(path, depth)
			}
			break
		}
	}

	return names
}

func pathSuffix(path string, depth int) string {
	normalized := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(strings.TrimPrefix(normalized, "/"), "/")
	if len(parts) == 0 {
		return normalized
	}
	if depth > len(parts) {
		depth = len(parts)
	}
	return strings.Join(parts[len(parts)-depth:], "/")
}

// nodeKey returns the stable identifier used for a node in rendered output:
// the path relative to basePath when it sits within it, otherwise the path
// unchanged. Shared by all formatters, so it is also the stable basis for
// deterministic edge labels.
func nodeKey(path, basePath string) string {
	if basePath == "" {
		return path
	}
	rel, err := filepath.Rel(basePath, path)
	if err != nil {
		return path
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return path
	}
	return rel
}

// moduleNodeLabel builds the label for a collapsed module node: the module
// name, the number of files it contains, and the aggregated churn. Lines are
// joined with sep ("\n" for DOT, "<br/>" for Mermaid).
func moduleNodeLabel(name string, md depgraph.FileMetadata, sep string) string {
	label := name
	if md.ModuleFileCount > 0 {
		unit := "files"
		if md.ModuleFileCount == 1 {
			unit = "file"
		}
		label += fmt.Sprintf("%s%d %s", sep, md.ModuleFileCount, unit)
	}
	if md.Stats != nil {
		var parts []string
		if md.Stats.Additions > 0 {
			parts = append(parts, fmt.Sprintf("+%d", md.Stats.Additions))
		}
		if md.Stats.Deletions > 0 {
			parts = append(parts, fmt.Sprintf("-%d", md.Stats.Deletions))
		}
		if len(parts) > 0 {
			label += sep + strings.Join(parts, " ")
		}
	}
	return label
}
