package watch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/vcs/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanInitialRepos_PrimaryNoWorktrees(t *testing.T) {
	repo := initRepoWithCommit(t)

	repos, mode, err := planInitialRepos(repo)
	require.NoError(t, err)
	assert.Equal(t, modePrimary, mode)
	require.Len(t, repos, 1)
	assert.Equal(t, primaryRepoID, repos[0].ID)
	assert.True(t, repos[0].IsPrimary)
	assert.Equal(t, "main", repos[0].Label)
}

func TestPlanInitialRepos_PrimaryWithLinkedWorktree(t *testing.T) {
	repo := initRepoWithCommit(t)
	wt := filepath.Join(t.TempDir(), "linked")
	runGit(t, repo, "worktree", "add", "-b", "feat/x", wt)

	repos, mode, err := planInitialRepos(repo)
	require.NoError(t, err)
	assert.Equal(t, modePrimary, mode)
	require.Len(t, repos, 2)

	assert.Equal(t, primaryRepoID, repos[0].ID)
	assert.True(t, repos[0].IsPrimary)
	assert.Equal(t, "main", repos[0].Label)
	// The linked worktree comes after the primary, with a derived id.
	assert.True(t, repos[1].ID != primaryRepoID, "linked worktree should not get the primary id")
	assert.False(t, repos[1].IsPrimary)
	assert.Equal(t, "linked", repos[1].Label, "label should be the worktree directory name")
}

func TestPlanInitialRepos_LinkedModeReturnsOnlyCwd(t *testing.T) {
	repo := initRepoWithCommit(t)
	wt := filepath.Join(t.TempDir(), "linked")
	runGit(t, repo, "worktree", "add", "-b", "feat/x", wt)

	repos, mode, err := planInitialRepos(wt)
	require.NoError(t, err)
	assert.Equal(t, modeLinked, mode)
	require.Len(t, repos, 1)
	assert.Equal(t, primaryRepoID, repos[0].ID, "cwd-tree gets the 'primary' id regardless of git's notion")
	assert.True(t, repos[0].IsPrimary)
	assert.Equal(t, "feat/x", repos[0].Label)
}

func TestPlanInitialRepos_NonRepoErrors(t *testing.T) {
	_, _, err := planInitialRepos(t.TempDir())
	require.Error(t, err)
}

// TestSupervisor_DetectsLiveWorktreeAdd is the core test for the user-facing
// behavior: starting `clarity watch` in the primary tree should make a newly
// added linked worktree appear as a tab without restarting.
func TestSupervisor_DetectsLiveWorktreeAdd(t *testing.T) {
	repo := initRepoWithCommit(t)
	b := newBroker()
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	supervisorDone := make(chan struct{})
	go func() {
		_ = runSupervisor(ctx, repo, &watchOptions{}, b, formatter)
		close(supervisorDone)
	}()

	// Wait for the initial primary tab to register.
	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(b.repos) == 1
	}, 2*time.Second, 20*time.Millisecond, "primary tab should register on startup")

	// Add a worktree from outside the supervisor and expect it to appear as a tab.
	wt := filepath.Join(t.TempDir(), "live-added")
	runGit(t, repo, "worktree", "add", "-b", "feat/live", wt)

	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(b.repos) == 2
	}, 3*time.Second, 50*time.Millisecond, "supervisor should add a tab for the new worktree")

	b.mu.Lock()
	gotIDs := []string{b.repos[0].ID, b.repos[1].ID}
	linkedID := b.repos[1].ID
	bothActive := b.repos[0].Active && b.repos[1].Active
	b.mu.Unlock()
	assert.Contains(t, gotIDs, primaryRepoID)
	assert.NotEqual(t, primaryRepoID, gotIDs[1], "second tab should be the linked worktree, not another primary")
	assert.True(t, bothActive, "freshly watched worktrees start active")

	// Removing the worktree keeps the tab as a frozen, inactive record — the
	// user closes it explicitly. The tab must NOT vanish on its own.
	runGit(t, repo, "worktree", "remove", "--force", wt)
	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		if len(b.repos) != 2 {
			return false
		}
		idx, ok := b.repoIndex[linkedID]
		return ok && !b.repos[idx].Active
	}, 3*time.Second, 50*time.Millisecond, "removed worktree should remain as an inactive tab")

	cancel()
	select {
	case <-supervisorDone:
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor did not exit after cancel")
	}
}

