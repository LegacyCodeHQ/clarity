package explain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LegacyCodeHQ/clarity/depgraph"
	"github.com/LegacyCodeHQ/clarity/depgraph/explain"
	"github.com/LegacyCodeHQ/clarity/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachEvidence_SwiftTypeReference(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "Sources", "App")
	require.NoError(t, os.MkdirAll(module, 0o755))
	a := filepath.Join(module, "A.swift")
	b := filepath.Join(module, "B.swift")
	require.NoError(t, os.WriteFile(a, []byte(`
struct A {
    let b: B
}
`), 0o644))
	require.NoError(t, os.WriteFile(b, []byte(`
struct B {
    let a: A
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph, err := depgraph.BuildDependencyGraph([]string{a, b}, reader)
	require.NoError(t, err)
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	aToB := fileGraph.Meta.Edges[depgraph.FileEdge{From: a, To: b}].Evidence
	require.NotEmpty(t, aToB)
	assert.Equal(t, "B", aToB[0].Symbol)
	assert.Equal(t, "swift-symbol", aToB[0].Kind)
	assert.Equal(t, depgraph.RelationshipTypeReference, aToB[0].Relationship)
	assert.Equal(t, 3, aToB[0].ReferenceLine)
	assert.Equal(t, 2, aToB[0].DeclarationLine)
	assert.Equal(t, depgraph.EvidenceConfidenceHigh, aToB[0].Confidence)
}

func TestAttachEvidence_MarkdownLink(t *testing.T) {
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	guide := filepath.Join(dir, "guide.md")
	require.NoError(t, os.WriteFile(readme, []byte("[guide](guide.md)\n"), 0o644))
	require.NoError(t, os.WriteFile(guide, []byte("[readme](README.md)\n"), 0o644))

	reader := vcs.FilesystemContentReader()
	graph, err := depgraph.BuildDependencyGraph([]string{readme, guide}, reader)
	require.NoError(t, err)
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: readme, To: guide}].Evidence
	require.NotEmpty(t, evidence)
	assert.Equal(t, "guide.md", evidence[0].Symbol)
	assert.Equal(t, "markdown-link", evidence[0].Kind)
	assert.Equal(t, depgraph.RelationshipNavigation, evidence[0].Relationship)
	assert.Equal(t, 1, evidence[0].ReferenceLine)
	assert.Equal(t, depgraph.EvidenceConfidenceHigh, evidence[0].Confidence)
}

func TestAttachEvidence_RustModuleAndTypeReferences(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "lib.rs")
	modelFile := filepath.Join(dir, "model.rs")
	require.NoError(t, os.WriteFile(mainFile, []byte(`mod model;
use crate::model::Job;

pub fn run(job: Job) {}
`), 0o644))
	require.NoError(t, os.WriteFile(modelFile, []byte(`pub struct Job;
use crate::run;
`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph := depgraph.MustDependencyGraph(map[string][]string{
		mainFile:  {modelFile},
		modelFile: {mainFile},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: mainFile, To: modelFile}].Evidence
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "model",
		Kind:            "rust-module-declaration",
		Relationship:    depgraph.RelationshipModuleDeclaration,
		ReferenceFile:   mainFile,
		ReferenceLine:   1,
		DeclarationFile: modelFile,
		DeclarationLine: 1,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "Job",
		Kind:            "rust-import",
		Relationship:    depgraph.RelationshipImport,
		ReferenceFile:   mainFile,
		ReferenceLine:   2,
		DeclarationFile: modelFile,
		DeclarationLine: 1,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
}

func TestAttachEvidence_GoSamePackageAndImportedReferences(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "main.go")
	helperFile := filepath.Join(dir, "helper.go")
	require.NoError(t, os.WriteFile(mainFile, []byte(`package app

func run() {
    helper()
}
`), 0o644))
	require.NoError(t, os.WriteFile(helperFile, []byte(`package app

func helper() {
    run()
}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph := depgraph.MustDependencyGraph(map[string][]string{
		mainFile:   {helperFile},
		helperFile: {mainFile},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: mainFile, To: helperFile}].Evidence
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "helper",
		Kind:            "go-same-package-call",
		Relationship:    depgraph.RelationshipCall,
		ReferenceFile:   mainFile,
		ReferenceLine:   4,
		DeclarationFile: helperFile,
		DeclarationLine: 3,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
}

func TestAttachEvidence_GoImportedPackageReference(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "app", "main.go")
	modelFile := filepath.Join(dir, "model", "model.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(mainFile), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(modelFile), 0o755))
	require.NoError(t, os.WriteFile(mainFile, []byte(`package app

import "example.com/project/model"

func run() {
    model.Execute()
}
`), 0o644))
	require.NoError(t, os.WriteFile(modelFile, []byte(`package model

func Execute() {}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph := depgraph.MustDependencyGraph(map[string][]string{
		mainFile:  {modelFile},
		modelFile: {mainFile},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: mainFile, To: modelFile}].Evidence
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "Execute",
		Kind:            "go-imported-call",
		Relationship:    depgraph.RelationshipCall,
		ReferenceFile:   mainFile,
		ReferenceLine:   6,
		DeclarationFile: modelFile,
		DeclarationLine: 3,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
}

func TestAttachEvidence_TypeScriptTypeImportAndCall(t *testing.T) {
	dir := t.TempDir()
	appFile := filepath.Join(dir, "app.ts")
	modelFile := filepath.Join(dir, "model.ts")
	require.NoError(t, os.WriteFile(appFile, []byte(`import type { Job } from "./model";
import { execute } from "./model";

export function run(job: Job) {
  return execute(job);
}
`), 0o644))
	require.NoError(t, os.WriteFile(modelFile, []byte(`export interface Job {}
export function execute(job: Job) {}
`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph := depgraph.MustDependencyGraph(map[string][]string{
		appFile:   {modelFile},
		modelFile: {appFile},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: appFile, To: modelFile}].Evidence
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "Job",
		Kind:            "typescript-type-import",
		Relationship:    depgraph.RelationshipTypeImport,
		ReferenceFile:   appFile,
		ReferenceLine:   1,
		DeclarationFile: modelFile,
		DeclarationLine: 1,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "execute",
		Kind:            "typescript-call",
		Relationship:    depgraph.RelationshipCall,
		ReferenceFile:   appFile,
		ReferenceLine:   5,
		DeclarationFile: modelFile,
		DeclarationLine: 2,
		Confidence:      depgraph.EvidenceConfidenceMedium,
	})
}

func TestAttachEvidence_JavaScriptReExport(t *testing.T) {
	dir := t.TempDir()
	indexFile := filepath.Join(dir, "index.mjs")
	modelFile := filepath.Join(dir, "model.mjs")
	require.NoError(t, os.WriteFile(indexFile, []byte(
		`export { execute } from "./model.mjs";`), 0o644))
	require.NoError(t, os.WriteFile(modelFile, []byte(
		`export function execute() {}`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph := depgraph.MustDependencyGraph(map[string][]string{
		indexFile: {modelFile},
		modelFile: {indexFile},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: indexFile, To: modelFile}].Evidence
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "execute",
		Kind:            "javascript-re-export",
		Relationship:    depgraph.RelationshipReExport,
		ReferenceFile:   indexFile,
		ReferenceLine:   1,
		DeclarationFile: modelFile,
		DeclarationLine: 1,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
}

func TestAttachEvidence_KotlinSamePackageCall(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "LevelConfig.kt")
	generatorFile := filepath.Join(dir, "LevelGenerator.kt")
	require.NoError(t, os.WriteFile(configFile, []byte(`package arena.core

data class LevelConfig(val generator: LevelGenerator)
`), 0o644))
	require.NoError(t, os.WriteFile(generatorFile, []byte(`package arena.core

class LevelGenerator(val config: LevelConfig)
`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph := depgraph.MustDependencyGraph(map[string][]string{
		configFile:    {generatorFile},
		generatorFile: {configFile},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	evidence := fileGraph.Meta.Edges[depgraph.FileEdge{From: configFile, To: generatorFile}].Evidence
	assert.Contains(t, evidence, depgraph.DependencyEvidence{
		Symbol:          "LevelGenerator",
		Kind:            "kotlin-same-package-type-reference",
		Relationship:    depgraph.RelationshipTypeReference,
		ReferenceFile:   configFile,
		ReferenceLine:   3,
		DeclarationFile: generatorFile,
		DeclarationLine: 3,
		Confidence:      depgraph.EvidenceConfidenceMedium,
	})
}

func TestAttachEvidence_HTMLNavigationAndScript(t *testing.T) {
	dir := t.TempDir()
	indexFile := filepath.Join(dir, "index.html")
	aboutFile := filepath.Join(dir, "about.html")
	scriptFile := filepath.Join(dir, "app.js")
	require.NoError(t, os.WriteFile(indexFile, []byte(`<a href="about.html">About</a>
<script src="app.js"></script>
`), 0o644))
	require.NoError(t, os.WriteFile(aboutFile, []byte(
		`<a href="index.html">Home</a>`), 0o644))
	require.NoError(t, os.WriteFile(scriptFile, []byte(`console.log("app");`), 0o644))

	reader := vcs.FilesystemContentReader()
	graph := depgraph.MustDependencyGraph(map[string][]string{
		indexFile:  {aboutFile, scriptFile},
		aboutFile:  {indexFile},
		scriptFile: {indexFile},
	})
	fileGraph, err := depgraph.NewFileDependencyGraph(graph, nil, reader)
	require.NoError(t, err)

	explain.AttachEvidence(&fileGraph, reader)

	navigation := fileGraph.Meta.Edges[depgraph.FileEdge{From: indexFile, To: aboutFile}].Evidence
	assert.Contains(t, navigation, depgraph.DependencyEvidence{
		Symbol:          "about.html",
		Kind:            "html-a-href",
		Relationship:    depgraph.RelationshipNavigation,
		ReferenceFile:   indexFile,
		ReferenceLine:   1,
		DeclarationFile: aboutFile,
		DeclarationLine: 1,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
	script := fileGraph.Meta.Edges[depgraph.FileEdge{From: indexFile, To: scriptFile}].Evidence
	assert.Contains(t, script, depgraph.DependencyEvidence{
		Symbol:          "app.js",
		Kind:            "html-script-src",
		Relationship:    depgraph.RelationshipScript,
		ReferenceFile:   indexFile,
		ReferenceLine:   2,
		DeclarationFile: scriptFile,
		DeclarationLine: 1,
		Confidence:      depgraph.EvidenceConfidenceHigh,
	})
}
