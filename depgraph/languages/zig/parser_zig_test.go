package zig

import (
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImports(t *testing.T) {
	source := []byte(`
const std = @import("std");
const helper = @import("helper.zig");
const nested = @import("pkg/nested.zig");
const escaped = @import("dir/with_\"quote\".zig");
// const ignored = @import("ignored.zig");
`)

	imports, err := ParseImports(source)
	require.NoError(t, err)

	assert.Equal(t, []Import{
		{Path: "std"},
		{Path: "helper.zig"},
		{Path: "pkg/nested.zig"},
		{Path: "dir/with_\"quote\".zig"},
	}, imports)
}

func TestResolveZigProjectImports(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "src", "main.zig")
	helperPath := filepath.Join(tmpDir, "src", "helper.zig")
	nestedPath := filepath.Join(tmpDir, "src", "pkg", "nested.zig")
	externalPath := filepath.Join(tmpDir, "src", "external.zig")

	suppliedFiles := map[string]bool{
		mainPath:   true,
		helperPath: true,
		nestedPath: true,
	}
	contentReader := func(path string) ([]byte, error) {
		if path == mainPath {
			return []byte(`
const std = @import("std");
const helper = @import("helper.zig");
const nested = @import("pkg/nested.zig");
const missing = @import("external.zig");
`), nil
		}
		return vcs.FilesystemContentReader()(path)
	}

	imports, err := ResolveZigProjectImports(mainPath, mainPath, ".zig", suppliedFiles, contentReader)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{helperPath, nestedPath}, imports)
	assert.NotContains(t, imports, externalPath)
}

func TestIsTestFile(t *testing.T) {
	assert.True(t, IsTestFile("/project/src/math_test.zig"))
	assert.True(t, IsTestFile("/project/test/math.zig"))
	assert.True(t, IsTestFile("/project/tests/math.zig"))
	assert.False(t, IsTestFile("/project/src/main.zig"))
	assert.False(t, IsTestFile("/project/src/math_test.go"))
}
