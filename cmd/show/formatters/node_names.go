package formatters

import (
	"path/filepath"
	"strings"
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
