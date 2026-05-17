package zig

import (
	"path/filepath"
	"strings"
)

func IsTestFile(filePath string) bool {
	if filepath.Ext(filepath.Base(filePath)) != ".zig" {
		return false
	}

	base := filepath.Base(filePath)
	path := filepath.ToSlash(filePath)
	return strings.HasSuffix(base, "_test.zig") || strings.Contains(path, "/test/") || strings.Contains(path, "/tests/")
}
