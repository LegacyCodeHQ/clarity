package parser

import (
	"os"
	"path/filepath"
	"testing"

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
	graph, err := BuildDependencyGraph(files)

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

func TestBuildDependencyGraph_EmptyFileList(t *testing.T) {
	graph, err := BuildDependencyGraph([]string{})

	require.NoError(t, err)
	assert.Empty(t, graph)
}

func TestBuildDependencyGraph_NonexistentFile(t *testing.T) {
	_, err := BuildDependencyGraph([]string{"/nonexistent/file.dart"})

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
	graph, err := BuildDependencyGraph(files)

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
	graph := DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart", "/project/models/user.dart"},
		"/project/utils.dart": {},
	}

	jsonData, err := graph.ToJSON()

	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "/project/main.dart")
	assert.Contains(t, string(jsonData), "/project/utils.dart")
	assert.Contains(t, string(jsonData), "/project/models/user.dart")
}

func TestDependencyGraph_ToDOT(t *testing.T) {
	graph := DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}

	dot := graph.ToDOT()

	assert.Contains(t, dot, "digraph dependencies")
	assert.Contains(t, dot, "main.dart")
	assert.Contains(t, dot, "utils.dart")
	assert.Contains(t, dot, "->")
}

func TestDependencyGraph_ToList(t *testing.T) {
	graph := DependencyGraph{
		"/project/main.dart":  {"/project/utils.dart"},
		"/project/utils.dart": {},
	}

	list := graph.ToList()

	assert.Contains(t, list, "/project/main.dart:")
	assert.Contains(t, list, "-> /project/utils.dart")
	assert.Contains(t, list, "/project/utils.dart:")
	assert.Contains(t, list, "(no project dependencies)")
}
