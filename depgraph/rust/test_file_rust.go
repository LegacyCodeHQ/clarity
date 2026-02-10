package rust

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// IsTestFile reports whether the given Rust path is a test file.
func IsTestFile(filePath string) bool {
	return IsTestFileWithContent(filePath, nil)
}

// IsTestFileWithContent reports whether the given Rust path is a test file,
// using file content when available to confirm.
func IsTestFileWithContent(filePath string, contentReader vcs.ContentReader) bool {
	fileName := filepath.Base(filePath)
	ext := filepath.Ext(fileName)
	if ext != ".rs" {
		return false
	}

	base := strings.TrimSuffix(fileName, ext)
	path := filepath.ToSlash(filePath)
	isCandidate := strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test") ||
		strings.Contains(path, "/tests/") || strings.Contains(path, "/test/")
	if !isCandidate {
		return false
	}

	if contentReader == nil {
		return true
	}

	content, err := contentReader(filePath)
	if err != nil {
		return true
	}

	return hasRustTestContent(content)
}

func hasRustTestContent(content []byte) bool {
	return bytes.Contains(content, []byte("#[test]")) ||
		bytes.Contains(content, []byte("#[cfg(test)]")) ||
		bytes.Contains(content, []byte("#[tokio::test]")) ||
		bytes.Contains(content, []byte("#[async_std::test]")) ||
		bytes.Contains(content, []byte("#[test_case]")) ||
		bytes.Contains(content, []byte("#[rstest]"))
}
