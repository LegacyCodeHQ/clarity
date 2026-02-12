package watch

import (
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
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, "digraph { A -> B; }", got.Snapshots[0].DOT)
		assert.Equal(t, got.Snapshots[0].ID, got.LatestID)
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
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, "digraph { X -> Y; }", got.Snapshots[0].DOT)
		assert.Equal(t, got.Snapshots[0].ID, got.LatestID)
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
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, "digraph { A; }", got.Snapshots[0].DOT)
	case <-time.After(time.Second):
		t.Fatal("ch1: timed out")
	}

	select {
	case got := <-ch2:
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, "digraph { A; }", got.Snapshots[0].DOT)
	case <-time.After(time.Second):
		t.Fatal("ch2: timed out")
	}
}

func TestHandleIndex_ServesHTML(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handleIndex(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "clarity watch")
	assert.Contains(t, w.Body.String(), "EventSource")
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

	var payload graphStreamPayload
	require.NoError(t, decodeSSEPayload(body, &payload))
	require.Len(t, payload.Snapshots, 1)
	assert.Equal(t, multiLine, payload.Snapshots[0].DOT)
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

	var payload graphStreamPayload
	require.NoError(t, decodeSSEPayload(body, &payload))
	require.Len(t, payload.Snapshots, 2)
	assert.Equal(t, "digraph { A; }", payload.Snapshots[0].DOT)
	assert.Equal(t, "digraph { B; }", payload.Snapshots[1].DOT)
	assert.Equal(t, payload.Snapshots[1].ID, payload.LatestID)
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

	mdEvent := fsnotify.Event{Name: "docs.md", Op: fsnotify.Write}
	assert.False(t, isRelevantChange(mdEvent))
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
	dot, err := buildDOTGraph(dir, opts)
	require.NoError(t, err)

	assert.Contains(t, dot, "digraph")
	assert.Contains(t, dot, "main.go")
}

func TestBuildDOTGraph_NoUncommittedChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	opts := &watchOptions{}
	_, err := buildDOTGraph(dir, opts)
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
	dot, err := buildDOTGraph(dir, opts)
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
	dot, err := buildDOTGraph(dir, opts)
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
	dot, err := buildDOTGraph(dir, opts)
	require.NoError(t, err)

	assert.Contains(t, dot, "main.go")
}

func TestPublishCurrentGraph_NoUncommittedChangesPublishesEmptyGraph(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	publishCurrentGraph(dir, &watchOptions{}, b)

	select {
	case got := <-ch:
		require.Len(t, got.Snapshots, 1)
		assert.Equal(t, emptyDOTGraph, got.Snapshots[0].DOT)
		assert.Equal(t, got.Snapshots[0].ID, got.LatestID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for empty graph publish")
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
