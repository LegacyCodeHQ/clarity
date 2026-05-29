package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)

		case <-gitStateTicker.C:
			stateSig, err := git.GetRepositoryStateSignature(repoPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "git state read error: %v\n", err)
				continue
			}
			if stateSig == lastGitStateSig {
				continue
			}

			headSig := extractHEADSignature(stateSig)
			headChanged := headSig != "" && headSig != lastHeadSig
			lastGitStateSig = stateSig
			lastHeadSig = headSig
			if headChanged {
				b.archiveWorkingSet(repoID)
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
	dot, err := buildDOTGraph(repoPath, opts, formatter)
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
