package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/cmd/watch/protocol"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroker_PublishAndSubscribe(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.publish("digraph { A -> B; }")

	select {
	case got := <-ch:
		require.Len(t, got.WorkingSnapshots, 1)
		assert.Equal(t, "digraph { A -> B; }", got.WorkingSnapshots[0].DOT)
		assert.Equal(t, got.WorkingSnapshots[0].ID, got.LatestWorkingID)
		assert.Empty(t, got.PastCollections)
		assert.Zero(t, got.LatestPastCollectionID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestBroker_NewSubscriberReceivesLatest(t *testing.T) {
	b := newBroker()
	b.publish("digraph { X -> Y; }")

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	select {
	case got := <-ch:
		require.Len(t, got.WorkingSnapshots, 1)
		assert.Equal(t, "digraph { X -> Y; }", got.WorkingSnapshots[0].DOT)
		assert.Equal(t, got.WorkingSnapshots[0].ID, got.LatestWorkingID)
		assert.Empty(t, got.PastCollections)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for latest graph")
	}
}

func TestBroker_MultipleSubscribers(t *testing.T) {
	b := newBroker()
	ch1 := b.subscribe()
	ch2 := b.subscribe()
	defer b.unsubscribe(ch1)
	defer b.unsubscribe(ch2)

	b.publish("digraph { A; }")

	select {
	case got := <-ch1:
		require.Len(t, got.WorkingSnapshots, 1)
		assert.Equal(t, "digraph { A; }", got.WorkingSnapshots[0].DOT)
	case <-time.After(time.Second):
		t.Fatal("ch1: timed out")
	}

	select {
	case got := <-ch2:
		require.Len(t, got.WorkingSnapshots, 1)
		assert.Equal(t, "digraph { A; }", got.WorkingSnapshots[0].DOT)
	case <-time.After(time.Second):
		t.Fatal("ch2: timed out")
	}
}

func TestHandleIndex_ServesHTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handleIndex("clarity • clarity watch")(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "clarity • clarity watch")
	assert.Contains(t, w.Body.String(), `<div id="app"></div>`)
	assert.Contains(t, w.Body.String(), `/assets/index-`)
}

func TestBuildWatchPageTitle(t *testing.T) {
	assert.Equal(t, "clarity • clarity watch", buildWatchPageTitle("/tmp/clarity"))
	assert.Equal(t, "clarity watch", buildWatchPageTitle("/"))
	assert.Equal(t, "clarity watch", buildWatchPageTitle(""))
}

func TestHandleSSE_StreamsGraphEvent(t *testing.T) {
	b := newBroker()

	// Pre-publish so the subscriber gets data immediately on subscribe.
	b.publish("digraph { test; }")

	handler := handleSSE(b)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	assert.Contains(t, body, "event: graph")
	assert.Contains(t, body, "\"dot\":\"digraph { test; }\"")
}

func TestHandleSSE_MultiLineData(t *testing.T) {
	b := newBroker()

	multiLine := "digraph {\n  A -> B;\n}"
	b.publish(multiLine)

	handler := handleSSE(b)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	assert.Contains(t, body, "event: graph")

	var payload protocol.GraphStreamPayload
	require.NoError(t, decodeSSEPayload(body, &payload))
	require.Len(t, payload.WorkingSnapshots, 1)
	assert.Equal(t, multiLine, payload.WorkingSnapshots[0].DOT)
}

func TestBroker_PublishSkipsDuplicateSnapshots(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.publish("digraph { A -> B; }")
	<-ch

	b.publish("digraph { A -> B; }")

	select {
	case <-ch:
		t.Fatal("unexpected duplicate snapshot publish")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBroker_NewPayloadOverwritesQueuedStalePayload(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	// Queue a stale reset payload and do not consume it yet.
	b.clearWorkingSet()

	// Publish a fresh working snapshot while the channel buffer is full.
	b.publish("digraph { A -> B; }")

	select {
	case got := <-ch:
		require.Len(t, got.WorkingSnapshots, 1)
		assert.Equal(t, "digraph { A -> B; }", got.WorkingSnapshots[0].DOT)
		assert.Equal(t, got.WorkingSnapshots[0].ID, got.LatestWorkingID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for latest payload")
	}
}

func TestBroker_ArchiveWorkingSetClearsActiveSnapshots(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.publish("digraph { A; }")
	<-ch

	b.archiveWorkingSet()

	select {
	case got := <-ch:
		assert.Empty(t, got.WorkingSnapshots)
		require.Len(t, got.PastCollections, 1)
		require.Len(t, got.PastCollections[0].Snapshots, 1)
		assert.Equal(t, "digraph { A; }", got.PastCollections[0].Snapshots[0].DOT)
		assert.Zero(t, got.LatestWorkingID)
		assert.Equal(t, got.PastCollections[0].ID, got.LatestPastCollectionID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for archive payload")
	}
}

func TestBroker_NewSubscriberReceivesArchivedState(t *testing.T) {
	b := newBroker()
	b.publish("digraph { A; }")
	b.archiveWorkingSet()

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	select {
	case got := <-ch:
		assert.Empty(t, got.WorkingSnapshots)
		require.Len(t, got.PastCollections, 1)
		require.Len(t, got.PastCollections[0].Snapshots, 1)
		assert.Equal(t, "digraph { A; }", got.PastCollections[0].Snapshots[0].DOT)
		assert.Zero(t, got.LatestWorkingID)
		assert.Equal(t, got.PastCollections[0].ID, got.LatestPastCollectionID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for archived payload")
	}
}

func TestBroker_ArchiveWorkingSetAcrossCycles(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.publish("digraph { A; }")
	<-ch
	b.archiveWorkingSet()
	<-ch

	b.publish("digraph { B; }")
	<-ch
	b.archiveWorkingSet()

	select {
	case got := <-ch:
		assert.Empty(t, got.WorkingSnapshots)
		require.Len(t, got.PastCollections, 2)
		require.Len(t, got.PastCollections[0].Snapshots, 1)
		require.Len(t, got.PastCollections[1].Snapshots, 1)
		assert.Equal(t, "digraph { A; }", got.PastCollections[0].Snapshots[0].DOT)
		assert.Equal(t, "digraph { B; }", got.PastCollections[1].Snapshots[0].DOT)
		assert.Equal(t, got.PastCollections[1].ID, got.LatestPastCollectionID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second archived payload")
	}
}

func TestBroker_ClearWorkingSetDoesNotArchive(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.publish("digraph { A; }")
	<-ch

	b.clearWorkingSet()

	select {
	case got := <-ch:
		assert.Empty(t, got.WorkingSnapshots)
		assert.Empty(t, got.PastCollections)
		assert.Zero(t, got.LatestWorkingID)
		assert.Zero(t, got.LatestPastCollectionID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clear payload")
	}
}

func TestHandleSSE_StreamsJSONPayload(t *testing.T) {
	b := newBroker()
	b.publish("digraph { A; }")
	b.publish("digraph { B; }")

	handler := handleSSE(b)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	var payload protocol.GraphStreamPayload
	require.NoError(t, decodeSSEPayload(body, &payload))
	require.Len(t, payload.WorkingSnapshots, 2)
	assert.Equal(t, "digraph { A; }", payload.WorkingSnapshots[0].DOT)
	assert.Equal(t, "digraph { B; }", payload.WorkingSnapshots[1].DOT)
	assert.Equal(t, payload.WorkingSnapshots[1].ID, payload.LatestWorkingID)
	assert.Empty(t, payload.PastCollections)
	assert.Zero(t, payload.LatestPastCollectionID)
}

func decodeSSEPayload(body string, target any) error {
	dataLine := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	if dataLine == "" {
		return fmt.Errorf("missing data line in SSE body")
	}
	return json.Unmarshal([]byte(dataLine), target)
}

func TestIsRelevantChange_SupportedExtension(t *testing.T) {
	goEvent := fsnotify.Event{Name: "main.go", Op: fsnotify.Write}
	assert.True(t, isRelevantChange(goEvent))

	tsEvent := fsnotify.Event{Name: "app.ts", Op: fsnotify.Create}
	assert.True(t, isRelevantChange(tsEvent))

	pyEvent := fsnotify.Event{Name: "script.py", Op: fsnotify.Remove}
	assert.True(t, isRelevantChange(pyEvent))
}

func TestIsRelevantChange_UnsupportedExtension(t *testing.T) {
	txtEvent := fsnotify.Event{Name: "README.txt", Op: fsnotify.Write}
	assert.False(t, isRelevantChange(txtEvent))

	binEvent := fsnotify.Event{Name: "image.png", Op: fsnotify.Write}
	assert.False(t, isRelevantChange(binEvent))
}

func TestIsRelevantChange_ChmodIgnored(t *testing.T) {
	chmodEvent := fsnotify.Event{Name: "main.go", Op: fsnotify.Chmod}
	assert.False(t, isRelevantChange(chmodEvent))
}

// initGitRepo creates a git repo in dir with an initial commit, then returns dir.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
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
}

func TestBuildDOTGraph_ProducesOutput(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	require.NoError(t, err)

	opts := &watchOptions{}
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := buildDOTGraph(dir, opts, formatter)
	require.NoError(t, err)

	assert.Contains(t, dot, "digraph")
	assert.Contains(t, dot, "main.go")
}

func TestBuildDOTGraph_RustPhantomTestOnly(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	const initial = `pub fn add(a: i32, b: i32) -> i32 { a + b }

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn it_adds() { assert_eq!(add(1, 2), 3); }
}
`
	const modified = `pub fn add(a: i32, b: i32) -> i32 { a + b }

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn it_adds() { assert_eq!(add(1, 2), 3); }
    #[test]
    fn it_adds_zero() { assert_eq!(add(0, 0), 0); }
}
`
	libPath := filepath.Join(dir, "lib.rs")
	require.NoError(t, os.WriteFile(libPath, []byte(initial), 0o644))
	requireCmd(t, dir, "git", "add", "lib.rs")
	requireCmd(t, dir, "git", "commit", "-m", "initial")
	require.NoError(t, os.WriteFile(libPath, []byte(modified), 0o644))

	opts := &watchOptions{}
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := buildDOTGraph(dir, opts, formatter)
	require.NoError(t, err)

	assert.Contains(t, dot, "::tests", "phantom node must appear when test region changed")
	assert.Contains(t, dot, "fillcolor=lightgreen", "phantom node is green")
}

func requireCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "cmd %v failed: %s", args, out)
}

func TestBuildDOTGraph_NoUncommittedChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	opts := &watchOptions{}
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	_, err = buildDOTGraph(dir, opts, formatter)
	assert.True(t, errors.Is(err, errNoUncommittedChanges))
}

func TestBuildDOTGraph_WithIncludeExt(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "app.py"), []byte("print('hi')\n"), 0o644)
	require.NoError(t, err)

	opts := &watchOptions{includeExt: ".go"}
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := buildDOTGraph(dir, opts, formatter)
	require.NoError(t, err)

	assert.Contains(t, dot, "main.go")
	assert.NotContains(t, dot, "app.py")
}

func TestBuildDOTGraph_WithExcludeExt(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "app.py"), []byte("print('hi')\n"), 0o644)
	require.NoError(t, err)

	opts := &watchOptions{excludeExt: ".py"}
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := buildDOTGraph(dir, opts, formatter)
	require.NoError(t, err)

	assert.Contains(t, dot, "main.go")
	assert.NotContains(t, dot, "app.py")
}

