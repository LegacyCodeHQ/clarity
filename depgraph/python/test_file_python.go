package python

import (
	"path/filepath"
	"strings"
)

// IsTestFile reports whether the given Python path is a test file.
func IsTestFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	ext := filepath.Ext(fileName)
	if ext != ".py" {
		return false
	}

	if strings.HasPrefix(fileName, "test_") || strings.HasSuffix(fileName, "_test.py") {
		return true
	}

	path := filepath.ToSlash(filePath)
	return strings.Contains(path, "/tests/") || strings.Contains(path, "/test/")
}
