package depgraph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustAdjacency(t *testing.T, g depgraph.DependencyGraph) map[string][]string {
	t.Helper()
	adj, err := depgraph.AdjacencyList(g)
	require.NoError(t, err)
	return adj
}

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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 4)

	// Check main.dart dependencies (should have 2 project imports)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 2)
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, apiPath)

	// Check user.dart dependencies (should have 1 project import)
	userDeps := adj[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check api.dart dependencies (should have 1 project import)
	apiDeps := adj[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check validator.dart dependencies (should have none)
	validatorDeps := adj[validatorPath]
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)
	adj := mustAdjacency(t, graph)

	deps := adj[clientPath]
	assert.Contains(t, deps, requestPath)
	assert.Contains(t, deps, responsePath)
	assert.Empty(t, adj[requestPath])
	assert.Empty(t, adj[responsePath])
}

func TestBuildDependencyGraph_EmptyFileList(t *testing.T) {
	graph, err := depgraph.BuildDependencyGraph([]string{}, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Empty(t, adj)
}

func TestBuildDependencyGraph_NonexistentFile(t *testing.T) {
	_, err := depgraph.BuildDependencyGraph([]string{"/nonexistent/file.dart"}, vcs.FilesystemContentReader())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 2)

	// Check main.dart dependencies (should only contain helper.dart, NOT utils.dart)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 1, "main.dart should only have 1 dependency (helper.dart)")
	assert.Contains(t, mainDeps, helperPath)
	assert.NotContains(t, mainDeps, utilsPath, "utils.dart should be filtered out since it wasn't supplied")

	// Check helper.dart dependencies (should have none)
	helperDeps := adj[helperPath]
	assert.Empty(t, helperDeps)
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 3, "graph should include all files")

	// Verify .dart file is in the graph (may have dependencies parsed)
	assert.Contains(t, adj, dartPath)

	// Verify non-.dart files are in the graph with empty dependencies
	assert.Contains(t, adj, goPath)
	assert.Empty(t, adj[goPath], "non-dart file should have no dependencies")

	assert.Contains(t, adj, readmePath)
	assert.Empty(t, adj[readmePath], "non-dart file should have no dependencies")
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 4)

	// Check main.go dependencies (should reference user.go and api.go files)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 2)
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, apiPath)

	// Check user.go dependencies (should reference validator.go file)
	userDeps := adj[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check api.go dependencies (should reference user.go file)
	apiDeps := adj[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check validator.go dependencies (should have none)
	validatorDeps := adj[validatorPath]
	assert.Empty(t, validatorDeps)
}

func TestBuildDependencyGraph_JavaFiles(t *testing.T) {
	tmpDir := t.TempDir()

	srcMain := filepath.Join(tmpDir, "src", "main", "java", "com", "example")
	srcUtil := filepath.Join(srcMain, "util")
	require.NoError(t, os.MkdirAll(srcUtil, 0o755))

	appPath := filepath.Join(srcMain, "App.java")
	appContent := `package com.example;

import com.example.util.Helper;
import java.util.List;

public class App {}
`
	require.NoError(t, os.WriteFile(appPath, []byte(appContent), 0o644))

	helperPath := filepath.Join(srcUtil, "Helper.java")
	helperContent := `package com.example.util;

public class Helper {}
`
	require.NoError(t, os.WriteFile(helperPath, []byte(helperContent), 0o644))

	files := []string{appPath, helperPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	require.Len(t, adj, 2)
	assert.Contains(t, adj[appPath], helperPath)
	assert.Empty(t, adj[helperPath])
}

func TestBuildDependencyGraph_JavaSamePackageInference(t *testing.T) {
	tmpDir := t.TempDir()

	srcMain := filepath.Join(tmpDir, "src", "main", "java", "com", "example", "model")
	require.NoError(t, os.MkdirAll(srcMain, 0o755))

	cartPath := filepath.Join(srcMain, "Cart.java")
	cartContent := `package com.example.model;

public class Cart {
    private PaymentMethod paymentMethod;
}
`
	require.NoError(t, os.WriteFile(cartPath, []byte(cartContent), 0o644))

	paymentPath := filepath.Join(srcMain, "PaymentMethod.java")
	paymentContent := `package com.example.model;

public class PaymentMethod {}
`
	require.NoError(t, os.WriteFile(paymentPath, []byte(paymentContent), 0o644))

	files := []string{cartPath, paymentPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())
	require.NoError(t, err)

	adj := mustAdjacency(t, graph)
	require.Len(t, adj, 2)
	assert.Contains(t, adj[cartPath], paymentPath)
	assert.Empty(t, adj[paymentPath])
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 4)

	// Check Dart file dependencies
	dartDeps := adj[dartPath]
	assert.Len(t, dartDeps, 1)
	assert.Contains(t, dartDeps, helperPath)

	// Check Go file dependencies (should reference helper.go file in utils package)
	goDeps := adj[goPath]
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

	// Create output_format.go with type definitions
	typesContent := `package main

type User struct {
	Name string
}

type Product struct {
	Title string
}
`
	typesPath := filepath.Join(tmpDir, "output_format.go")
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 3)

	// Check main.go dependencies (should depend on both output_format.go and helpers.go)
	// because it uses User and Product from output_format.go, and FormatUser from helpers.go
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 2, "main.go should depend on both output_format.go and helpers.go")
	assert.Contains(t, mainDeps, typesPath, "main.go uses User and Product")
	assert.Contains(t, mainDeps, helpersPath, "main.go uses FormatUser")

	// Check helpers.go dependencies (should depend on output_format.go)
	// because it uses User type
	helpersDeps := adj[helpersPath]
	assert.Len(t, helpersDeps, 1, "helpers.go should depend on output_format.go")
	assert.Contains(t, helpersDeps, typesPath, "helpers.go uses User type")

	// Check output_format.go dependencies (should have none)
	typesDeps := adj[typesPath]
	assert.Empty(t, typesDeps, "output_format.go has no dependencies")
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
    fun main() {
        val user: User? = null
        val service: ApiService? = null
        println(user)
        println(service)
    }
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

