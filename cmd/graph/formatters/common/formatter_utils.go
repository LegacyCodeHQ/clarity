package common

import (
	"path/filepath"
	"sort"
)

// GetExtensionColors takes a list of file names and returns a map containing
// file extensions and corresponding colors. Each unique extension is assigned
// a color from a predefined palette.
func GetExtensionColors(fileNames []string) map[string]string {
	// Available colors for dynamic assignment to extensions
	availableColors := []string{
		"lightblue", "lightyellow", "mistyrose", "lightcyan", "lightsalmon",
		"lightpink", "lavender", "peachpuff", "plum", "powderblue", "khaki",
		"palegreen", "palegoldenrod", "paleturquoise", "thistle",
	}

	// Extract unique extensions from file names
	uniqueExtensions := make(map[string]bool)
	for _, fileName := range fileNames {
		ext := filepath.Ext(fileName)
		if ext != "" {
			uniqueExtensions[ext] = true
		}
	}

	// Sort extensions for deterministic color assignment
	sortedExtensions := make([]string, 0, len(uniqueExtensions))
	for ext := range uniqueExtensions {
		sortedExtensions = append(sortedExtensions, ext)
	}
	sort.Strings(sortedExtensions)

	// Assign colors to extensions
	extensionColors := make(map[string]string)
	for i, ext := range sortedExtensions {
		color := availableColors[i%len(availableColors)]
		extensionColors[ext] = color
	}

	return extensionColors
}
