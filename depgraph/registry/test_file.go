package registry

import (
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// IsTestFile reports whether a file path should be treated as a test file.
// Detection is delegated to language-specific implementations, optionally using file content.
func IsTestFile(filePath string, contentReader vcs.ContentReader) bool {
	ext := filepath.Ext(filepath.Base(filePath))

	provider, ok := ProviderForExtension(ext)
	if !ok {
		return false
	}

	return provider.IsTestFile(filePath, contentReader)
}
