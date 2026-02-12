package diff

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffDefaultsToWorkingTreeMode(t *testing.T) {
	cmd := NewCommand()
	comparison, err := resolveModeAndCommitComparison(cmd, ".", "")
	if err != nil {
		t.Fatalf("resolveModeAndCommitComparison() error = %v", err)
	}
	if comparison.mode != diffModeWorkingTree {
		t.Fatalf("expected working-tree mode, got %q", comparison.mode)
	}
}

func TestDiffCommitSingleCommit_ValidRef(t *testing.T) {
	repoDir, head := initGitRepoWithSingleCommit(t)

	cmd := NewCommand()
	comparison, err := resolveModeAndCommitComparison(cmd, repoDir, head)
	if err != nil {
		t.Fatalf("resolveModeAndCommitComparison() error = %v", err)
	}
	if comparison.mode != diffModeCommit {
		t.Fatalf("expected commit mode, got %q", comparison.mode)
	}
	if comparison.baseRef != "" {
		t.Fatalf("expected empty base for single-commit mode, got %q", comparison.baseRef)
	}
	if comparison.targetRef != head {
		t.Fatalf("expected target ref %q, got %q", head, comparison.targetRef)
	}
}

func TestDiffCommitPair_ValidRefsWithWhitespace(t *testing.T) {
	repoDir, firstCommit, secondCommit := initGitRepoWithTwoCommits(t)

	cmd := NewCommand()
	comparison, err := resolveModeAndCommitComparison(cmd, repoDir, "  "+firstCommit+" , "+secondCommit+"  ")
	if err != nil {
		t.Fatalf("resolveModeAndCommitComparison() error = %v", err)
	}
	if comparison.mode != diffModeCommit {
		t.Fatalf("expected commit mode, got %q", comparison.mode)
	}
	if comparison.baseRef != firstCommit {
		t.Fatalf("expected base ref %q, got %q", firstCommit, comparison.baseRef)
	}
	if comparison.targetRef != secondCommit {
		t.Fatalf("expected target ref %q, got %q", secondCommit, comparison.targetRef)
	}
}

func TestDiffCommitPairRejectsExtraCommas(t *testing.T) {
	repoDir, firstCommit, secondCommit := initGitRepoWithTwoCommits(t)

	cmd := NewCommand()
	_, err := resolveModeAndCommitComparison(cmd, repoDir, firstCommit+","+secondCommit+",HEAD")
	if err == nil {
		t.Fatal("expected error for malformed commit list")
	}
	if !strings.Contains(err.Error(), "expected <commit> or <A>,<B>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiffCommitPairRejectsEmptyRefs(t *testing.T) {
	tests := []string{"A,", ",B", "  ,  "}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			cmd := NewCommand()
			_, err := resolveModeAndCommitComparison(cmd, ".", tc)
			if err == nil {
				t.Fatal("expected empty-ref error")
			}
			if !strings.Contains(err.Error(), "both refs are required") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDiffCommitRejectsInvalidRef(t *testing.T) {
	repoDir, _ := initGitRepoWithSingleCommit(t)

	cmd := NewCommand()
	_, err := resolveModeAndCommitComparison(cmd, repoDir, "not-a-ref")
	if err == nil {
		t.Fatal("expected invalid ref error")
	}
	if !strings.Contains(err.Error(), "invalid commit reference") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiffCommitConflictsWithSnapshotSelector(t *testing.T) {
	repoDir, head := initGitRepoWithSingleCommit(t)

	cmd := NewCommand()
	if err := cmd.Flags().Set("staged", "true"); err != nil {
		t.Fatalf("set staged flag: %v", err)
	}

	_, err := resolveModeAndCommitComparison(cmd, repoDir, head)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "--commit cannot be combined with --staged") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiffCommandAcceptsSummaryFlag(t *testing.T) {
	repoDir, _ := initGitRepoWithSingleCommit(t)

	cmd := NewCommand()
	cmd.SetArgs([]string{"--summary", "--repo", repoDir})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}
}

func TestRenderDelta_UnsupportedFormat(t *testing.T) {
	_, err := renderDelta("json", graphDelta{})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	if !strings.Contains(err.Error(), "not supported for diff") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderDelta_Mermaid_UnchangedNodesUseDottedBorder(t *testing.T) {
	delta := graphDelta{
		edgesAdded: []graphEdge{
			{from: "/repo/changed.go", to: "/repo/unchanged.go"},
			{from: "/repo/changed.go", to: "/repo/changed2.go"},
			{from: "/repo/unchanged.go", to: "/repo/unchanged2.go"},
		},
		changedNodes: map[string]struct{}{
			"/repo/changed.go":  {},
			"/repo/changed2.go": {},
		},
	}

	out, err := renderDelta("mermaid", delta)
	if err != nil {
		t.Fatalf("renderDelta(mermaid) error = %v", err)
	}
	if strings.Contains(out, "✨") {
		t.Fatalf("expected plain edges without sparkle labels, got:\n%s", out)
	}
	if !strings.Contains(out, "classDef unchanged") || !strings.Contains(out, "stroke-dasharray") {
		t.Fatalf("expected unchanged class with dotted border, got:\n%s", out)
	}
}

func initGitRepoWithSingleCommit(t *testing.T) (repoDir string, commit string) {
	t.Helper()

	repoDir = t.TempDir()
	gitInitRepo(t, repoDir)

	filePath := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	gitRun(t, repoDir, "add", "main.go")
	gitRun(t, repoDir, "commit", "-m", "initial commit")

	commit = strings.TrimSpace(gitOutput(t, repoDir, "rev-parse", "HEAD"))
	return repoDir, commit
}

func initGitRepoWithTwoCommits(t *testing.T) (repoDir string, firstCommit string, secondCommit string) {
	t.Helper()

	repoDir, firstCommit = initGitRepoWithSingleCommit(t)

	filePath := filepath.Join(repoDir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	gitRun(t, repoDir, "add", "main.go")
	gitRun(t, repoDir, "commit", "-m", "second commit")
	secondCommit = strings.TrimSpace(gitOutput(t, repoDir, "rev-parse", "HEAD"))
	return repoDir, firstCommit, secondCommit
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

func gitOutput(t *testing.T, repoDir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v\nstderr: %s", args, err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String()
}
