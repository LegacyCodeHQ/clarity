package formatters

import (
	"path/filepath"
	"sort"
)

var extensionColorPalette = []string{
	"lightblue", "lightyellow", "mistyrose", "lightsalmon",
	"lightpink", "lavender", "peachpuff", "plum", "powderblue", "khaki",
	"palegoldenrod", "thistle",
}

// fileTypeKey returns the key used to group a file for color assignment.
// Files with an extension are grouped by extension (e.g. ".go"). Files without
// an extension are grouped by their basename, so `pre-commit` and `pre-push`
// become distinct types instead of collapsing into a single empty-string key.
func fileTypeKey(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		return ext
	}
	return base
}

func getExtensionColors(fileNames []string) map[string]string {
	uniqueExtensions := make(map[string]bool)
	for _, fileName := range fileNames {
		uniqueExtensions[fileTypeKey(fileName)] = true
	}

	sortedExtensions := make([]string, 0, len(uniqueExtensions))
	for ext := range uniqueExtensions {
		sortedExtensions = append(sortedExtensions, ext)
	}
	sort.Strings(sortedExtensions)

	extensionColors := make(map[string]string)
	for i, ext := range sortedExtensions {
		color := extensionColorPalette[i%len(extensionColorPalette)]
		extensionColors[ext] = color
	}

	return extensionColors
}
