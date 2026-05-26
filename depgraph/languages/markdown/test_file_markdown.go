package markdown

// IsTestFile reports whether the given Markdown path is a test file.
// Markdown documents have no widely shared test-file convention, so every
// Markdown file is treated as production content.
func IsTestFile(_ string) bool {
	return false
}
