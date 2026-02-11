package formatters

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getExtensionColors_BasicFunctionality(t *testing.T) {
	fileNames := []string{
		"main.go",
		"utils.go",
		"main.dart",
		"helper.dart",
		"config.json",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")

	assert.NotEmpty(t, colors[".go"])
	assert.NotEmpty(t, colors[".dart"])
	assert.NotEmpty(t, colors[".json"])

	assert.NotEqual(t, colors[".go"], colors[".dart"])
	assert.NotEqual(t, colors[".go"], colors[".json"])
	assert.NotEqual(t, colors[".dart"], colors[".json"])
}

func Test_getExtensionColors_EmptyList(t *testing.T) {
	colors := getExtensionColors([]string{})

	assert.Empty(t, colors)
}

func Test_getExtensionColors_FilesWithoutExtensions(t *testing.T) {
	fileNames := []string{
		"README",
		"LICENSE",
		"Makefile",
		"main.go",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 1)
	assert.Contains(t, colors, ".go")
	assert.NotEmpty(t, colors[".go"])
}

func Test_getExtensionColors_SameExtensionMultipleFiles(t *testing.T) {
	fileNames := []string{
		"main.go",
		"utils.go",
		"output_format.go",
		"helpers.go",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 1)
	assert.Contains(t, colors, ".go")
	assert.NotEmpty(t, colors[".go"])
}

func Test_getExtensionColors_WithPaths(t *testing.T) {
	fileNames := []string{
		"/path/to/project/main.go",
		"/path/to/project/utils.go",
		"/another/path/helper.dart",
		"relative/path/config.json",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")
}

func Test_getExtensionColors_ManyExtensions(t *testing.T) {
	fileNames := []string{
		"file1.go",
		"file2.dart",
		"file3.json",
		"file4.yaml",
		"file5.xml",
		"file6.md",
		"file7.txt",
		"file8.py",
		"file9.js",
		"file10.ts",
		"file11.rs",
		"file12.cpp",
		"file13.java",
		"file14.rb",
		"file15.php",
		"file16.swift",
		"file17.kt",
		"file18.scala",
		"file19.clj",
		"file20.hs",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 20)

	for ext, color := range colors {
		assert.NotEmpty(t, ext, "extension should not be empty")
		assert.NotEmpty(t, color, "color should not be empty for extension %s", ext)
	}

	expectedExts := []string{".go", ".dart", ".json", ".yaml", ".xml", ".md", ".txt", ".py", ".js", ".ts", ".rs", ".cpp", ".java", ".rb", ".php", ".swift", ".kt", ".scala", ".clj", ".hs"}
	for _, ext := range expectedExts {
		assert.Contains(t, colors, ext, "extension %s should be in the map", ext)
	}
}

func Test_getExtensionColors_WithinCallConsistency(t *testing.T) {
	fileNames := []string{
		"file1.go",
		"file2.go",
		"file3.go",
		"helper.dart",
		"main.dart",
		"config.json",
		"settings.json",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 3)

	goColor := colors[".go"]
	assert.NotEmpty(t, goColor)

	dartColor := colors[".dart"]
	assert.NotEmpty(t, dartColor)

	jsonColor := colors[".json"]
	assert.NotEmpty(t, jsonColor)

	assert.NotEqual(t, goColor, dartColor)
	assert.NotEqual(t, goColor, jsonColor)
	assert.NotEqual(t, dartColor, jsonColor)
}

func Test_getExtensionColors_MixedWithAndWithoutExtensions(t *testing.T) {
	fileNames := []string{
		"main.go",
		"README",
		"helper.dart",
		"LICENSE",
		"config.json",
		"Makefile",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")
	assert.NotContains(t, colors, "")
}

func Test_getExtensionColors_ValidColorNames(t *testing.T) {
	fileNames := []string{
		"file1.go",
		"file2.dart",
		"file3.json",
		"file4.yaml",
		"file5.xml",
	}

	colors := getExtensionColors(fileNames)

	validColors := []string{
		"lightblue", "lightyellow", "mistyrose", "lightcyan", "lightsalmon",
		"lightpink", "lavender", "peachpuff", "plum", "powderblue", "khaki",
		"palegreen", "palegoldenrod", "paleturquoise", "thistle",
	}

	for ext, color := range colors {
		assert.Contains(t, validColors, color, "extension %s should have a valid color, got %s", ext, color)
	}
}

func Test_getExtensionColors_FilesWithMultipleDots(t *testing.T) {
	fileNames := []string{
		"app.min.js",
		"bundle.min.js",
		"styles.min.css",
		"main.go",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".js")
	assert.Contains(t, colors, ".css")
	assert.Contains(t, colors, ".go")
	assert.NotContains(t, colors, ".min.js")
	assert.NotContains(t, colors, ".min.css")
}

func Test_getExtensionColors_HiddenFiles(t *testing.T) {
	fileNames := []string{
		".gitignore",
		".env",
		".dockerignore",
		"main.go",
	}

	colors := getExtensionColors(fileNames)

	assert.Contains(t, colors, ".go")
	assert.NotContains(t, colors, "")
}

func Test_getExtensionColors_PathsWithDotsInDirectoryNames(t *testing.T) {
	fileNames := []string{
		"/path/to.dir/file.go",
		"/another/path.with.dots/helper.dart",
		"relative/path.dir/config.json",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")
	assert.NotContains(t, colors, ".dir")
}

func Test_getExtensionColors_MatchesToDOTBehavior(t *testing.T) {
	fileNames := []string{
		"/absolute/path/to/main.go",
		"/absolute/path/to/utils.go",
		"relative/path/helper.dart",
		"config.json",
	}

	colors := getExtensionColors(fileNames)

	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")

	for _, fileName := range fileNames {
		ext := filepath.Ext(fileName)
		if ext != "" {
			assert.Contains(t, colors, ext, "extension %s from %s should be in colors map", ext, fileName)
		}
	}
}
