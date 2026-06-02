package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/cmd/watch/protocol"
	"github.com/LegacyCodeHQ/clarity/internal/mcplogdlog"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
	"github.com/fsnotify/fsnotify"
)

type repoMode string

const (
	modePrimary repoMode = "primary"
	modeLinked  repoMode = "linked"
)

// gitdirAppearTimeout bounds how long the meta-watcher waits for a freshly
// created `<common>/worktrees/<name>/gitdir` file to appear before giving up
// on registering that worktree. `git worktree add` writes it within
// milliseconds, but we add headroom for slow disks/CI.
const gitdirAppearTimeout = 2 * time.Second

// planInitialRepos resolves which worktrees to watch when `clarity watch`
// starts in `cwd`. The first entry is always the cwd-tree, given the literal
// id "primary" so it's the default tab. In primary mode (cwd is the primary
// worktree), additional descriptors follow for each linked worktree.
func planInitialRepos(cwd string) ([]protocol.RepoDescriptor, repoMode, error) {
	isPrimary, err := git.IsPrimaryWorktree(cwd)
	if err != nil {
		return nil, "", err
	}

	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, "", err
	}

	if !isPrimary {
		return []protocol.RepoDescriptor{{
			ID:        primaryRepoID,
			Path:      cwdAbs,
			Label:     primaryRepoLabel(cwdAbs, currentBranchFor(cwdAbs)),
			IsPrimary: true,
			Active:    true,
		}}, modeLinked, nil
	}

	worktrees, err := git.ListWorktrees(cwdAbs)
	if err != nil {
		return nil, "", err
	}

	repos := []protocol.RepoDescriptor{{
		ID:        primaryRepoID,
		Path:      cwdAbs,
		Label:     primaryRepoLabel(cwdAbs, primaryBranch(worktrees)),
		IsPrimary: true,
		Active:    true,
	}}
	for _, w := range worktrees {
		if w.IsPrimary {
			continue
		}
		repos = append(repos, descriptorForLinked(w))
	}
	return repos, modePrimary, nil
}

func descriptorForLinked(w git.Worktree) protocol.RepoDescriptor {
	return protocol.RepoDescriptor{
		ID:        repoIDFor(w.Path, false),
		Path:      w.Path,
		Label:     linkedRepoLabel(w.Path),
		IsPrimary: false,
		Active:    true,
	}
}

func primaryBranch(worktrees []git.Worktree) string {
	for _, w := range worktrees {
		if w.IsPrimary {
			return w.Branch
		}
	}
	return ""
}

func currentBranchFor(path string) string {
	wts, err := git.ListWorktrees(path)
	if err != nil {
		return ""
	}
	pathResolved := resolveSymlinksOrSelf(path)
	for _, w := range wts {
		if resolveSymlinksOrSelf(w.Path) == pathResolved {
			return w.Branch
		}
	}
	return ""
}

func resolveSymlinksOrSelf(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

// runSupervisor is the multi-worktree replacement for the old single-call
// `watchAndRebuild` flow. It registers initial tabs with the broker, fans out
// one watcher goroutine per tree, and (in primary mode) installs a
// meta-watcher on `<common-git-dir>/worktrees/` to pick up `git worktree add`
// and `git worktree remove` events live.
//
// Returns when ctx is cancelled.
func runSupervisor(ctx context.Context, cwd string, opts *watchOptions, b *broker, formatter formatters.Formatter) error {
	repos, mode, err := planInitialRepos(cwd)
	if err != nil {
		return err
	}

	sup := &supervisor{
		b:              b,
		opts:           opts,
		formatter:      formatter,
		watchers:       make(map[string]context.CancelFunc),
		subdirToRepoID: make(map[string]string),
	}

	// In primary mode, install the meta-watcher BEFORE spawning initial
	// watchers so a `git worktree add` racing with startup is never missed.
	var metaDone <-chan struct{}
	if mode == modePrimary {
		commonDir, err := git.GetCommonDir(cwd)
		if err != nil {
			mcplogdlog.Error("watch: common-dir resolution failed", map[string]any{"error": err.Error()})
		} else {
			sup.commonDir = commonDir
			ready := make(chan struct{})
			done := make(chan struct{})
			go func() {
				if err := sup.runMetaWatcher(ctx, ready); err != nil {
					mcplogdlog.Error("watch: meta-watcher failed", map[string]any{"error": err.Error()})
				}
				close(done)
			}()
			<-ready
			metaDone = done
		}
	}

	for _, desc := range repos {
		sup.spawnWatcher(ctx, desc)
	}

	<-ctx.Done()
	sup.shutdown()
	if metaDone != nil {
		<-metaDone
	}
	return nil
}

type supervisor struct {
	b         *broker
	opts      *watchOptions
	formatter formatters.Formatter
	commonDir string

	mu             sync.Mutex
	watchers       map[string]context.CancelFunc // repoID -> cancel
	subdirToRepoID map[string]string             // worktree-subdir name -> repoID
}

func (s *supervisor) spawnWatcher(parent context.Context, desc protocol.RepoDescriptor) {
	s.b.registerRepo(desc)
	wctx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	s.watchers[desc.ID] = cancel
	s.mu.Unlock()
	go func() {
		if err := watchAndRebuild(wctx, desc.ID, desc.Path, s.opts, s.b, s.formatter); err != nil {
			fmt.Fprintf(os.Stderr, "watcher %s exited: %v\n", desc.ID, err)
		}
	}()
	// Seed an initial graph so the tab has content immediately.
	publishCurrentGraph(desc.ID, desc.Path, s.opts, s.b, s.formatter)
}

// finishWatcher stops monitoring a worktree whose git working tree was removed
// but keeps its tab: the file watcher is cancelled while the broker flips the
// tab to inactive and preserves its snapshot history. The tab survives as a
// frozen, read-only record until the user closes it (see broker.closeRepo).
func (s *supervisor) finishWatcher(repoID string) {
	s.mu.Lock()
	cancel, ok := s.watchers[repoID]
	delete(s.watchers, repoID)
	s.mu.Unlock()
	if ok {
		cancel()
	}
	s.b.markRepoFinished(repoID)
}

func (s *supervisor) shutdown() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.watchers))
	for _, c := range s.watchers {
		cancels = append(cancels, c)
	}
	s.watchers = make(map[string]context.CancelFunc)
	s.mu.Unlock()
	for _, c := range cancels {
		c()
	}
}