func TestParseExtensions(t *testing.T) {
	exts := parseExtensions(".go,.py,.ts")
	assert.True(t, exts[".go"])
	assert.True(t, exts[".py"])
	assert.True(t, exts[".ts"])
	assert.False(t, exts[".rs"])
}

func TestParseExtensions_WithoutDots(t *testing.T) {
	exts := parseExtensions("go,py")
	assert.True(t, exts[".go"])
	assert.True(t, exts[".py"])
}

func TestParseExtensions_CaseInsensitive(t *testing.T) {
	exts := parseExtensions(".GO,.Py")
	assert.True(t, exts[".go"])
	assert.True(t, exts[".py"])
}

func TestExtractHEADSignature(t *testing.T) {
	assert.Equal(t, "abc123", extractHEADSignature("abc123\nM main.go"))
	assert.Equal(t, "abc123", extractHEADSignature("abc123"))
	assert.Equal(t, "", extractHEADSignature(""))
}

func TestNewCommand_DefaultPort(t *testing.T) {
	cmd := NewCommand()
	port, err := cmd.Flags().GetInt("port")
	require.NoError(t, err)
	assert.Equal(t, 4900, port)
}

func TestBuildDOTGraph_IncludesFileStats(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	require.NoError(t, err)

	opts := &watchOptions{}
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	dot, err := buildDOTGraph(dir, opts, formatter)
	require.NoError(t, err)

	assert.Contains(t, dot, "main.go")
}

