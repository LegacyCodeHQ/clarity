package depgraph

import "path/filepath"

// IsTestFile reports whether a file path should be treated as a test file.
// Detection is delegated to language-specific implementations.
func IsTestFile(filePath string) bool {
	ext := filepath.Ext(filepath.Base(filePath))

	module, ok := moduleForExtension(ext)
	if !ok {
		return false
	}

	return module.IsTestFile(filePath)
}
