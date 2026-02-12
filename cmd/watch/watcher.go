package watch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LegacyCodeHQ/clarity/depgraph/registry"
	"github.com/fsnotify/fsnotify"
)

const debounceInterval = 300 * time.Millisecond

var skippedDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	".dart_tool":  true,
	"build":       true,
	"__pycache__":  true,
	".gradle":     true,
	".idea":       true,
	".vscode":     true,
}

func watchAndRebuild(ctx context.Context, repoPath string, opts *watchOptions, b *broker) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	if err := addWatchDirs(watcher, repoPath); err != nil {
		return fmt.Errorf("failed to watch directories: %w", err)
	}

	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if !isRelevantChange(event) {
				continue
			}

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceInterval, func() {
				dot, err := buildDOTGraph(repoPath, opts)
				if errors.Is(err, errNoUncommittedChanges) {
					return
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "graph rebuild error: %v\n", err)
					return
				}
				b.publish(dot)
			})

			if event.Has(fsnotify.Create) {
				addIfDirectory(watcher, event.Name)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		}
	}
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
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
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
