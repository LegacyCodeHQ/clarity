package html

// IsTestFile reports whether the given HTML path is a test file. HTML documents
// have no widely shared test-file convention, so every HTML file is treated as
// production content.
func IsTestFile(_ string) bool {
	return false
}
