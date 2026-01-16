package parsers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/sanity/cmd/graph/formatters"
	"github.com/LegacyCodeHQ/sanity/git"
	"github.com/LegacyCodeHQ/sanity/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDependencyGraph(t *testing.T) {
	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Create main.dart
	mainContent := `
		import 'dart:io';
		import 'package:flutter/material.dart';
		import 'models/user.dart';
		import 'services/api.dart';

		void main() {}
	`
	mainPath := filepath.Join(tmpDir, "main.dart")
	err := os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create models directory and user.dart
	modelsDir := filepath.Join(tmpDir, "models")
	err = os.Mkdir(modelsDir, 0755)
	require.NoError(t, err)

	userContent := `
		import '../utils/validator.dart';

		class User {
			String name;
		}
	`
	userPath := filepath.Join(modelsDir, "user.dart")
	err = os.WriteFile(userPath, []byte(userContent), 0644)
	require.NoError(t, err)

	// Create services directory and api.dart
	servicesDir := filepath.Join(tmpDir, "services")
	err = os.Mkdir(servicesDir, 0755)
	require.NoError(t, err)

	apiContent := `
		import 'package:http/http.dart';
		import '../models/user.dart';

		class Api {}
	`
	apiPath := filepath.Join(servicesDir, "api.dart")
	err = os.WriteFile(apiPath, []byte(apiContent), 0644)
	require.NoError(t, err)

	// Create utils directory and validator.dart
	utilsDir := filepath.Join(tmpDir, "utils")
	err = os.Mkdir(utilsDir, 0755)
	require.NoError(t, err)

	validatorContent := `
		class Validator {}
	`
	validatorPath := filepath.Join(utilsDir, "validator.dart")
	err = os.WriteFile(validatorPath, []byte(validatorContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{mainPath, userPath, apiPath, validatorPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 4)

	// Check main.dart dependencies (should have 2 project imports)
	mainDeps := graph[mainPath]
	assert.Len(t, mainDeps, 2)
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, apiPath)

	// Check user.dart dependencies (should have 1 project import)
	userDeps := graph[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check api.dart dependencies (should have 1 project import)
	apiDeps := graph[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check validator.dart dependencies (should have none)
	validatorDeps := graph[validatorPath]
	assert.Empty(t, validatorDeps)
}

func TestBuildDependencyGraph_KotlinSamePackageReferences(t *testing.T) {
	tmpDir := t.TempDir()

	clientContent := `
package com.example

interface LicensingClient {
  fun activate(request: ActivateLicenseRequest): ActivateLicenseResponse
}
`
	clientPath := filepath.Join(tmpDir, "LicensingClient.kt")
	require.NoError(t, os.WriteFile(clientPath, []byte(clientContent), 0644))

	requestContent := `
package com.example

data class ActivateLicenseRequest(val token: String)
`
	requestPath := filepath.Join(tmpDir, "ActivateLicenseRequest.kt")
	require.NoError(t, os.WriteFile(requestPath, []byte(requestContent), 0644))

	responseContent := `
package com.example

data class ActivateLicenseResponse(val license: String)
`
	responsePath := filepath.Join(tmpDir, "ActivateLicenseResponse.kt")
	require.NoError(t, os.WriteFile(responsePath, []byte(responseContent), 0644))

	files := []string{clientPath, requestPath, responsePath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")
	require.NoError(t, err)

	deps := graph[clientPath]
	assert.Contains(t, deps, requestPath)
	assert.Contains(t, deps, responsePath)
	assert.Empty(t, graph[requestPath])
	assert.Empty(t, graph[responsePath])
}

func TestBuildDependencyGraph_EmptyFileList(t *testing.T) {
	graph, err := parsers.BuildDependencyGraph([]string{}, "", "")

	require.NoError(t, err)
	assert.Empty(t, graph)
}

func TestBuildDependencyGraph_NonexistentFile(t *testing.T) {
	_, err := parsers.BuildDependencyGraph([]string{"/nonexistent/file.dart"}, "", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse imports")
}

func TestBuildDependencyGraph_FiltersNonSuppliedFiles(t *testing.T) {
	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Create main.dart that imports helper.dart and utils.dart
	mainContent := `
		import 'helper.dart';
		import 'utils.dart';

		void main() {}
	`
	mainPath := filepath.Join(tmpDir, "main.dart")
	err := os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create helper.dart (we'll include this in the supplied files)
	helperContent := `
		class Helper {}
	`
	helperPath := filepath.Join(tmpDir, "helper.dart")
	err = os.WriteFile(helperPath, []byte(helperContent), 0644)
	require.NoError(t, err)

	// Create utils.dart (we'll NOT include this in the supplied files)
	utilsContent := `
		class Utils {}
	`
	utilsPath := filepath.Join(tmpDir, "utils.dart")
	err = os.WriteFile(utilsPath, []byte(utilsContent), 0644)
	require.NoError(t, err)

	// Build dependency graph with only main.dart and helper.dart
	// (utils.dart is NOT supplied, so it should be filtered out)
	files := []string{mainPath, helperPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 2)

	// Check main.dart dependencies (should only contain helper.dart, NOT utils.dart)
	mainDeps := graph[mainPath]
	assert.Len(t, mainDeps, 1, "main.dart should only have 1 dependency (helper.dart)")
	assert.Contains(t, mainDeps, helperPath)
	assert.NotContains(t, mainDeps, utilsPath, "utils.dart should be filtered out since it wasn't supplied")

	// Check helper.dart dependencies (should have none)
	helperDeps := graph[helperPath]
	assert.Empty(t, helperDeps)
}

func TestDependencyGraph_ToJSON(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart", "/project/models/user.dart"},
		"/project/utils.dart": {},
	}

	formatter, err := formatters.NewFormatter("json")
	require.NoError(t, err)
	jsonData, err := formatter.Format(graph, formatters.FormatOptions{})

	require.NoError(t, err)
	assert.Contains(t, jsonData, "/project/main.dart")
	assert.Contains(t, jsonData, "/project/utils.dart")
	assert.Contains(t, jsonData, "/project/models/user.dart")
}

func TestDependencyGraph_ToDOT(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	assert.Contains(t, dot, "digraph dependencies")
	assert.Contains(t, dot, "main.dart")
	assert.Contains(t, dot, "utils.dart")
	assert.Contains(t, dot, "->")
}

func TestDependencyGraph_ToDOT_NewFilesUseSeedlingLabel(t *testing.T) {
	graph := parsers.DependencyGraph{
		"/project/new_file.dart":       {},
		"/project/new_with_stats.dart": {},
		"/project/existing.dart":       {},
	}

	stats := map[string]git.FileStats{
		"/project/new_file.dart": {
			IsNew: true,
		},
		"/project/new_with_stats.dart": {
			IsNew:     true,
			Additions: 12,
			Deletions: 1,
		},
		"/project/existing.dart": {
			Additions: 3,
		},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{FileStats: stats})
	require.NoError(t, err)

	assert.Contains(t, dot, "\"new_file.dart\" [label=\"🪴 new_file.dart\",")
	assert.Contains(t, dot, "\"new_with_stats.dart\" [label=\"🪴 new_with_stats.dart\\n+12 -1\",")
	assert.Contains(t, dot, "\"existing.dart\" [label=\"existing.dart\\n+3\",")
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen(t *testing.T) {
	// Test Go test files
	graph := parsers.DependencyGraph{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be light green
	assert.Contains(t, dot, "main_test.go")
	assert.Contains(t, dot, "utils_test.go")
	assert.Contains(t, dot, `"main_test.go" [label="main_test.go", style=filled, fillcolor=lightgreen]`)
	assert.Contains(t, dot, `"utils_test.go" [label="utils_test.go", style=filled, fillcolor=lightgreen]`)

	// Non-test files should not be light green
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.NotContains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=lightgreen]`)
	assert.NotContains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=lightgreen]`)
}

func TestDependencyGraph_ToDOT_TestFilesAreLightGreen_Dart(t *testing.T) {
	// Test Dart test files (in test/ directory)
	graph := parsers.DependencyGraph{
		"/project/lib/main.dart":        {"/project/lib/utils.dart"},
		"/project/lib/utils.dart":       {},
		"/project/test/main_test.dart":  {"/project/lib/main.dart"},
		"/project/test/utils_test.dart": {"/project/lib/utils.dart"},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be light green
	assert.Contains(t, dot, "main_test.dart")
	assert.Contains(t, dot, "utils_test.dart")
	assert.Contains(t, dot, `"main_test.dart" [label="main_test.dart", style=filled, fillcolor=lightgreen]`)
	assert.Contains(t, dot, `"utils_test.dart" [label="utils_test.dart", style=filled, fillcolor=lightgreen]`)

	// Non-test files should not be light green
	assert.Contains(t, dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.dart" [label="utils.dart", style=filled, fillcolor=white]`)
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite(t *testing.T) {
	// Create graph with majority .go files (5 files) and minority .dart files (2 files)
	graph := parsers.DependencyGraph{
		"/project/main.go":    {"/project/utils.go"},
		"/project/utils.go":   {},
		"/project/types.go":   {},
		"/project/helpers.go": {},
		"/project/config.go":  {},
		"/project/main.dart":  {},
		"/project/utils.dart": {},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// All .go files (majority extension) should be white
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"types.go" [label="types.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"helpers.go" [label="helpers.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"config.go" [label="config.go", style=filled, fillcolor=white]`)

	// .dart files (minority extension) should have a different color (not white)
	// They should have a color from the extension color palette
	assert.Contains(t, dot, "main.dart")
	assert.Contains(t, dot, "utils.dart")
	// Verify they are not white
	assert.NotContains(t, dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`)
	assert.NotContains(t, dot, `"utils.dart" [label="utils.dart", style=filled, fillcolor=white]`)
}

func TestDependencyGraph_ToDOT_MajorityExtensionIsWhite_WithTestFiles(t *testing.T) {
	// Test that test files are light green even if they're part of majority extension
	graph := parsers.DependencyGraph{
		"/project/main.go":       {"/project/utils.go"},
		"/project/utils.go":      {},
		"/project/types.go":      {},
		"/project/main_test.go":  {"/project/main.go"},
		"/project/utils_test.go": {"/project/utils.go"},
		"/project/main.dart":     {},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files should be light green (priority over majority extension)
	assert.Contains(t, dot, `"main_test.go" [label="main_test.go", style=filled, fillcolor=lightgreen]`)
	assert.Contains(t, dot, `"utils_test.go" [label="utils_test.go", style=filled, fillcolor=lightgreen]`)

	// Non-test .go files (majority extension) should be white
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"types.go" [label="types.go", style=filled, fillcolor=white]`)

	// .dart file (minority extension) should not be white
	assert.Contains(t, dot, "main.dart")
	assert.NotContains(t, dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`)
}

func TestDependencyGraph_ToDOT_MajorityExtensionTie(t *testing.T) {
	// Test when there's a tie for majority (should pick one deterministically)
	graph := parsers.DependencyGraph{
		"/project/main.go":    {},
		"/project/utils.go":   {},
		"/project/main.dart":  {},
		"/project/utils.dart": {},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// One extension should be white (the one chosen as majority)
	// The other should have a different color
	goIsWhite := strings.Contains(dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`) &&
		strings.Contains(dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	dartIsWhite := strings.Contains(dot, `"main.dart" [label="main.dart", style=filled, fillcolor=white]`) &&
		strings.Contains(dot, `"utils.dart" [label="utils.dart", style=filled, fillcolor=white]`)

	// Exactly one extension should be white (not both)
	assert.True(t, goIsWhite != dartIsWhite, "Exactly one extension should be white, not both")
}

func TestDependencyGraph_ToDOT_SingleExtensionAllWhite(t *testing.T) {
	// When all files have the same extension, they should all be white
	graph := parsers.DependencyGraph{
		"/project/main.go":  {"/project/utils.go"},
		"/project/utils.go": {},
		"/project/types.go": {},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// All files should be white (single extension)
	assert.Contains(t, dot, `"main.go" [label="main.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.go" [label="utils.go", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"types.go" [label="types.go", style=filled, fillcolor=white]`)
}

func TestBuildDependencyGraph_IncludesNonDartFiles(t *testing.T) {
	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Create a .dart file
	dartContent := `
		import 'dart:io';
		void main() {}
	`
	dartPath := filepath.Join(tmpDir, "main.dart")
	err := os.WriteFile(dartPath, []byte(dartContent), 0644)
	require.NoError(t, err)

	// Create a non-.dart file (Go file)
	goPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(goPath, []byte("package main"), 0644)
	require.NoError(t, err)

	// Create another non-.dart file (README)
	readmePath := filepath.Join(tmpDir, "README.md")
	err = os.WriteFile(readmePath, []byte("# Test"), 0644)
	require.NoError(t, err)

	// Build dependency graph with all files
	files := []string{dartPath, goPath, readmePath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 3, "graph should include all files")

	// Verify .dart file is in the graph (may have dependencies parsed)
	assert.Contains(t, graph, dartPath)

	// Verify non-.dart files are in the graph with empty dependencies
	assert.Contains(t, graph, goPath)
	assert.Empty(t, graph[goPath], "non-dart file should have no dependencies")

	assert.Contains(t, graph, readmePath)
	assert.Empty(t, graph[readmePath], "non-dart file should have no dependencies")
}

func TestBuildDependencyGraph_GoFiles(t *testing.T) {
	// Create temporary directory with test Go files
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module testproject

go 1.25
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Create main.go
	mainContent := `package main

import (
	"fmt"
	"testproject/models"
	"testproject/services"
)

func main() {
	fmt.Println("Hello")
}
`
	mainPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create models directory and user.go
	modelsDir := filepath.Join(tmpDir, "models")
	err = os.Mkdir(modelsDir, 0755)
	require.NoError(t, err)

	userContent := `package models

import "testproject/utils"

type User struct {
	Name string
}
`
	userPath := filepath.Join(modelsDir, "user.go")
	err = os.WriteFile(userPath, []byte(userContent), 0644)
	require.NoError(t, err)

	// Create services directory and api.go
	servicesDir := filepath.Join(tmpDir, "services")
	err = os.Mkdir(servicesDir, 0755)
	require.NoError(t, err)

	apiContent := `package services

import (
	"net/http"
	"testproject/models"
)

type Api struct {}
`
	apiPath := filepath.Join(servicesDir, "api.go")
	err = os.WriteFile(apiPath, []byte(apiContent), 0644)
	require.NoError(t, err)

	// Create utils directory and validator.go
	utilsDir := filepath.Join(tmpDir, "utils")
	err = os.Mkdir(utilsDir, 0755)
	require.NoError(t, err)

	validatorContent := `package utils

type Validator struct {}
`
	validatorPath := filepath.Join(utilsDir, "validator.go")
	err = os.WriteFile(validatorPath, []byte(validatorContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	// Note: Go imports refer to packages (directories), but the graph maps
	// file to file dependencies (all files in the imported package)
	files := []string{mainPath, userPath, apiPath, validatorPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 4)

	// Check main.go dependencies (should reference user.go and api.go files)
	mainDeps := graph[mainPath]
	assert.Len(t, mainDeps, 2)
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, apiPath)

	// Check user.go dependencies (should reference validator.go file)
	userDeps := graph[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check api.go dependencies (should reference user.go file)
	apiDeps := graph[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check validator.go dependencies (should have none)
	validatorDeps := graph[validatorPath]
	assert.Empty(t, validatorDeps)
}

func TestBuildDependencyGraph_MixedDartAndGo(t *testing.T) {
	// Create temporary directory with mixed files
	tmpDir := t.TempDir()

	// Create go.mod for Go support
	goModContent := `module mixedproject

go 1.25
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Create a Dart file
	dartContent := `
		import 'dart:io';
		import 'helper.dart';

		void main() {}
	`
	dartPath := filepath.Join(tmpDir, "main.dart")
	err = os.WriteFile(dartPath, []byte(dartContent), 0644)
	require.NoError(t, err)

	helperContent := `
		class Helper {}
	`
	helperPath := filepath.Join(tmpDir, "helper.dart")
	err = os.WriteFile(helperPath, []byte(helperContent), 0644)
	require.NoError(t, err)

	// Create a Go file
	goContent := `package main

import (
	"fmt"
	"mixedproject/utils"
)

func main() {}
`
	goPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(goPath, []byte(goContent), 0644)
	require.NoError(t, err)

	// Create utils directory
	utilsDir := filepath.Join(tmpDir, "utils")
	err = os.Mkdir(utilsDir, 0755)
	require.NoError(t, err)

	utilsContent := `package utils

func Helper() {}
`
	utilsPath := filepath.Join(utilsDir, "helper.go")
	err = os.WriteFile(utilsPath, []byte(utilsContent), 0644)
	require.NoError(t, err)

	// Build dependency graph with both Dart and Go files
	files := []string{dartPath, helperPath, goPath, utilsPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 4)

	// Check Dart file dependencies
	dartDeps := graph[dartPath]
	assert.Len(t, dartDeps, 1)
	assert.Contains(t, dartDeps, helperPath)

	// Check Go file dependencies (should reference helper.go file in utils package)
	goDeps := graph[goPath]
	assert.Len(t, goDeps, 1)
	assert.Contains(t, goDeps, utilsPath)
}

func TestBuildDependencyGraph_GoSymbolLevel(t *testing.T) {
	// Create temporary directory with Go files in same package
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module symboltest

go 1.25
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Create types.go with type definitions
	typesContent := `package main

type User struct {
	Name string
}

type Product struct {
	Title string
}
`
	typesPath := filepath.Join(tmpDir, "types.go")
	err = os.WriteFile(typesPath, []byte(typesContent), 0644)
	require.NoError(t, err)

	// Create helpers.go with helper functions
	helpersContent := `package main

func FormatUser(u User) string {
	return u.Name
}
`
	helpersPath := filepath.Join(tmpDir, "helpers.go")
	err = os.WriteFile(helpersPath, []byte(helpersContent), 0644)
	require.NoError(t, err)

	// Create main.go that uses both User and FormatUser
	mainContent := `package main

import "fmt"

func main() {
	u := User{Name: "Alice"}
	fmt.Println(FormatUser(u))

	p := Product{Title: "Book"}
	fmt.Println(p.Title)
}
`
	mainPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{typesPath, helpersPath, mainPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 3)

	// Check main.go dependencies (should depend on both types.go and helpers.go)
	// because it uses User and Product from types.go, and FormatUser from helpers.go
	mainDeps := graph[mainPath]
	assert.Len(t, mainDeps, 2, "main.go should depend on both types.go and helpers.go")
	assert.Contains(t, mainDeps, typesPath, "main.go uses User and Product")
	assert.Contains(t, mainDeps, helpersPath, "main.go uses FormatUser")

	// Check helpers.go dependencies (should depend on types.go)
	// because it uses User type
	helpersDeps := graph[helpersPath]
	assert.Len(t, helpersDeps, 1, "helpers.go should depend on types.go")
	assert.Contains(t, helpersDeps, typesPath, "helpers.go uses User type")

	// Check types.go dependencies (should have none)
	typesDeps := graph[typesPath]
	assert.Empty(t, typesDeps, "types.go has no dependencies")
}

func TestGetExtensionColors_BasicFunctionality(t *testing.T) {
	fileNames := []string{
		"main.go",
		"utils.go",
		"main.dart",
		"helper.dart",
		"config.json",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should have 3 unique extensions: .go, .dart, .json
	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")

	// Each extension should have a color assigned
	assert.NotEmpty(t, colors[".go"])
	assert.NotEmpty(t, colors[".dart"])
	assert.NotEmpty(t, colors[".json"])

	// Different extensions should have different colors
	assert.NotEqual(t, colors[".go"], colors[".dart"])
	assert.NotEqual(t, colors[".go"], colors[".json"])
	assert.NotEqual(t, colors[".dart"], colors[".json"])
}

func TestGetExtensionColors_EmptyList(t *testing.T) {
	colors := formatters.GetExtensionColors([]string{})

	assert.Empty(t, colors)
}

func TestGetExtensionColors_FilesWithoutExtensions(t *testing.T) {
	fileNames := []string{
		"README",
		"LICENSE",
		"Makefile",
		"main.go", // One file with extension
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should only have .go extension
	assert.Len(t, colors, 1)
	assert.Contains(t, colors, ".go")
	assert.NotEmpty(t, colors[".go"])
}

func TestGetExtensionColors_SameExtensionMultipleFiles(t *testing.T) {
	fileNames := []string{
		"main.go",
		"utils.go",
		"types.go",
		"helpers.go",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should only have one extension (.go) with one color
	assert.Len(t, colors, 1)
	assert.Contains(t, colors, ".go")
	assert.NotEmpty(t, colors[".go"])
}

func TestGetExtensionColors_WithPaths(t *testing.T) {
	fileNames := []string{
		"/path/to/project/main.go",
		"/path/to/project/utils.go",
		"/another/path/helper.dart",
		"relative/path/config.json",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should extract extensions correctly from full paths
	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")
}

func TestGetExtensionColors_ManyExtensions(t *testing.T) {
	// Test with more extensions than available colors to ensure cycling works
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

	colors := formatters.GetExtensionColors(fileNames)

	// Should have 20 unique extensions
	assert.Len(t, colors, 20)

	// All extensions should have colors assigned
	for ext, color := range colors {
		assert.NotEmpty(t, ext, "extension should not be empty")
		assert.NotEmpty(t, color, "color should not be empty for extension %s", ext)
	}

	// Verify all extensions are present
	expectedExts := []string{".go", ".dart", ".json", ".yaml", ".xml", ".md", ".txt", ".py", ".js", ".ts", ".rs", ".cpp", ".java", ".rb", ".php", ".swift", ".kt", ".scala", ".clj", ".hs"}
	for _, ext := range expectedExts {
		assert.Contains(t, colors, ext, "extension %s should be in the map", ext)
	}
}

func TestGetExtensionColors_WithinCallConsistency(t *testing.T) {
	// Test that within a single call, the same extension always maps to the same color
	fileNames := []string{
		"file1.go",
		"file2.go",
		"file3.go",
		"helper.dart",
		"main.dart",
		"config.json",
		"settings.json",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should have 3 unique extensions
	assert.Len(t, colors, 3)

	// All .go files should map to the same color
	goColor := colors[".go"]
	assert.NotEmpty(t, goColor)

	// All .dart files should map to the same color
	dartColor := colors[".dart"]
	assert.NotEmpty(t, dartColor)

	// All .json files should map to the same color
	jsonColor := colors[".json"]
	assert.NotEmpty(t, jsonColor)

	// Different extensions should have different colors
	assert.NotEqual(t, goColor, dartColor)
	assert.NotEqual(t, goColor, jsonColor)
	assert.NotEqual(t, dartColor, jsonColor)
}

func TestGetExtensionColors_MixedWithAndWithoutExtensions(t *testing.T) {
	fileNames := []string{
		"main.go",
		"README",
		"helper.dart",
		"LICENSE",
		"config.json",
		"Makefile",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should only have extensions for files that have them
	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")
	assert.NotContains(t, colors, "")
}

func TestGetExtensionColors_ValidColorNames(t *testing.T) {
	fileNames := []string{
		"file1.go",
		"file2.dart",
		"file3.json",
		"file4.yaml",
		"file5.xml",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Valid color names from the palette
	validColors := []string{
		"lightblue", "lightyellow", "mistyrose", "lightcyan", "lightsalmon",
		"lightpink", "lavender", "peachpuff", "plum", "powderblue", "khaki",
		"palegreen", "palegoldenrod", "paleturquoise", "thistle",
	}

	// All assigned colors should be from the valid palette
	for ext, color := range colors {
		assert.Contains(t, validColors, color, "extension %s should have a valid color, got %s", ext, color)
	}
}

func TestGetExtensionColors_FilesWithMultipleDots(t *testing.T) {
	// Test files with multiple dots (e.g., file.min.js should extract .js)
	fileNames := []string{
		"app.min.js",
		"bundle.min.js",
		"styles.min.css",
		"main.go",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should extract the last extension correctly
	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".js")
	assert.Contains(t, colors, ".css")
	assert.Contains(t, colors, ".go")
	// Should not have .min.js or .min.css as separate extensions
	assert.NotContains(t, colors, ".min.js")
	assert.NotContains(t, colors, ".min.css")
}

func TestGetExtensionColors_HiddenFiles(t *testing.T) {
	// Test hidden files (files starting with dot)
	fileNames := []string{
		".gitignore",
		".env",
		".dockerignore",
		"main.go",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Hidden files without extensions should be skipped
	// .env might be considered as having no extension or empty extension
	// .gitignore and .dockerignore have no extension
	// Only .go should be present
	assert.Contains(t, colors, ".go")
	// The exact behavior depends on filepath.Ext() - it returns empty for .gitignore
	assert.NotContains(t, colors, "")
}

func TestGetExtensionColors_PathsWithDotsInDirectoryNames(t *testing.T) {
	// Test paths with dots in directory names (ToDOT uses filepath.Base first)
	fileNames := []string{
		"/path/to.dir/file.go",
		"/another/path.with.dots/helper.dart",
		"relative/path.dir/config.json",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should extract extensions correctly regardless of dots in directory names
	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")
	// Should not have directory names as extensions
	assert.NotContains(t, colors, ".dir")
}

func TestGetExtensionColors_MatchesToDOTBehavior(t *testing.T) {
	// Test that GetExtensionColors behaves the same as ToDOT would
	// ToDOT uses filepath.Ext(filepath.Base(source))
	// This test verifies that filepath.Ext() on full paths works the same way
	fileNames := []string{
		"/absolute/path/to/main.go",
		"/absolute/path/to/utils.go",
		"relative/path/helper.dart",
		"config.json",
	}

	colors := formatters.GetExtensionColors(fileNames)

	// Should extract extensions correctly (same as ToDOT would)
	assert.Len(t, colors, 3)
	assert.Contains(t, colors, ".go")
	assert.Contains(t, colors, ".dart")
	assert.Contains(t, colors, ".json")

	// Verify that filepath.Ext() works correctly on full paths
	// (should be equivalent to filepath.Ext(filepath.Base()))
	for _, fileName := range fileNames {
		ext := filepath.Ext(fileName)
		if ext != "" {
			assert.Contains(t, colors, ext, "extension %s from %s should be in colors map", ext, fileName)
		}
	}
}

func TestBuildDependencyGraph_KotlinFiles(t *testing.T) {
	// Create temporary directory with test Kotlin files
	tmpDir := t.TempDir()

	// Create MainActivity.kt
	mainContent := `package com.example.app

import kotlin.collections.List
import com.google.gson.Gson
import com.example.app.models.User
import com.example.app.services.ApiService

class MainActivity {
    fun main() {}
}`
	mainPath := filepath.Join(tmpDir, "MainActivity.kt")
	err := os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create models directory and User.kt
	modelsDir := filepath.Join(tmpDir, "models")
	err = os.Mkdir(modelsDir, 0755)
	require.NoError(t, err)

	userContent := `package com.example.app.models

import com.example.app.utils.Validator

data class User(val name: String)`
	userPath := filepath.Join(modelsDir, "User.kt")
	err = os.WriteFile(userPath, []byte(userContent), 0644)
	require.NoError(t, err)

	// Create services directory and ApiService.kt
	servicesDir := filepath.Join(tmpDir, "services")
	err = os.Mkdir(servicesDir, 0755)
	require.NoError(t, err)

	apiContent := `package com.example.app.services

import retrofit2.http.GET
import com.example.app.models.User

interface ApiService {
    fun getUsers(): List<User>
}`
	apiPath := filepath.Join(servicesDir, "ApiService.kt")
	err = os.WriteFile(apiPath, []byte(apiContent), 0644)
	require.NoError(t, err)

	// Create utils directory and Validator.kt
	utilsDir := filepath.Join(tmpDir, "utils")
	err = os.Mkdir(utilsDir, 0755)
	require.NoError(t, err)

	validatorContent := `package com.example.app.utils

object Validator {
    fun validate(input: String): Boolean = true
}`
	validatorPath := filepath.Join(utilsDir, "Validator.kt")
	err = os.WriteFile(validatorPath, []byte(validatorContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{mainPath, userPath, apiPath, validatorPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 4)

	// Check MainActivity.kt dependencies (should have 2 project imports - User and ApiService)
	// External imports like Gson should be filtered out
	// Standard library imports like kotlin.collections should be filtered out
	mainDeps := graph[mainPath]
	assert.Len(t, mainDeps, 2)
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, apiPath)

	// Check User.kt dependencies (should have 1 project import - Validator)
	userDeps := graph[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check ApiService.kt dependencies (should have 1 project import - User)
	// External import retrofit2 should be filtered out
	apiDeps := graph[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check Validator.kt dependencies (should have none)
	validatorDeps := graph[validatorPath]
	assert.Empty(t, validatorDeps)
}

func TestBuildDependencyGraph_KotlinWildcardImports(t *testing.T) {
	// Create temporary directory with test Kotlin files
	tmpDir := t.TempDir()

	// Create MainActivity.kt with wildcard import
	mainContent := `package com.example.app

import com.example.app.models.*

class MainActivity {
    fun main() {}
}`
	mainPath := filepath.Join(tmpDir, "MainActivity.kt")
	err := os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create models directory with multiple files
	modelsDir := filepath.Join(tmpDir, "models")
	err = os.Mkdir(modelsDir, 0755)
	require.NoError(t, err)

	userContent := `package com.example.app.models

data class User(val name: String)`
	userPath := filepath.Join(modelsDir, "User.kt")
	err = os.WriteFile(userPath, []byte(userContent), 0644)
	require.NoError(t, err)

	productContent := `package com.example.app.models

data class Product(val id: Int, val name: String)`
	productPath := filepath.Join(modelsDir, "Product.kt")
	err = os.WriteFile(productPath, []byte(productContent), 0644)
	require.NoError(t, err)

	orderContent := `package com.example.app.models

data class Order(val id: Int, val userId: Int)`
	orderPath := filepath.Join(modelsDir, "Order.kt")
	err = os.WriteFile(orderPath, []byte(orderContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{mainPath, userPath, productPath, orderPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 4)

	// Check MainActivity.kt dependencies (should have all 3 model files due to wildcard import)
	mainDeps := graph[mainPath]
	assert.Len(t, mainDeps, 3, "Wildcard import should include all files in the package")
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, productPath)
	assert.Contains(t, mainDeps, orderPath)

	// Check model files have no dependencies
	assert.Empty(t, graph[userPath])
	assert.Empty(t, graph[productPath])
	assert.Empty(t, graph[orderPath])
}

func TestBuildDependencyGraph_TypeScriptFiles(t *testing.T) {
	// Create temporary directory with test TypeScript files
	tmpDir := t.TempDir()

	// Create index.ts
	indexContent := `
import { User } from './models/user';
import { ApiService } from './services/api';
import fs from 'fs';
import React from 'react';

export const app = { name: 'test' };
`
	indexPath := filepath.Join(tmpDir, "index.ts")
	err := os.WriteFile(indexPath, []byte(indexContent), 0644)
	require.NoError(t, err)

	// Create models directory and user.ts
	modelsDir := filepath.Join(tmpDir, "models")
	err = os.Mkdir(modelsDir, 0755)
	require.NoError(t, err)

	userContent := `
import { validateName } from '../utils/validator';

export interface User {
  name: string;
}
`
	userPath := filepath.Join(modelsDir, "user.ts")
	err = os.WriteFile(userPath, []byte(userContent), 0644)
	require.NoError(t, err)

	// Create services directory and api.ts
	servicesDir := filepath.Join(tmpDir, "services")
	err = os.Mkdir(servicesDir, 0755)
	require.NoError(t, err)

	apiContent := `
import axios from 'axios';
import { User } from '../models/user';

export class ApiService {
  async getUsers(): Promise<User[]> {
    return [];
  }
}
`
	apiPath := filepath.Join(servicesDir, "api.ts")
	err = os.WriteFile(apiPath, []byte(apiContent), 0644)
	require.NoError(t, err)

	// Create utils directory and validator.ts
	utilsDir := filepath.Join(tmpDir, "utils")
	err = os.Mkdir(utilsDir, 0755)
	require.NoError(t, err)

	validatorContent := `
export function validateName(name: string): boolean {
  return name.length > 0;
}
`
	validatorPath := filepath.Join(utilsDir, "validator.ts")
	err = os.WriteFile(validatorPath, []byte(validatorContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{indexPath, userPath, apiPath, validatorPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 4)

	// Check index.ts dependencies (should have 2 project imports - user and api)
	// External imports like fs and React should be filtered out
	indexDeps := graph[indexPath]
	assert.Len(t, indexDeps, 2)
	assert.Contains(t, indexDeps, userPath)
	assert.Contains(t, indexDeps, apiPath)

	// Check user.ts dependencies (should have 1 project import - validator)
	userDeps := graph[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check api.ts dependencies (should have 1 project import - user)
	// External import axios should be filtered out
	apiDeps := graph[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check validator.ts dependencies (should have none)
	validatorDeps := graph[validatorPath]
	assert.Empty(t, validatorDeps)
}

func TestBuildDependencyGraph_TypeScriptWithTSX(t *testing.T) {
	// Create temporary directory with TypeScript and TSX files
	tmpDir := t.TempDir()

	// Create App.tsx
	appContent := `
import React from 'react';
import { Button } from './components/Button';
import { useUser } from './hooks/useUser';

const App: React.FC = () => {
  const user = useUser();
  return <Button onClick={() => {}}>{user.name}</Button>;
};

export default App;
`
	appPath := filepath.Join(tmpDir, "App.tsx")
	err := os.WriteFile(appPath, []byte(appContent), 0644)
	require.NoError(t, err)

	// Create components directory and Button.tsx
	componentsDir := filepath.Join(tmpDir, "components")
	err = os.Mkdir(componentsDir, 0755)
	require.NoError(t, err)

	buttonContent := `
import React from 'react';

interface ButtonProps {
  onClick: () => void;
  children: React.ReactNode;
}

export const Button: React.FC<ButtonProps> = ({ onClick, children }) => {
  return <button onClick={onClick}>{children}</button>;
};
`
	buttonPath := filepath.Join(componentsDir, "Button.tsx")
	err = os.WriteFile(buttonPath, []byte(buttonContent), 0644)
	require.NoError(t, err)

	// Create hooks directory and useUser.ts
	hooksDir := filepath.Join(tmpDir, "hooks")
	err = os.Mkdir(hooksDir, 0755)
	require.NoError(t, err)

	useUserContent := `
import { useState } from 'react';

export const useUser = () => {
  const [user] = useState({ name: 'Test User' });
  return user;
};
`
	useUserPath := filepath.Join(hooksDir, "useUser.ts")
	err = os.WriteFile(useUserPath, []byte(useUserContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{appPath, buttonPath, useUserPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 3)

	// Check App.tsx dependencies (should have Button.tsx and useUser.ts)
	appDeps := graph[appPath]
	assert.Len(t, appDeps, 2)
	assert.Contains(t, appDeps, buttonPath)
	assert.Contains(t, appDeps, useUserPath)

	// Check Button.tsx dependencies (should have none - only external React)
	buttonDeps := graph[buttonPath]
	assert.Empty(t, buttonDeps)

	// Check useUser.ts dependencies (should have none - only external React)
	useUserDeps := graph[useUserPath]
	assert.Empty(t, useUserDeps)
}

func TestBuildDependencyGraph_TypeScriptReExports(t *testing.T) {
	// Create temporary directory with TypeScript files using re-exports
	tmpDir := t.TempDir()

	// Create index.ts that re-exports from other modules
	indexContent := `
export { User } from './models/user';
export { ApiService } from './services/api';
export * from './utils';
`
	indexPath := filepath.Join(tmpDir, "index.ts")
	err := os.WriteFile(indexPath, []byte(indexContent), 0644)
	require.NoError(t, err)

	// Create models directory and user.ts
	modelsDir := filepath.Join(tmpDir, "models")
	err = os.Mkdir(modelsDir, 0755)
	require.NoError(t, err)

	userContent := `
export interface User {
  name: string;
}
`
	userPath := filepath.Join(modelsDir, "user.ts")
	err = os.WriteFile(userPath, []byte(userContent), 0644)
	require.NoError(t, err)

	// Create services directory and api.ts
	servicesDir := filepath.Join(tmpDir, "services")
	err = os.Mkdir(servicesDir, 0755)
	require.NoError(t, err)

	apiContent := `
export class ApiService {}
`
	apiPath := filepath.Join(servicesDir, "api.ts")
	err = os.WriteFile(apiPath, []byte(apiContent), 0644)
	require.NoError(t, err)

	// Create utils.ts
	utilsContent := `
export function helper() {}
`
	utilsPath := filepath.Join(tmpDir, "utils.ts")
	err = os.WriteFile(utilsPath, []byte(utilsContent), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{indexPath, userPath, apiPath, utilsPath}
	graph, err := parsers.BuildDependencyGraph(files, "", "")

	require.NoError(t, err)
	assert.Len(t, graph, 4)

	// Check index.ts dependencies (should have all 3 re-exported files)
	indexDeps := graph[indexPath]
	assert.Len(t, indexDeps, 3)
	assert.Contains(t, indexDeps, userPath)
	assert.Contains(t, indexDeps, apiPath)
	assert.Contains(t, indexDeps, utilsPath)
}

func TestDependencyGraph_ToDOT_TypeScriptTestFiles(t *testing.T) {
	// Test TypeScript test files are styled as light green
	graph := parsers.DependencyGraph{
		"/project/src/App.tsx":                    {"/project/src/utils.tsx"},
		"/project/src/utils.tsx":                  {},
		"/project/src/App.test.tsx":               {"/project/src/App.tsx"},
		"/project/src/__tests__/utils.test.tsx":   {"/project/src/utils.tsx"},
		"/project/src/components/Button.spec.tsx": {},
	}

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := formatter.Format(graph, formatters.FormatOptions{})
	require.NoError(t, err)

	// Test files with .test.tsx suffix should be light green
	assert.Contains(t, dot, `"App.test.tsx" [label="App.test.tsx", style=filled, fillcolor=lightgreen]`)

	// Test files in __tests__ directory should be light green
	assert.Contains(t, dot, `"utils.test.tsx" [label="utils.test.tsx", style=filled, fillcolor=lightgreen]`)

	// Test files with .spec.tsx suffix should be light green
	assert.Contains(t, dot, `"Button.spec.tsx" [label="Button.spec.tsx", style=filled, fillcolor=lightgreen]`)

	// Non-test files should NOT be light green (they should be white as majority extension)
	assert.Contains(t, dot, `"App.tsx" [label="App.tsx", style=filled, fillcolor=white]`)
	assert.Contains(t, dot, `"utils.tsx" [label="utils.tsx", style=filled, fillcolor=white]`)
}