data class User(val name: String, val validator: Validator? = null)`
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 4)

	// Check MainActivity.kt dependencies (should have 2 project imports - User and ApiService)
	// External imports like Gson should be filtered out
	// Standard library imports like kotlin.collections should be filtered out
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 2)
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, apiPath)

	// Check User.kt dependencies (should have 1 project import - Validator)
	userDeps := adj[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check ApiService.kt dependencies (should have 1 project import - User)
	// External import retrofit2 should be filtered out
	apiDeps := adj[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check Validator.kt dependencies (should have none)
	validatorDeps := adj[validatorPath]
	assert.Empty(t, validatorDeps)
}

func TestBuildDependencyGraph_KotlinWildcardImports(t *testing.T) {
	// Create temporary directory with test Kotlin files
	tmpDir := t.TempDir()

	// Create MainActivity.kt with wildcard import
	mainContent := `package com.example.app

import com.example.app.models.*

class MainActivity {
    fun main() {
        val user: User? = null
        val product: Product? = null
        val order: Order? = null
        println(user)
        println(product)
        println(order)
    }
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 4)

	// Check MainActivity.kt dependencies (should have all 3 model files due to wildcard import)
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 3, "Wildcard import should include all files in the package")
	assert.Contains(t, mainDeps, userPath)
	assert.Contains(t, mainDeps, productPath)
	assert.Contains(t, mainDeps, orderPath)

	// Check model files have no dependencies
	assert.Empty(t, adj[userPath])
	assert.Empty(t, adj[productPath])
	assert.Empty(t, adj[orderPath])
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 4)

	// Check index.ts dependencies (should have 2 project imports - user and api)
	// External imports like fs and React should be filtered out
	indexDeps := adj[indexPath]
	assert.Len(t, indexDeps, 2)
	assert.Contains(t, indexDeps, userPath)
	assert.Contains(t, indexDeps, apiPath)

	// Check user.ts dependencies (should have 1 project import - validator)
	userDeps := adj[userPath]
	assert.Len(t, userDeps, 1)
	assert.Contains(t, userDeps, validatorPath)

	// Check api.ts dependencies (should have 1 project import - user)
	// External import axios should be filtered out
	apiDeps := adj[apiPath]
	assert.Len(t, apiDeps, 1)
	assert.Contains(t, apiDeps, userPath)

	// Check validator.ts dependencies (should have none)
	validatorDeps := adj[validatorPath]
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 3)

	// Check App.tsx dependencies (should have Button.tsx and useUser.ts)
	appDeps := adj[appPath]
	assert.Len(t, appDeps, 2)
	assert.Contains(t, appDeps, buttonPath)
	assert.Contains(t, appDeps, useUserPath)

	// Check Button.tsx dependencies (should have none - only external React)
	buttonDeps := adj[buttonPath]
	assert.Empty(t, buttonDeps)

	// Check useUser.ts dependencies (should have none - only external React)
	useUserDeps := adj[useUserPath]
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
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 4)

	// Check index.ts dependencies (should have all 3 re-exported files)
	indexDeps := adj[indexPath]
	assert.Len(t, indexDeps, 3)
	assert.Contains(t, indexDeps, userPath)
	assert.Contains(t, indexDeps, apiPath)
	assert.Contains(t, indexDeps, utilsPath)
}

