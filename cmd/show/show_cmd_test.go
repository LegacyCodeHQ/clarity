package show

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/internal/testhelpers"
)

func TestGraphInputDirectory_WithJavaFiles_RendersDependencyEdges(t *testing.T) {
	repoDir := t.TempDir()
	dir := filepath.Join(repoDir, "src", "main", "java", "com", "example")
	if err := os.MkdirAll(filepath.Join(dir, "util"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	appFile := filepath.Join(dir, "App.java")
	appContent := `package com.example;

import com.example.util.Helper;

public class App {}
`
	if err := os.WriteFile(appFile, []byte(appContent), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	helperFile := filepath.Join(dir, "util", "Helper.java")
	if err := os.WriteFile(helperFile, []byte("package com.example.util;\n\npublic class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", filepath.Join(repoDir, "src"), "-f", "dot", "--allow-outside-repo"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "App.java") || !strings.Contains(output, "Helper.java") {
		t.Fatalf("expected graph output to include App.java and Helper.java nodes, got:\n%s", output)
	}
	if !strings.Contains(output, `App.java" -> "`) || !strings.Contains(output, `Helper.java"`) {
		t.Fatalf("expected Java import edge App.java -> Helper.java, got:\n%s", output)
	}
}

func TestGraphInput_WithMJSFiles_RendersDependencyEdges(t *testing.T) {
	repoDir := t.TempDir()
	testFile := filepath.Join(repoDir, "viewer_state.test.mjs")
	stateFile := filepath.Join(repoDir, "viewer_state.mjs")

	testContent := `import {
  getViewModel,
} from "./viewer_state.mjs";
`
	if err := os.WriteFile(testFile, []byte(testContent), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(stateFile, []byte("export function getViewModel() {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", testFile + "," + stateFile, "-f", "dot", "--allow-outside-repo"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"viewer_state.test.mjs"`) || !strings.Contains(output, `"viewer_state.mjs"`) {
		t.Fatalf("expected graph output to include mjs nodes, got:\n%s", output)
	}
	if !strings.Contains(output, `viewer_state.test.mjs" -> "`) || !strings.Contains(output, `viewer_state.mjs"`) {
		t.Fatalf("expected mjs import edge viewer_state.test.mjs -> viewer_state.mjs, got:\n%s", output)
	}
}

func TestGraphCommit_WithJavaFiles_RendersDependencyEdges(t *testing.T) {
	repoDir := t.TempDir()
	gitInitRepo(t, repoDir)

	appFile := filepath.Join(repoDir, "src", "main", "java", "com", "example", "App.java")
	helperFile := filepath.Join(repoDir, "src", "main", "java", "com", "example", "util", "Helper.java")
	if err := os.MkdirAll(filepath.Dir(helperFile), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	appContent := `package com.example;

import com.example.util.Helper;

public class App {}
`
	if err := os.WriteFile(appFile, []byte(appContent), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(helperFile, []byte("package com.example.util;\n\npublic class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "add java file")

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-c", "HEAD", "-f", "dot"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "App.java") || !strings.Contains(output, "Helper.java") {
		t.Fatalf("expected graph output to include App.java and Helper.java nodes, got:\n%s", output)
	}
	if !strings.Contains(output, `App.java" -> "`) || !strings.Contains(output, `Helper.java"`) {
		t.Fatalf("expected Java import edge App.java -> Helper.java, got:\n%s", output)
	}
}

func TestGraphCommit_RendersDeletedFileNode(t *testing.T) {
	repoDir := t.TempDir()
	gitInitRepo(t, repoDir)

	keeperFile := filepath.Join(repoDir, "keeper.ts")
	obsoleteFile := filepath.Join(repoDir, "obsolete.ts")

	// Commit 1: keeper imports obsolete; both files exist.
	if err := os.WriteFile(keeperFile, []byte("import { gone } from './obsolete';\nexport const k = gone;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(obsoleteFile, []byte("export const gone = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	gitRun(t, repoDir, "add", "-A")
	gitRun(t, repoDir, "commit", "-m", "add files")

	// Commit 2: delete obsolete.ts while keeper still references it (a dangling
	// import to a removed file is exactly what the deleted node should surface).
	if err := os.Remove(obsoleteFile); err != nil {
		t.Fatalf("os.Remove() error = %v", err)
	}
	if err := os.WriteFile(keeperFile, []byte("import { gone } from './obsolete';\nexport const k = gone + 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	gitRun(t, repoDir, "add", "-A")
	gitRun(t, repoDir, "commit", "-m", "delete obsolete")

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-c", "HEAD", "-f", "dot"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "obsolete.ts") {
		t.Fatalf("expected deleted obsolete.ts node in output, got:\n%s", output)
	}
	if !strings.Contains(output, "(deleted)") {
		t.Fatalf("expected obsolete.ts to be marked (deleted), got:\n%s", output)
	}
	if !strings.Contains(output, `keeper.ts" -> "`) {
		t.Fatalf("expected edge from keeper.ts to the deleted node, got:\n%s", output)
	}
}

func TestGraphCommit_RendersRenameAsMoveNotDeletion(t *testing.T) {
	repoDir := t.TempDir()
	gitInitRepo(t, repoDir)

	oldFile := filepath.Join(repoDir, "old.ts")
	newFile := filepath.Join(repoDir, "new.ts")
	body := []byte("export const value = 1;\n")

	if err := os.WriteFile(oldFile, body, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	gitRun(t, repoDir, "add", "-A")
	gitRun(t, repoDir, "commit", "-m", "add old.ts")

	// Rename old.ts -> new.ts with identical content.
	if err := os.Remove(oldFile); err != nil {
		t.Fatalf("os.Remove() error = %v", err)
	}
	if err := os.WriteFile(newFile, body, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	gitRun(t, repoDir, "add", "-A")
	gitRun(t, repoDir, "commit", "-m", "rename old.ts to new.ts")

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-c", "HEAD", "-f", "dot"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "(renamed)") {
		t.Fatalf("expected old.ts to be marked (renamed), got:\n%s", output)
	}
	if strings.Contains(output, "(deleted)") {
		t.Fatalf("expected a rename not to render as (deleted), got:\n%s", output)
	}
}

func TestGraphCommit_WithInput_UsesCommitTreePaths(t *testing.T) {
	repoDir := t.TempDir()
	gitInitRepo(t, repoDir)

	if err := os.MkdirAll(filepath.Join(repoDir, "cmd", "graph"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	originalPath := filepath.Join(repoDir, "cmd", "graph", "formatter_factory.go")
	renamedPath := filepath.Join(repoDir, "cmd", "graph", "formatter.go")
	fileContent := "package graph\n\nfunc formatterName() string { return \"factory\" }\n"

	if err := os.WriteFile(originalPath, []byte(fileContent), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "add formatter factory file")

	// Simulate a working tree rename not present in HEAD.
	if err := os.Rename(originalPath, renamedPath); err != nil {
		t.Fatalf("os.Rename() error = %v", err)
	}
	gitRun(t, repoDir, "add", "-A")

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-c", "HEAD",
		"-i", "cmd/graph",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "formatter_factory.go") {
		t.Fatalf("expected graph output to include formatter_factory.go from HEAD, got:\n%s", output)
	}
	if strings.Contains(output, `"formatter.go"`) {
		t.Fatalf("expected graph output not to include working-tree formatter.go path, got:\n%s", output)
	}
}

func TestGraphInput_WithSupportedFiles_RendersNode(t *testing.T) {
	repoDir := t.TempDir()
	supportedFile := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(supportedFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", supportedFile, "-f", "dot", "--allow-outside-repo"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", stdout.String())
	}
}

func TestGraphInput_WithJSONFormat_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	supportedFile := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(supportedFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", supportedFile, "-f", "json", "--allow-outside-repo"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("cmd.Execute() expected error for json format, got nil")
	}
	if !strings.Contains(err.Error(), "unknown format: json (valid options: dot, mermaid)") {
		t.Fatalf("expected unknown format error including input value, got: %v", err)
	}
}

func TestGraphInput_Exclude_RemovesSpecificFile(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	javaFile := filepath.Join(repoDir, "Helper.java")

	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(javaFile, []byte("public class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-i", repoDir,
		"-f", "dot",
		"--exclude", "Helper.java",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", output)
	}
	if strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to exclude Helper.java node, got:\n%s", output)
	}
}

func TestGraphInput_Exclude_RemovesDirectory(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "internal"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	goFile := filepath.Join(repoDir, "main.go")
	internalFile := filepath.Join(repoDir, "internal", "helper.go")

	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(internalFile, []byte("package internal\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-i", repoDir,
		"-f", "dot",
		"--exclude", "internal",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", output)
	}
	if strings.Contains(output, `"helper.go"`) {
		t.Fatalf("expected graph output to exclude helper.go node, got:\n%s", output)
	}
}

func TestGraphInput_Exclude_AllFiles_ReturnsError(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-i", repoDir,
		"-f", "dot",
		"--exclude", repoDir,
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when --exclude removes all files")
	}
	if !strings.Contains(err.Error(), "no files remain after applying --exclude") {
		t.Fatalf("expected exclude error, got: %v", err)
	}
}

func TestGraphInput_IncludeExt_KeepsOnlyMatchingExtension(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	javaFile := filepath.Join(repoDir, "Helper.java")

	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(javaFile, []byte("public class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", repoDir, "-f", "dot", "--allow-outside-repo", "--include-ext", ".go"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", output)
	}
	if strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to exclude Helper.java node, got:\n%s", output)
	}
}

func TestGraphInput_IncludeExt_MultipleExtensions_AreAccepted(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	javaFile := filepath.Join(repoDir, "Helper.java")
	pyFile := filepath.Join(repoDir, "tool.py")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(javaFile, []byte("public class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(pyFile, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", repoDir, "-f", "dot", "--allow-outside-repo", "--include-ext", "go,.java"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", output)
	}
	if !strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to include Helper.java node, got:\n%s", output)
	}
	if strings.Contains(output, `"tool.py"`) {
		t.Fatalf("expected graph output to exclude tool.py node, got:\n%s", output)
	}
}

func TestGraphInput_IncludeExtAndExcludeExt_ExcludeWins(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	javaFile := filepath.Join(repoDir, "Helper.java")
	pyFile := filepath.Join(repoDir, "tool.py")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(javaFile, []byte("public class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(pyFile, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-i", repoDir, "-f", "dot", "--allow-outside-repo",
		"--include-ext", ".go,.java",
		"--exclude-ext", ".java",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", output)
	}
	if strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to exclude Helper.java node, got:\n%s", output)
	}
	if strings.Contains(output, `"tool.py"`) {
		t.Fatalf("expected graph output to exclude tool.py node, got:\n%s", output)
	}
}

func TestGraphInput_ExcludeExt_SkipsMatchingExtension(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	javaFile := filepath.Join(repoDir, "Helper.java")

	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(javaFile, []byte("public class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", repoDir, "-f", "dot", "--allow-outside-repo", "--exclude-ext", ".java"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", output)
	}
	if strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to exclude Helper.java node, got:\n%s", output)
	}
}

func TestGraphInput_ExcludeExt_WithoutDot_IsAccepted(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	javaFile := filepath.Join(repoDir, "Helper.java")

	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(javaFile, []byte("public class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", repoDir, "-f", "dot", "--allow-outside-repo", "--exclude-ext", "java"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to exclude Helper.java node, got:\n%s", output)
	}
}

func TestGraphInput_ExcludeExt_MultipleExtensions_AreAccepted(t *testing.T) {
	repoDir := t.TempDir()
	goFile := filepath.Join(repoDir, "main.go")
	javaFile := filepath.Join(repoDir, "Helper.java")
	pyFile := filepath.Join(repoDir, "tool.py")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(javaFile, []byte("public class Helper {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(pyFile, []byte("print('hello')\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", repoDir, "-f", "dot", "--allow-outside-repo", "--exclude-ext", ".go,.java"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if strings.Contains(output, `"main.go"`) {
		t.Fatalf("expected graph output to exclude main.go node, got:\n%s", output)
	}
	if strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to exclude Helper.java node, got:\n%s", output)
	}
	if !strings.Contains(output, `"tool.py"`) {
		t.Fatalf("expected graph output to include tool.py node, got:\n%s", output)
	}
}

func TestGraphInputRelativePath_WithRepo_ResolvesFromRepoRoot(t *testing.T) {
	repoDir := t.TempDir()
	relativePath := filepath.Join("src", "main.go")
	absolutePath := filepath.Join(repoDir, relativePath)

	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(absolutePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-i", relativePath, "-f", "dot"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", stdout.String())
	}
}

func TestGraphBetweenRelativePaths_WithRepo_ResolvesFromRepoRoot(t *testing.T) {
	repoDir := t.TempDir()
	leftFile := filepath.Join(repoDir, "a.go")
	rightFile := filepath.Join(repoDir, "b.go")
	for _, filePath := range []string{leftFile, rightFile} {
		if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-w", "a.go,b.go", "-f", "dot"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.go"`) || !strings.Contains(output, `"b.go"`) {
		t.Fatalf("expected graph output to include a.go and b.go nodes, got:\n%s", output)
	}
}

func TestGraphFileRelativePath_WithRepo_ResolvesFromRepoRoot(t *testing.T) {
	repoDir := t.TempDir()
	targetRelativePath := filepath.Join("pkg", "main.go")
	targetAbsolutePath := filepath.Join(repoDir, targetRelativePath)

	if err := os.MkdirAll(filepath.Dir(targetAbsolutePath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetAbsolutePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-r", repoDir, "-p", targetRelativePath, "-f", "dot"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"main.go"`) {
		t.Fatalf("expected graph output to include main.go node, got:\n%s", stdout.String())
	}
}

func TestGraphFileScopeDownstream_LevelZero_IncludesTransitiveOutgoingOnly(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	bFile := filepath.Join(repoDir, "b.ts")
	cFile := filepath.Join(repoDir, "c.ts")
	upstreamFile := filepath.Join(repoDir, "x.ts")

	if err := os.WriteFile(aFile, []byte("import { b } from './b';\nexport const a = b;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bFile, []byte("import { c } from './c';\nexport const b = c;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(cFile, []byte("export const c = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(upstreamFile, []byte("import { a } from './a';\nexport const x = a;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"--scope", "downstream",
		"-l", "0",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.ts"`) || !strings.Contains(output, `"b.ts"`) || !strings.Contains(output, `"c.ts"`) {
		t.Fatalf("expected downstream transitive graph to include a.ts, b.ts, c.ts, got:\n%s", output)
	}
	if !strings.Contains(output, `a.ts" -> "`) || !strings.Contains(output, `b.ts" -> "`) || !strings.Contains(output, `c.ts"`) {
		t.Fatalf("expected downstream transitive edges a.ts->b.ts and b.ts->c.ts, got:\n%s", output)
	}
	if strings.Contains(output, `"x.ts"`) {
		t.Fatalf("expected downstream scope to exclude upstream dependent x.ts, got:\n%s", output)
	}
}

func TestGraphFile_DefaultScope_IsDownstreamAtLevelOne(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	bFile := filepath.Join(repoDir, "b.ts")
	cFile := filepath.Join(repoDir, "c.ts")
	upstreamFile := filepath.Join(repoDir, "x.ts")

	if err := os.WriteFile(aFile, []byte("import { b } from './b';\nexport const a = b;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bFile, []byte("import { c } from './c';\nexport const b = c;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(cFile, []byte("export const c = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(upstreamFile, []byte("import { a } from './a';\nexport const x = a;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"-l", "1",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.ts"`) || !strings.Contains(output, `"b.ts"`) {
		t.Fatalf("expected default scope to include a.ts with immediate downstream nodes, got:\n%s", output)
	}
	if strings.Contains(output, `"x.ts"`) {
		t.Fatalf("expected default downstream scope to exclude upstream dependent x.ts, got:\n%s", output)
	}
	if strings.Contains(output, `"c.ts"`) {
		t.Fatalf("expected level 1 to exclude transitive downstream node c.ts, got:\n%s", output)
	}
}

func TestGraphFile_InvalidScope_ReturnsError(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"-p", "a.ts", "--scope", "sideways"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for invalid --scope")
	}
	if !strings.Contains(err.Error(), "unknown scope: sideways") {
		t.Fatalf("expected unknown scope error, got: %v", err)
	}
}

func TestGraphFile_PruneStopsTraversal(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	bFile := filepath.Join(repoDir, "b.ts")
	cFile := filepath.Join(repoDir, "c.ts")
	dFile := filepath.Join(repoDir, "d.ts")

	if err := os.WriteFile(aFile, []byte("import { b } from './b';\nexport const a = b;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bFile, []byte("import { c } from './c';\nexport const b = c;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(cFile, []byte("import { d } from './d';\nexport const c = d;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(dFile, []byte("export const d = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"--prune", "b.ts",
		"-l", "0",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.ts"`) {
		t.Fatalf("expected a.ts in output, got:\n%s", output)
	}
	if !strings.Contains(output, `"b.ts"`) {
		t.Fatalf("expected pruned node b.ts to still appear in output, got:\n%s", output)
	}
	if strings.Contains(output, `"c.ts"`) {
		t.Fatalf("expected c.ts (descendant of pruned b.ts) to be excluded, got:\n%s", output)
	}
	if strings.Contains(output, `"d.ts"`) {
		t.Fatalf("expected d.ts (descendant of pruned b.ts) to be excluded, got:\n%s", output)
	}
	if !strings.Contains(output, `dashed`) {
		t.Fatalf("expected pruned node b.ts to have dashed style, got:\n%s", output)
	}
}

func TestGraphFile_PruneDoesNotBlockNodesReachableViaOtherPaths(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	bFile := filepath.Join(repoDir, "b.ts")
	cFile := filepath.Join(repoDir, "c.ts")
	dFile := filepath.Join(repoDir, "d.ts")

	// a -> b -> d, a -> c -> d. Prune b; d should still appear via c.
	if err := os.WriteFile(aFile, []byte("import { b } from './b';\nimport { c } from './c';\nexport const a = b + c;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bFile, []byte("import { d } from './d';\nexport const b = d;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(cFile, []byte("import { d } from './d';\nexport const c = d;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(dFile, []byte("export const d = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"--prune", "b.ts",
		"-l", "0",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.ts"`) || !strings.Contains(output, `"b.ts"`) || !strings.Contains(output, `"c.ts"`) || !strings.Contains(output, `"d.ts"`) {
		t.Fatalf("expected all four nodes (d reachable via c), got:\n%s", output)
	}
}

func TestGraphFile_PruneWithoutFile_ReturnsError(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"--prune", "b.ts"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when --prune is used without --file")
	}
	if !strings.Contains(err.Error(), "--prune requires --file flag") {
		t.Fatalf("expected prune requires file error, got: %v", err)
	}
}

func TestRepoLabelName_UsesGoModuleNameWhenPresent(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "clarity-cli")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module github.com/LegacyCodeHQ/clarity\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	got := repoLabelName(repoDir)
	if got != "clarity" {
		t.Fatalf("repoLabelName() = %q, want %q", got, "clarity")
	}
}

func TestRepoLabelName_FallsBackToRootDirectoryName(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "my-service")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	got := repoLabelName(repoDir)
	if got != "my-service" {
		t.Fatalf("repoLabelName() = %q, want %q", got, "my-service")
	}
}

func TestBuildGraphLabel_UsesGoModuleNamePrefix(t *testing.T) {
	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "clarity-cli")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module github.com/acme/clarity\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	gitInitRepo(t, repoDir)

	filePath := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "add main.go")

	label := buildGraphLabel(&graphOptions{repoPath: repoDir}, formatters.OutputFormatMermaid, "", "", false, []string{filePath})

	if !strings.HasPrefix(label, "clarity • ") {
		t.Fatalf("buildGraphLabel() = %q, want prefix %q", label, "clarity • ")
	}
	if !strings.Contains(label, " • 1 file") {
		t.Fatalf("buildGraphLabel() = %q, want file count suffix", label)
	}
}

func gitInitRepo(t *testing.T, repoDir string) {
	t.Helper()

	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.name", "test")
	gitRun(t, repoDir, "config", "user.email", "test@example.com")
}

func TestGraphFile_Also_IncludesConnectedTestFile(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	bFile := filepath.Join(repoDir, "b.ts")
	testFile := filepath.Join(repoDir, "a.test.ts")

	// a imports b; a.test.ts imports a
	if err := os.WriteFile(aFile, []byte("import { b } from './b';\nexport const a = b;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bFile, []byte("export const b = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(testFile, []byte("import { a } from './a';\nconsole.log(a);\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"-l", "0",
		"--also", "*.test.ts",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.ts"`) {
		t.Fatalf("expected a.ts in output, got:\n%s", output)
	}
	if !strings.Contains(output, `"b.ts"`) {
		t.Fatalf("expected b.ts in output, got:\n%s", output)
	}
	if !strings.Contains(output, `"a.test.ts"`) {
		t.Fatalf("expected a.test.ts in output (connected via import), got:\n%s", output)
	}
	if !strings.Contains(output, `a.test.ts" -> "`) {
		t.Fatalf("expected edge from a.test.ts, got:\n%s", output)
	}
}

func TestGraphFile_Also_ExcludesUnconnectedFiles(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	bFile := filepath.Join(repoDir, "b.ts")
	isolatedTest := filepath.Join(repoDir, "z.test.ts")

	// a imports b; z.test.ts has no connection to a or b
	if err := os.WriteFile(aFile, []byte("import { b } from './b';\nexport const a = b;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bFile, []byte("export const b = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(isolatedTest, []byte("export const z = 42;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"-l", "0",
		"--also", "*.test.ts",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.ts"`) {
		t.Fatalf("expected a.ts in output, got:\n%s", output)
	}
	if strings.Contains(output, `"z.test.ts"`) {
		t.Fatalf("expected z.test.ts to be excluded (no connection), got:\n%s", output)
	}
}

func TestGraphFile_Also_WithoutFile_ReturnsError(t *testing.T) {
	cmd := NewCommand()
	cmd.SetArgs([]string{"--also", "*.test.ts"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when --also is used without --file")
	}
	if !strings.Contains(err.Error(), "--also requires --file flag") {
		t.Fatalf("expected also requires file error, got: %v", err)
	}
}

func TestGraphFile_Also_MultiplePatterns(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	testFile := filepath.Join(repoDir, "a.test.ts")
	specFile := filepath.Join(repoDir, "a.spec.ts")

	if err := os.WriteFile(aFile, []byte("export const a = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(testFile, []byte("import { a } from './a';\nconsole.log(a);\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(specFile, []byte("import { a } from './a';\nconsole.log(a);\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"-l", "0",
		"--also", "*.test.ts,*.spec.ts",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.test.ts"`) {
		t.Fatalf("expected a.test.ts in output, got:\n%s", output)
	}
	if !strings.Contains(output, `"a.spec.ts"`) {
		t.Fatalf("expected a.spec.ts in output, got:\n%s", output)
	}
}

func TestGraphFile_Also_IncludesConnectedTestFiles_ExcludesUnconnected(t *testing.T) {
	repoDir := t.TempDir()

	// a → b → c (downstream chain)
	// a.test.ts → a (connected test)
	// b.test.ts → b (connected test)
	// z.test.ts (isolated, no imports)
	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
	}
	writeFile("a.ts", "import { b } from './b';\nexport const a = b;\n")
	writeFile("b.ts", "import { c } from './c';\nexport const b = c;\n")
	writeFile("c.ts", "export const c = 1;\n")
	writeFile("a.test.ts", "import { a } from './a';\nconsole.log(a);\n")
	writeFile("b.test.ts", "import { b } from './b';\nconsole.log(b);\n")
	writeFile("z.test.ts", "export const z = 42;\n")

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"-l", "0",
		"--also", "*.test.ts",
		"-f", "mermaid",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	g := testhelpers.MermaidGoldie(t)
	g.Assert(t, t.Name(), []byte(strings.TrimSpace(stdout.String())))
}

func TestGraphFile_Also_NoMatches_ReturnsOriginalGraph(t *testing.T) {
	repoDir := t.TempDir()
	aFile := filepath.Join(repoDir, "a.ts")
	bFile := filepath.Join(repoDir, "b.ts")

	if err := os.WriteFile(aFile, []byte("import { b } from './b';\nexport const a = b;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if err := os.WriteFile(bFile, []byte("export const b = 1;\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{
		"-r", repoDir,
		"-p", "a.ts",
		"-l", "0",
		"--also", "*.test.ts",
		"-f", "dot",
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"a.ts"`) || !strings.Contains(output, `"b.ts"`) {
		t.Fatalf("expected original graph nodes a.ts and b.ts, got:\n%s", output)
	}
}

func gitRun(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v\nstderr: %s", args, err, strings.TrimSpace(stderr.String()))
	}
}

const rustWithTestRegion = `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn it_adds() { assert_eq!(add(1, 2), 3); }
}
`

func TestGraphRust_PhantomDefaultOn(t *testing.T) {
	repoDir := t.TempDir()
	rsFile := filepath.Join(repoDir, "lib.rs")
	if err := os.WriteFile(rsFile, []byte(rustWithTestRegion), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", rsFile, "-f", "dot", "--allow-outside-repo"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "::tests") {
		t.Fatalf("baseline expected phantom node ::tests for rust with #[cfg(test)] region, got:\n%s", output)
	}
}

func TestGraphRust_NoPhantomFlag_SuppressesPhantom(t *testing.T) {
	repoDir := t.TempDir()
	rsFile := filepath.Join(repoDir, "lib.rs")
	if err := os.WriteFile(rsFile, []byte(rustWithTestRegion), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := NewCommand()
	cmd.SetArgs([]string{"-i", rsFile, "-f", "dot", "--allow-outside-repo", "--no-phantom"})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	output := stdout.String()
	if strings.Contains(output, "::tests") {
		t.Fatalf("--no-phantom should suppress ::tests phantom node, got:\n%s", output)
	}
	if !strings.Contains(output, "lib.rs") {
		t.Fatalf("expected lib.rs prod node to remain in output, got:\n%s", output)
	}
}
