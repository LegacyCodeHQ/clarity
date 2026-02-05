package golang

import (
	"path/filepath"
	"strings"
)

// IsTestFile reports whether the given Go file path is a test file.
func IsTestFile(filePath string) bool {
	return strings.HasSuffix(filepath.Base(filePath), "_test.go")
}