func TestPublishCurrentGraph_NoUncommittedChangesClearsWorkingSnapshots(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	publishCurrentGraph(dir, &watchOptions{}, b, formatter)

	select {
	case got := <-ch:
		assert.Empty(t, got.WorkingSnapshots)
		assert.Empty(t, got.PastCollections)
		assert.Zero(t, got.LatestWorkingID)
		assert.Zero(t, got.LatestPastCollectionID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clear publish")
	}
}

func TestListenWithPortFallback_PicksNextAvailablePort(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer occupied.Close()

	occupiedPort := occupied.Addr().(*net.TCPAddr).Port
	reservedNext, err := net.Listen("tcp", fmt.Sprintf(":%d", occupiedPort+1))
	require.NoError(t, err)
	defer reservedNext.Close()

	ln, actualPort, err := listenWithPortFallback(occupiedPort)
	require.NoError(t, err)
	defer ln.Close()

	assert.Equal(t, occupiedPort+2, actualPort)
}

// TestWatchAndRebuild_DetectsFileRename reproduces the user-observed bug where
// renaming a file in the working tree leaves clarity watch showing the
// pre-rename graph until the watcher process is restarted.
func TestWatchAndRebuild_DetectsFileRename(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Seed a committed importer + importee so the importer (consumer.ts) is the
	// stable "anchor" file whose dependencies we will mutate.
	consumerPath := filepath.Join(dir, "consumer.ts")
	originalPath := filepath.Join(dir, "chart.ts")
	require.NoError(t, os.WriteFile(consumerPath, []byte("import './chart';\n"), 0o644))
	require.NoError(t, os.WriteFile(originalPath, []byte("export const x = 1;\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "seed")

	b := newBroker()
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	opts := &watchOptions{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = watchAndRebuild(ctx, dir, opts, b, formatter) }()

	// Give the watcher a moment to install its fsnotify watches before we
	// mutate the tree. Then create the first uncommitted change so the watcher
	// publishes a baseline graph.
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(consumerPath, []byte("import './chart';\n// edit\n"), 0o644))

	// Wait for the watcher's first publish.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if snap, ok := latestSnapshot(b); ok && strings.Contains(snap, "consumer.ts") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	snap, _ := latestSnapshot(b)
	require.Contains(t, snap, "consumer.ts", "watcher never published initial graph")

	// Rename the importee. Update the importer to point at the new name so the
	// edge stays valid. Both ops mirror what `git mv` + a re-import would do
	// in real editor flow.
	renamedPath := filepath.Join(dir, "ScatterPlot.ts")
	require.NoError(t, os.Rename(originalPath, renamedPath))
	require.NoError(t, os.WriteFile(consumerPath, []byte("import './ScatterPlot';\n"), 0o644))

	// Wait long enough for fsnotify debounce + git state poll to fire.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if snap, ok := latestSnapshot(b); ok && strings.Contains(snap, "ScatterPlot.ts") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	snap, ok := latestSnapshot(b)
	require.True(t, ok, "no graph published after rename")
	assert.Contains(t, snap, "ScatterPlot.ts", "post-rename graph missing new file name")
	assert.NotContains(t, snap, "chart.ts", "post-rename graph still references old file name")
}

// TestWatchAndRebuild_DebounceFiresOnEverySaveCycle reproduces a watcher bug
// where the debounce path only triggered a rebuild for the first event burst
// after startup. Subsequent edits to an already-dirty file (the common
// editor-save pattern) silently dropped because `debounceC` was set to nil
// after the first fire and never re-armed to the timer's channel.
func TestWatchAndRebuild_DebounceFiresOnEverySaveCycle(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	target := filepath.Join(dir, "app.ts")
	require.NoError(t, os.WriteFile(target, []byte("export const v = 0;\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "seed")

	b := newBroker()
	formatter, err := formatters.NewFormatter("dot")
	require.NoError(t, err)
	opts := &watchOptions{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = watchAndRebuild(ctx, dir, opts, b, formatter) }()

	time.Sleep(100 * time.Millisecond)

	// First edit makes the file dirty and changes porcelain — both the
	// debounce arm and the git poller will trigger a rebuild here.
	require.NoError(t, os.WriteFile(target, []byte("export const v = 1;\n"), 0o644))
	waitForSnapshotID(t, b, 1, 2*time.Second)

	// Second edit grows the file. Porcelain stays `" M app.ts"`, so the
	// git poller sees state_changed=false — but the diff stats change, so
	// the rendered DOT differs and a republish would NOT be deduped. The
	// ONLY path that can republish here is the fsnotify debounce arm.
	require.NoError(t, os.WriteFile(target, []byte("export const v = 2;\nexport const w = 3;\n"), 0o644))
	waitForSnapshotID(t, b, 2, 2*time.Second)
}

func waitForSnapshotID(t *testing.T, b *broker, want int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		id := b.nextID
		b.mu.Unlock()
		if id >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	b.mu.Lock()
	got := b.nextID
	b.mu.Unlock()
	t.Fatalf("broker never reached snapshot_id %d within %s (got %d)", want, timeout, got)
}

func latestSnapshot(b *broker) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.history) == 0 {
		return "", false
	}
	return b.history[len(b.history)-1].DOT, true
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
}
