package watch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/cmd/watch/protocol"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
	"github.com/fsnotify/fsnotify"
)

type repoMode string

const (
	modePrimary repoMode = "primary"
	modeLinked  repoMode = "linked"
)

// worktreeReconcileInterval backs up fsnotify delivery with a lightweight
// registry scan so stale worktree entries or dropped events do not strand the
// watch process. fsnotify is the fast path (instant), so this is only a
// backstop for missed events (e.g. a kqueue subscription dropped across
// sleep/wake) and runs at a relaxed cadence -- worktree adds/removes are rare.
const worktreeReconcileInterval = 2 * time.Second

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
		if !pathExists(w.Path) {
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
		b:         b,
		opts:      opts,
		formatter: formatter,
		rootPath:  repos[0].Path,
		watchers:  make(map[string]context.CancelFunc),
	}

	// In primary mode, install the meta-watcher BEFORE spawning initial
	// watchers so a `git worktree add` racing with startup is never missed.
	var metaDone <-chan struct{}
	if mode == modePrimary {
		// Meta-watching is best-effort; if the common dir can't be resolved or the
		// watcher fails, fall back to running without it.
		if commonDir, err := git.GetCommonDir(cwd); err == nil {
			sup.commonDir = commonDir
			ready := make(chan struct{})
			done := make(chan struct{})
			go func() {
				_ = sup.runMetaWatcher(ctx, ready)
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
	rootPath  string

	mu       sync.Mutex
	watchers map[string]context.CancelFunc // repoID -> cancel
}

func (s *supervisor) spawnWatcher(parent context.Context, desc protocol.RepoDescriptor) {
	if !pathExists(desc.Path) {
		return
	}
	s.b.registerRepo(desc)
	wctx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	s.watchers[desc.ID] = cancel
	s.mu.Unlock()
	go func() {
		if err := watchAndRebuild(wctx, desc.ID, desc.Path, s.opts, s.b, s.formatter); err != nil {
			if !pathExists(desc.Path) {
				s.finishWatcher(desc.ID)
				return
			}
			fmt.Fprintf(os.Stderr, "watcher %s exited: %v\n", desc.ID, err)
		}
	}()
	// Seed an initial graph so the tab has content immediately.
	if !pathExists(desc.Path) {
		s.finishWatcher(desc.ID)
		return
	}
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

// runMetaWatcher installs an fsnotify watch on `<common>/worktrees/`. Any event
// there — `git worktree add`, `remove`, or `move` — is treated as a bare trigger
// to reconcile: the watch carries no per-event logic, since `git worktree list`
// (via reconcileWorktrees) is the single source of truth for the tab set. A
// periodic tick backstops dropped fsnotify events.
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

	// The watch is installed; let startup proceed.
	closeReady()
	reconcileTicker := time.NewTicker(worktreeReconcileInterval)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			s.reconcileWorktrees(ctx)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "meta-watcher error: %v\n", err)
		case <-reconcileTicker.C:
			s.reconcileWorktrees(ctx)
		}
	}
}

func (s *supervisor) reconcileWorktrees(ctx context.Context) {
	worktrees, err := git.ListWorktrees(s.rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "worktree reconciliation error: %v\n", err)
		return
	}

	seen := make(map[string]bool)
	for _, w := range worktrees {
		if w.IsPrimary || !pathExists(w.Path) {
			continue
		}
		desc := descriptorForLinked(w)
		seen[desc.ID] = true
		if !s.hasWatcher(desc.ID) {
			s.spawnWatcher(ctx, desc)
		}
	}

	for _, repoID := range s.linkedWatchersMissingFrom(seen) {
		s.finishWatcher(repoID)
	}
}

func (s *supervisor) hasWatcher(repoID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watchers[repoID] != nil
}

func (s *supervisor) linkedWatchersMissingFrom(seen map[string]bool) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var missing []string
	for repoID := range s.watchers {
		if repoID != primaryRepoID && !seen[repoID] {
			missing = append(missing, repoID)
		}
	}
	return missing
}
