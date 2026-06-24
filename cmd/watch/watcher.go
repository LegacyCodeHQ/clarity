package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
	"github.com/fsnotify/fsnotify"
)

const debounceInterval = 300 * time.Millisecond
const gitStatePollInterval = 500 * time.Millisecond

var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	".dart_tool":   true,
	"build":        true,
	"__pycache__":  true,
	".gradle":      true,
	".idea":        true,
	".vscode":      true,
}

func watchAndRebuild(ctx context.Context, repoID, repoPath string, opts *watchOptions, b *broker, formatter formatters.Formatter) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	if err := addWatchDirs(watcher, repoPath); err != nil {
		return fmt.Errorf("failed to watch directories: %w", err)
	}

	var debounceTimer *time.Timer
	var debounceC <-chan time.Time
	lastGitStateSig, err := git.GetRepositoryStateSignature(repoPath)
	lastHeadSig := extractHEADSignature(lastGitStateSig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "git state read error: %v\n", err)
	}
	gitStateTicker := time.NewTicker(gitStatePollInterval)
	defer gitStateTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				stopAndDrainTimer(debounceTimer)
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if !isRelevantChange(event) {
				continue
			}

			if debounceTimer == nil {
				debounceTimer = time.NewTimer(debounceInterval)
				debounceC = debounceTimer.C
			} else {
				stopAndDrainTimer(debounceTimer)
				debounceTimer.Reset(debounceInterval)
			}

			if event.Has(fsnotify.Create) {
				addIfDirectory(watcher, event.Name)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			// kqueue (macOS) reports a removed or replaced watched directory as
			// an ENOENT error rather than a Remove event. It is benign: the path
			// is already gone and its watch is self-pruned, so don't spam stderr.
			if isMissingPath(err) {
				slog.Debug("watched path removed", "error", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)

		case <-gitStateTicker.C:
			// Teardown backstop: if the worktree directory has vanished (its
			// `git worktree remove` REMOVE event may have been coalesced/dropped
			// by fsnotify during a batch removal), stop polling git against the
			// dead path and flip the tab to a finished, closable record. This is
			// independent of the meta-watcher, which is the primary trigger.
			if !pathExists(repoPath) {
				b.markRepoFinished(repoID)
				return nil
			}
			stateSig, err := git.GetRepositoryStateSignature(repoPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "git state read error: %v\n", err)
				continue
			}
			if stateSig == lastGitStateSig {
				continue
			}

			previousHeadSig := lastHeadSig
			headSig := extractHEADSignature(stateSig)
			headChanged := headSig != "" && headSig != lastHeadSig
			lastGitStateSig = stateSig
			lastHeadSig = headSig
			if headChanged {
				commitHistory, err := git.GetCommitHistory(repoPath, previousHeadSig, headSig)
				if err != nil {
					fmt.Fprintf(os.Stderr, "git commit history read error: %v\n", err)
				}
				b.archiveWorkingSetWithCommitHistory(repoID, commitHistory)
			}
			publishCurrentGraph(repoID, repoPath, opts, b, formatter)

		case <-debounceC:
			publishCurrentGraph(repoID, repoPath, opts, b, formatter)
			// Drop the timer too so the next event takes the
			// `debounceTimer == nil` branch and re-arms debounceC.
			// Without this, Reset would fire the timer into a nil
			// channel and no further debounced rebuilds would happen.
			debounceC = nil
			debounceTimer = nil
		}
	}
}

func stopAndDrainTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func publishCurrentGraph(repoID, repoPath string, opts *watchOptions, b *broker, formatter formatters.Formatter) {
	if isLinkedWorktreeTeardownSnapshot(repoPath) {
		b.markRepoFinished(repoID)
		return
	}

	dot, err := buildGraph(repoPath, opts, formatter)
	if errors.Is(err, errNoUncommittedChanges) {
		b.clearWorkingSet(repoID)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "graph rebuild error: %v\n", err)
		return
	}
	b.publish(repoID, dot)
}

func isLinkedWorktreeTeardownSnapshot(repoPath string) bool {
	if !pathExists(repoPath) {
		return false
	}

	isPrimary, err := git.IsPrimaryWorktree(repoPath)
	if err != nil || isPrimary {
		return false
	}

	nonDeletedChanges, err := git.GetUncommittedFiles(repoPath)
	if err != nil || len(nonDeletedChanges) > 0 {
		return false
	}

	deletedFiles, err := git.GetUncommittedDeletedFiles(repoPath)
	if err != nil || len(deletedFiles) == 0 {
		return false
	}

	trackedFiles, err := git.ListTrackedFiles(repoPath)
	if err != nil {
		return false
	}

	trackedSource := supportedPathSet(trackedFiles)
	if len(trackedSource) == 0 {
		return false
	}
	deletedSource := supportedPathSet(deletedFiles)

	return samePathSet(trackedSource, deletedSource)
}

func supportedPathSet(paths []string) map[string]bool {
	set := make(map[string]bool)
	for _, path := range paths {
		if registry.IsSupportedLanguageExtension(filepath.Ext(path)) {
			set[filepath.Clean(path)] = true
		}
	}
	return set
}

func samePathSet(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for path := range left {
		if !right[path] {
			return false
		}
	}
	return true
}

func isRelevantChange(event fsnotify.Event) bool {
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) &&
		!event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
		return false
	}
	ext := filepath.Ext(event.Name)
	return registry.IsSupportedLanguageExtension(ext)
}

func addWatchDirs(watcher *fsnotify.Watcher, root string) error {
	return addWatchDirsWithAdder(root, watcher.Add)
}

type watchDirAdder func(path string) error

func addWatchDirsWithAdder(root string, add watchDirAdder) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if isMissingPath(err) && path != root {
				return nil
			}
			return err
		}
		isDir := d.IsDir()
		if d.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(path)
			if err != nil {
				if isMissingPath(err) {
					return nil
				}
				return err
			}
			isDir = info.IsDir()
		}
		if isDir {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			if err := add(path); err != nil {
				if isMissingPath(err) {
					return nil
				}
				return err
			}
		}
		return nil
	})
}

func isMissingPath(err error) bool {
	return os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist)
}

// pathExists reports whether a filesystem path is currently present. A watcher
// uses it to detect that its worktree directory was removed so it can tear
// itself down instead of polling git against a dead path.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractHEADSignature(repositoryStateSignature string) string {
	if repositoryStateSignature == "" {
		return ""
	}
	headLine, _, found := strings.Cut(repositoryStateSignature, "\n")
	if !found {
		return strings.TrimSpace(repositoryStateSignature)
	}
	return strings.TrimSpace(headLine)
}

func addIfDirectory(watcher *fsnotify.Watcher, path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		_ = addWatchDirs(watcher, path)
	}
}
