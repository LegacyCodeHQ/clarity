package depgraph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/stretchr/testify/assert"
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
			name:     "python test prefix",
			filePath: "/project/tests/test_handlers.py",
			want:     true,
		},
		{
			name:     "python non-test file",
			filePath: "/project/app/main.py",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, depgraph.IsTestFile(tc.filePath, nil))
		})
	}
}
