package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestAddWatchDirsIgnoresMissingPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	root := t.TempDir()
	initGitRepo(t, root)
	worktree := filepath.Join(root, ".claude", "worktrees", "beautiful-gauss")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	linkPath := filepath.Join(worktree, "clarity.project")
	if err := os.Symlink("clarity/AGENTS.md", linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	defer watcher.Close()

	if err := addWatchDirs(watcher, root); err != nil {
		t.Fatalf("addWatchDirs: %v", err)
	}
}

func TestAddWatchDirsIgnoresMissingDirectoriesFromAdder(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "missing-dir")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.RemoveAll(target); err != nil {
		t.Fatalf("remove target: %v", err)
	}

	adder := func(path string) error {
		if path == target {
			return fs.ErrNotExist
		}
		return nil
	}

	if err := addWatchDirsWithAdder(root, adder); err != nil {
		t.Fatalf("addWatchDirsWithAdder: %v", err)
	}
}

func TestAddWatchDirsIgnoresRootRemovedBeforeGitInspection(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)

	// Simulates the fsnotify Create event fired for a transient scratch
	// directory such as Xcode/SwiftFormat's atomic-save folder
	// "(A Document Being Saved By swift-format)": it exists just long enough
	// to trigger a Create event, then is gone by the time the watcher gets
	// around to inspecting it.
	scratch := filepath.Join(root, "Sources", "(A Document Being Saved By swift-format)")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatalf("mkdir scratch: %v", err)
	}
	if err := os.RemoveAll(scratch); err != nil {
		t.Fatalf("remove scratch: %v", err)
	}

	err := addWatchDirsWithAdder(scratch, func(path string) error { return nil })
	if err != nil && !isMissingPath(err) {
		t.Fatalf("expected a vanished root directory to be treated as a benign missing-path race, got: %v", err)
	}
}

func TestAddWatchDirsSkipsBrokenSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	root := t.TempDir()
	initGitRepo(t, root)
	worktree := filepath.Join(root, ".claude", "worktrees", "beautiful-gauss")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	linkPath := filepath.Join(worktree, "clarity.project")
	if err := os.Symlink("clarity/AGENTS.md", linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	var added []string
	adder := func(path string) error {
		added = append(added, path)
		return nil
	}

	if err := addWatchDirsWithAdder(root, adder); err != nil {
		t.Fatalf("addWatchDirsWithAdder: %v", err)
	}

	for _, path := range added {
		if path == linkPath {
			t.Fatalf("expected broken symlink to be skipped, but was added")
		}
	}
}

func TestAddWatchDirsHonorsGitIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)

	if err := os.WriteFile(
		filepath.Join(root, ".gitignore"),
		[]byte(".build/\n"),
		0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, ".git", "info", "exclude"),
		[]byte(".cache/\n"),
		0o644); err != nil {
		t.Fatalf("write repository exclude: %v", err)
	}

	sourceDir := filepath.Join(root, "Sources", "App")
	ignoredDirs := []string{
		filepath.Join(root, ".build"),
		filepath.Join(root, ".build", "checkouts", "Dependency"),
		filepath.Join(root, ".cache"),
		filepath.Join(root, ".cache", "index"),
	}
	for _, dir := range append([]string{sourceDir}, ignoredDirs...) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	var added []string
	if err := addWatchDirsWithAdder(root, func(path string) error {
		added = append(added, path)
		return nil
	}); err != nil {
		t.Fatalf("addWatchDirsWithAdder: %v", err)
	}

	if !containsPath(added, sourceDir) {
		t.Fatalf("expected source directory to be watched, got %v", added)
	}
	for _, ignored := range ignoredDirs {
		if containsPath(added, ignored) {
			t.Fatalf("expected Git-ignored directory %s not to be watched", ignored)
		}
	}
}

func TestAddWatchDirsHonorsGitIgnoreForDirectoryCreatedAfterStartup(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)

	if err := os.WriteFile(
		filepath.Join(root, ".gitignore"),
		[]byte("Generated/\n"),
		0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	generated := filepath.Join(root, "Generated")
	nested := filepath.Join(generated, "deep", "output")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir generated tree: %v", err)
	}

	var added []string
	if err := addWatchDirsWithAdder(generated, func(path string) error {
		added = append(added, path)
		return nil
	}); err != nil {
		t.Fatalf("addWatchDirsWithAdder: %v", err)
	}

	if len(added) > 0 {
		t.Fatalf("expected newly created ignored tree not to be watched, got %v", added)
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
