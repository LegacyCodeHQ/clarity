package dart

import (
	"path/filepath"
	"strings"
)

// IsTestFile reports whether the given Dart file path is a test file.
func IsTestFile(filePath string) bool {
	if filepath.Ext(filepath.Base(filePath)) != ".dart" {
		return false
	}

	return strings.Contains(filepath.ToSlash(filePath), "/test/")
}
