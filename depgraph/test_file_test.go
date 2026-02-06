package depgraph_test

import (
	"testing"

	"github.com/LegacyCodeHQ/sanity/depgraph"
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, depgraph.IsTestFile(tc.filePath))
		})
	}
}