// runMetaWatcher installs an fsnotify watch on `<common>/worktrees/`. CREATE
// events fire when `git worktree add` registers a new linked worktree; REMOVE
// events fire on `git worktree remove`. Each registered subdir is mapped to a
// repoID so removals can tear down the right watcher.
// `ready` is closed once the fsnotify watch is in place, so callers can
// sequence work that must not race with the watch installation.
func (s *supervisor) runMetaWatcher(ctx context.Context, ready chan<- struct{}) error {
	closeReady := func() {
		if ready != nil {
			close(ready)
			ready = nil
		}
	}
	defer closeReady()

	worktreesDir := filepath.Join(s.commonDir, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("ensure worktrees dir: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("meta-watcher create: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(worktreesDir); err != nil {
		return fmt.Errorf("meta-watcher add %s: %w", worktreesDir, err)
	}

	// Seed the subdir → repoID map so REMOVE events on already-watched trees
	// can find their target.
	if entries, err := os.ReadDir(worktreesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if path := readWorktreeGitdir(filepath.Join(worktreesDir, e.Name())); path != "" {
				id := repoIDFor(path, false)
				s.mu.Lock()
				s.subdirToRepoID[e.Name()] = id
				s.mu.Unlock()
			}
		}
	}

	closeReady()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			subdir := filepath.Base(ev.Name)
			switch {
			case ev.Has(fsnotify.Create):
				s.handleWorktreeCreate(ctx, ev.Name, subdir)
			case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
				s.handleWorktreeRemove(subdir)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "meta-watcher error: %v\n", err)
		}
	}
}

func (s *supervisor) handleWorktreeCreate(ctx context.Context, subdirPath, subdirName string) {
	gitdirPath, deadline := "", time.Now().Add(gitdirAppearTimeout)
	for time.Now().Before(deadline) {
		gitdirPath = readWorktreeGitdir(subdirPath)
		if gitdirPath != "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if gitdirPath == "" {
		return
	}

	desc := descriptorForLinked(git.Worktree{
		Path:   gitdirPath,
		Branch: currentBranchFor(gitdirPath),
	})

	s.mu.Lock()
	s.subdirToRepoID[subdirName] = desc.ID
	already := s.watchers[desc.ID] != nil
	s.mu.Unlock()
	if already {
		return
	}
	s.spawnWatcher(ctx, desc)
}

func (s *supervisor) handleWorktreeRemove(subdirName string) {
	s.mu.Lock()
	repoID, ok := s.subdirToRepoID[subdirName]
	delete(s.subdirToRepoID, subdirName)
	s.mu.Unlock()
	if !ok {
		return
	}
	s.finishWatcher(repoID)
}

// readWorktreeGitdir reads `<subdirPath>/gitdir` — a one-line file written by
// `git worktree add` containing the absolute path to the worktree's `.git`
// file (which itself sits at `<worktree>/.git`). Returns the worktree's
// working-tree path (the parent of that `.git`), or "" if unreadable.
func readWorktreeGitdir(subdirPath string) string {
	raw, err := os.ReadFile(filepath.Join(subdirPath, "gitdir"))
	if err != nil {
		return ""
	}
	gitFilePath := strings.TrimSpace(string(raw))
	if gitFilePath == "" {
		return ""
	}
	return filepath.Dir(gitFilePath)
}