// TestSupervisor_ReconcileDiscoversWorktreeWithoutFsnotify pins the reconcile
// backstop in isolation: with no meta-watcher running, a worktree added after
// the supervisor exists is invisible to fsnotify, so only reconcileWorktrees can
// discover it. This guards the recovery path for a dropped/stale fsnotify
// subscription (the failure that strands a long-running watch) and would fail if
// the reconcile scan were removed.
func TestSupervisor_ReconcileDiscoversWorktreeWithoutFsnotify(t *testing.T) {
	repo := initRepoWithCommit(t)
	b := newBroker()
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &supervisor{
		b:              b,
		opts:           &watchOptions{},
		formatter:      formatter,
		rootPath:       repo,
		watchers:       make(map[string]context.CancelFunc),
		subdirToRepoID: make(map[string]string),
	}

	// Added with no fsnotify meta-watcher in the picture, so only an explicit
	// reconcile can discover it.
	live := filepath.Join(t.TempDir(), "reconciled")
	runGit(t, repo, "worktree", "add", "-b", "feat/reconciled", live)

	countReconciled := func() int {
		b.mu.Lock()
		defer b.mu.Unlock()
		n := 0
		for _, r := range b.repos {
			if r.Label == "reconciled" && r.Active {
				n++
			}
		}
		return n
	}

	require.Zero(t, countReconciled(), "tab must not exist before a reconcile runs")

	s.reconcileWorktrees(ctx)
	require.Eventually(t, func() bool { return countReconciled() == 1 },
		2*time.Second, 20*time.Millisecond,
		"reconcile alone should register the worktree tab")

	// Idempotent: a second pass must not double-register the same worktree.
	s.reconcileWorktrees(ctx)
	require.Equal(t, 1, countReconciled(), "reconcile must be idempotent")
}

// TestSupervisor_SkipsStaleInitialWorktreeAndDetectsLaterAdds reproduces the
// failure mode where Git still lists a linked worktree after its directory has
// disappeared. A stale startup entry must not leave a dead active tab or break
// discovery of subsequently added worktrees.
func TestSupervisor_SkipsStaleInitialWorktreeAndDetectsLaterAdds(t *testing.T) {
	repo := initRepoWithCommit(t)
	stale := filepath.Join(t.TempDir(), "stale")
	runGit(t, repo, "worktree", "add", "-b", "feat/stale", stale)
	require.NoError(t, os.RemoveAll(stale))

	b := newBroker()
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	supervisorDone := make(chan struct{})
	go func() {
		_ = runSupervisor(ctx, repo, &watchOptions{}, b, formatter)
		close(supervisorDone)
	}()

	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(b.repos) > 0 && b.repos[0].ID == primaryRepoID
	}, 2*time.Second, 20*time.Millisecond, "primary tab should register on startup")

	live := filepath.Join(t.TempDir(), "live-after-stale")
	runGit(t, repo, "worktree", "add", "-b", "feat/live-after-stale", live)

	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		if len(b.repos) != 2 {
			return false
		}
		hasLive := false
		for _, repo := range b.repos {
			if repo.Label == "stale" {
				return false
			}
			if repo.Label == "live-after-stale" && repo.Active {
				hasLive = true
			}
		}
		return hasLive
	}, 3*time.Second, 50*time.Millisecond, "live worktree discovery should continue after a stale startup entry")

	cancel()
	select {
	case <-supervisorDone:
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor did not exit after cancel")
	}
}

// TestSupervisor_VanishedWorktreeSelfFinishes guards the teardown backstop: a
// watcher whose working directory disappears must stop polling git and flip its
// tab to inactive on its own, WITHOUT a meta-watcher REMOVE event. This is the
// case that breaks when fsnotify coalesces/drops events during a batch
// `git worktree remove`, leaving watchers polling deleted directories forever.
func TestSupervisor_VanishedWorktreeSelfFinishes(t *testing.T) {
	repo := initRepoWithCommit(t)
	wt := filepath.Join(t.TempDir(), "gone")
	runGit(t, repo, "worktree", "add", "-b", "feat/gone", wt)

	b := newBroker()
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)

	sup := &supervisor{
		b:              b,
		opts:           &watchOptions{},
		formatter:      formatter,
		watchers:       make(map[string]context.CancelFunc),
		subdirToRepoID: make(map[string]string),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spawn the watcher directly, bypassing the meta-watcher entirely.
	desc := descriptorForLinked(git.Worktree{Path: wt, Branch: "feat/gone"})
	sup.spawnWatcher(ctx, desc)

	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		idx, ok := b.repoIndex[desc.ID]
		return ok && b.repos[idx].Active
	}, 2*time.Second, 20*time.Millisecond, "worktree tab should register active")

	// Delete the working tree out from under the watcher without telling the
	// supervisor — simulating a dropped REMOVE event.
	require.NoError(t, os.RemoveAll(wt))

	require.Eventually(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		idx, ok := b.repoIndex[desc.ID]
		return ok && !b.repos[idx].Active
	}, 5*time.Second, 100*time.Millisecond, "watcher should self-finish when its worktree vanishes")
}

// initRepoWithCommit creates a fresh git repo with one commit so worktree-add
// can succeed, then returns its absolute path.
func initRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd %v failed: %s", args, out)
	}
	return dir
}
