package graph

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
	if !strings.Contains(output, `"App.java"`) || !strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to include App.java and Helper.java nodes, got:\n%s", output)
	}
	if !strings.Contains(output, `"App.java" -> "Helper.java"`) {
		t.Fatalf("expected Java import edge App.java -> Helper.java, got:\n%s", output)
	}
}

func TestGraphAlias_PrintsDeprecationWarning(t *testing.T) {
	repoDir := t.TempDir()
	supportedFile := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(supportedFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	root := &cobra.Command{Use: "clarity"}
	root.AddCommand(NewCommand())
	root.SetArgs([]string{"graph", "-i", supportedFile, "-f", "dot", "--allow-outside-repo"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	if err := root.Execute(); err != nil {
		t.Fatalf("root.Execute() error = %v", err)
	}

	if !strings.Contains(stderr.String(), "deprecated") {
		t.Fatalf("expected deprecation warning for graph alias, got:\n%s", stderr.String())
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
	if !strings.Contains(output, `"App.java"`) || !strings.Contains(output, `"Helper.java"`) {
		t.Fatalf("expected graph output to include App.java and Helper.java nodes, got:\n%s", output)
	}
	if !strings.Contains(output, `"App.java" -> "Helper.java"`) {
		t.Fatalf("expected Java import edge App.java -> Helper.java, got:\n%s", output)
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

func gitInitRepo(t *testing.T, repoDir string) {
	t.Helper()

	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.name", "test")
	gitRun(t, repoDir, "config", "user.email", "test@example.com")
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
