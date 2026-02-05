package depgraph

import (
	"path/filepath"

	"github.com/LegacyCodeHQ/sanity/depgraph/dart"
	"github.com/LegacyCodeHQ/sanity/depgraph/golang"
	"github.com/LegacyCodeHQ/sanity/depgraph/typescript"
)

// IsTestFile reports whether a file path should be treated as a test file.
// Detection is delegated to language-specific implementations.
func IsTestFile(filePath string) bool {
	switch filepath.Ext(filepath.Base(filePath)) {
	case ".go":
		return golang.IsTestFile(filePath)
	case ".dart":
		return dart.IsTestFile(filePath)
	case ".ts", ".tsx", ".js", ".jsx":
		return typescript.IsTestFile(filePath)
	default:
		return false
	}
}