func TestBuildDependencyGraph_GoEmbed(t *testing.T) {
	// Create temporary directory with Go files using //go:embed
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module embedtest

go 1.25
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Create cmd directory
	cmdDir := filepath.Join(tmpDir, "cmd")
	err = os.Mkdir(cmdDir, 0755)
	require.NoError(t, err)

	// Create main.go that embeds a markdown file
	mainContent := `package main

import _ "embed"

//go:embed README.md
var readme string

func main() {
	println(readme)
}
`
	mainPath := filepath.Join(cmdDir, "main.go")
	err = os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create README.md in the same directory
	readmePath := filepath.Join(cmdDir, "README.md")
	err = os.WriteFile(readmePath, []byte("# Test README"), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{mainPath, readmePath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 2)

	// Check main.go dependencies - should include README.md via embed directive
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 1, "main.go should have exactly 1 dependency (README.md via embed)")
	assert.Contains(t, mainDeps, readmePath, "main.go should depend on README.md via //go:embed")

	// Check README.md has no dependencies
	readmeDeps := adj[readmePath]
	assert.Empty(t, readmeDeps)
}

func TestBuildDependencyGraph_GoImportDoesNotIncludeNonGoFiles(t *testing.T) {
	// This test verifies that Go imports only create dependencies on .go files,
	// not on non-Go files that happen to be in the same package directory.
	// Non-Go file dependencies should only come from //go:embed directives.
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module importtest

go 1.25
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Create pkg directory with a Go file AND a markdown file
	pkgDir := filepath.Join(tmpDir, "pkg")
	err = os.Mkdir(pkgDir, 0755)
	require.NoError(t, err)

	// Create pkg/lib.go with an exported function
	libContent := `package pkg

func Helper() string {
	return "hello"
}
`
	libPath := filepath.Join(pkgDir, "lib.go")
	err = os.WriteFile(libPath, []byte(libContent), 0644)
	require.NoError(t, err)

	// Create pkg/README.md (non-Go file in the package directory)
	pkgReadmePath := filepath.Join(pkgDir, "README.md")
	err = os.WriteFile(pkgReadmePath, []byte("# Package docs"), 0644)
	require.NoError(t, err)

	// Create main.go that imports the pkg package
	mainContent := `package main

import "importtest/pkg"

func main() {
	println(pkg.Helper())
}
`
	mainPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Build dependency graph with all files including the README
	files := []string{mainPath, libPath, pkgReadmePath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 3)

	// Check main.go dependencies - should only include lib.go, NOT README.md
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 1, "main.go should only depend on lib.go, not README.md")
	assert.Contains(t, mainDeps, libPath, "main.go should depend on lib.go via import")
	assert.NotContains(t, mainDeps, pkgReadmePath, "main.go should NOT depend on README.md via import")

	// Check lib.go has no dependencies
	libDeps := adj[libPath]
	assert.Empty(t, libDeps)

	// Check README.md has no dependencies
	readmeDeps := adj[pkgReadmePath]
	assert.Empty(t, readmeDeps)
}

func TestBuildDependencyGraph_GoEmbedMultipleFiles(t *testing.T) {
	// Test that multiple embed directives create multiple dependencies
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module multiembed

go 1.25
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Create main.go with multiple embed directives
	mainContent := `package main

import _ "embed"

//go:embed config.json
var config string

//go:embed templates/index.html
var indexTemplate string

func main() {
	println(config)
	println(indexTemplate)
}
`
	mainPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create config.json
	configPath := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configPath, []byte(`{"key": "value"}`), 0644)
	require.NoError(t, err)

	// Create templates directory and index.html
	templatesDir := filepath.Join(tmpDir, "templates")
	err = os.Mkdir(templatesDir, 0755)
	require.NoError(t, err)

	indexPath := filepath.Join(templatesDir, "index.html")
	err = os.WriteFile(indexPath, []byte("<html></html>"), 0644)
	require.NoError(t, err)

	// Build dependency graph
	files := []string{mainPath, configPath, indexPath}
	graph, err := depgraph.BuildDependencyGraph(files, vcs.FilesystemContentReader())

	require.NoError(t, err)
	adj := mustAdjacency(t, graph)
	assert.Len(t, adj, 3)

	// Check main.go dependencies - should include both embedded files
	mainDeps := adj[mainPath]
	assert.Len(t, mainDeps, 2, "main.go should have 2 dependencies via embed directives")
	assert.Contains(t, mainDeps, configPath, "main.go should depend on config.json")
	assert.Contains(t, mainDeps, indexPath, "main.go should depend on index.html")
}
