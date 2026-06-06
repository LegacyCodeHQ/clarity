package cycles

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCycles_DetectsCircularDependency(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir)

	if !strings.Contains(out, "a.mjs") || !strings.Contains(out, "b.mjs") {
		t.Fatalf("expected cycle output to mention a.mjs and b.mjs, got:\n%s", out)
	}
	if !strings.Contains(out, "→") {
		t.Fatalf("expected a cycle arrow in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Found 1 cycle") {
		t.Fatalf("expected a one-cycle summary, got:\n%s", out)
	}
}

func TestCycles_NoCycles_ReportsNone(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `export function b() {}
`)

	out := executeCycles(t, dir)

	if !strings.Contains(strings.ToLower(out), "no cycles") {
		t.Fatalf("expected a 'no cycles' message, got:\n%s", out)
	}
}

func TestCycles_RelativizesToScope(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	writeFile(t, filepath.Join(sub, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(sub, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir)

	if !strings.Contains(out, "pkg/a.mjs"+cycleArrow) {
		t.Fatalf("expected cycle paths relative to scope (pkg/a.mjs), got:\n%s", out)
	}
	if strings.Contains(out, filepath.Join(dir, "pkg", "a.mjs")) {
		t.Fatalf("expected cycle file paths to be relative, not absolute, got:\n%s", out)
	}
}

func TestCycles_URLFlag_EmitsDiagramLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir, "-u")

	if !strings.Contains(out, "https://") || !strings.Contains(out, "dreampuf.github.io") {
		t.Fatalf("expected a visualization URL with -u, got:\n%s", out)
	}
	if !strings.Contains(out, "a.mjs"+cycleArrow) {
		t.Fatalf("expected -u output to still include the cycle path, got:\n%s", out)
	}
}

func TestCycles_NoURLByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mjs"), `import { b } from "./b.mjs";
export function a() {}
`)
	writeFile(t, filepath.Join(dir, "b.mjs"), `import { a } from "./a.mjs";
export function b() {}
`)

	out := executeCycles(t, dir)

	if strings.Contains(out, "https://") {
		t.Fatalf("expected no URL without -u, got:\n%s", out)
	}
}

func executeCycles(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewCommand()
	cmd.SetArgs(args)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}
	return stdout.String()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
}
