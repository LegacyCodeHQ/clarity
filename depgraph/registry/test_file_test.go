package registry_test

import (
	"fmt"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "go test file",
			filePath: "/project/main_test.go",
			want:     true,
		},
		{
			name:     "go non-test file",
			filePath: "/project/main.go",
			want:     false,
		},
		{
			name:     "dart file under test directory",
			filePath: "/project/test/widget_test.dart",
			want:     true,
		},
		{
			name:     "dart file outside test directory",
			filePath: "/project/lib/main.dart",
			want:     false,
		},
		{
			name:     "typescript .test suffix",
			filePath: "/project/src/App.test.tsx",
			want:     true,
		},
		{
			name:     "typescript .spec suffix",
			filePath: "/project/src/components/Button.spec.ts",
			want:     true,
		},
		{
			name:     "javascript __tests__ directory",
			filePath: "/project/src/__tests__/helper.js",
			want:     true,
		},
		{
			name:     "typescript non-test file",
			filePath: "/project/src/index.ts",
			want:     false,
		},
		{
			name:     "kotlin test file suffix",
			filePath: "/project/src/Test.kt",
			want:     true,
		},
		{
			name:     "kotlin non-test file",
			filePath: "/project/src/Main.kt",
			want:     false,
		},
		{
			name:     "java test file suffix",
			filePath: "/project/src/test/java/com/example/AppTest.java",
			want:     true,
		},
		{
			name:     "java non-test file",
			filePath: "/project/src/main/java/com/example/App.java",
			want:     false,
		},
		{
			name:     "c test prefix",
			filePath: "/project/tests/test_math.c",
			want:     true,
		},
		{
			name:     "c non-test file",
			filePath: "/project/src/main.c",
			want:     false,
		},
		{
			name:     "cpp test suffix",
			filePath: "/project/tests/math_test.cpp",
			want:     true,
		},
		{
			name:     "cpp non-test file",
			filePath: "/project/src/main.cpp",
			want:     false,
		},
		{
			name:     "csharp tests suffix",
			filePath: "/project/tests/HandlersTests.cs",
			want:     true,
		},
		{
			name:     "csharp non-test file",
			filePath: "/project/src/Program.cs",
			want:     false,
		},
		{
			name:     "swift tests directory",
			filePath: "/project/Tests/AppTests.swift",
			want:     true,
		},
		{
			name:     "swift non-test file",
			filePath: "/project/Sources/App.swift",
			want:     false,
		},
		{
			name:     "rust tests directory",
			filePath: "/project/tests/lib_test.rs",
			want:     true,
		},
		{
			name:     "rust non-test file",
			filePath: "/project/src/lib.rs",
			want:     false,
		},
		{
			name:     "scala test file suffix",
			filePath: "/project/src/test/scala/com/example/AppTest.scala",
			want:     true,
		},
		{
			name:     "scala non-test file",
			filePath: "/project/src/main/scala/com/example/App.scala",
			want:     false,
		},
		{
			name:     "svelte test file",
			filePath: "/project/src/App.test.svelte",
			want:     true,
		},
		{
			name:     "svelte spec file",
			filePath: "/project/src/Button.spec.svelte",
			want:     true,
		},
		{
			name:     "svelte non-test file",
			filePath: "/project/src/App.svelte",
			want:     false,
		},
		{
			name:     "python test prefix",
			filePath: "/project/tests/test_handlers.py",
			want:     true,
		},
		{
			name:     "python non-test file",
			filePath: "/project/app/main.py",
			want:     false,
		},
		{
			name:     "zig test suffix",
			filePath: "/project/src/math_test.zig",
			want:     true,
		},
		{
			name:     "zig non-test file",
			filePath: "/project/src/main.zig",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, registry.IsTestFile(tc.filePath, nil))
		})
	}
}

func TestIsTestFile_ZigUsesTestDeclarations(t *testing.T) {
	testPath := "/project/lib/std/Io/Threaded/test.zig"
	productionPath := "/project/lib/std/Io/Threaded.zig"
	content := map[string][]byte{
		testPath: []byte(`
const std = @import("std");

test "async context alignment" {
    try std.testing.expect(true);
}
`),
		productionPath: []byte(`
const std = @import("../std.zig");

pub fn run() void {}

test {
    _ = @import("Threaded/test.zig");
}
`),
	}
	contentReader := func(path string) ([]byte, error) {
		if source, ok := content[path]; ok {
			return source, nil
		}
		return nil, fmt.Errorf("missing test content for %s", path)
	}

	assert.True(t, registry.IsTestFile(testPath, contentReader))
	assert.False(t, registry.IsTestFile(productionPath, contentReader))
	require.False(t, registry.IsTestFile(testPath, nil), "test.zig basename alone is not enough to classify a Zig test file")
}
