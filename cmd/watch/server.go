package watch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LegacyCodeHQ/clarity/cmd/watch/protocol"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

const maxSnapshots = 250

const watchPageTitleSuffix = "clarity watch"

// repoState holds the per-worktree snapshot history and archived cycles.
type repoState struct {
	history        []protocol.GraphSnapshot
	archivedCycles []protocol.SnapshotCollection
	hasState       bool
	// sessionStarted records whether this repo's first snapshot (the
	// watcher-attach state) has already been emitted. Set once and never reset,
	// so the session-start marker doesn't reappear after commit/archive cycles.
	sessionStarted bool
}

// broker manages SSE client connections and broadcasts graph snapshots.
// It maintains independent snapshot history per registered worktree (`repoID`)
// while aggregating them into a single flat SSE payload.
type broker struct {
	mu          sync.Mutex
	clients     map[chan protocol.GraphStreamPayload]struct{}
	repos       []protocol.RepoDescriptor
	repoIndex   map[string]int
	repoStates  map[string]*repoState
	nextID      int64
	nextCycleID int64
}

func newBroker() *broker {
	return &broker{
		clients:    make(map[chan protocol.GraphStreamPayload]struct{}),
		repoIndex:  make(map[string]int),
		repoStates: make(map[string]*repoState),
	}
}

func (b *broker) subscribe() chan protocol.GraphStreamPayload {
	ch := make(chan protocol.GraphStreamPayload, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	payload, ok := b.currentPayloadLocked()
	if ok {
		ch <- payload
	}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan protocol.GraphStreamPayload) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}

// registerRepo adds a worktree to the broker's tab set. If `desc.ID` already
// exists, the descriptor is updated in place (path/label/isPrimary may change
// on git operations like `worktree move`).
func (b *broker) registerRepo(desc protocol.RepoDescriptor) {
	b.mu.Lock()
	if idx, ok := b.repoIndex[desc.ID]; ok {
		b.repos[idx] = desc
	} else {
		b.repoIndex[desc.ID] = len(b.repos)
		b.repos = append(b.repos, desc)
		b.repoStates[desc.ID] = &repoState{}
	}
	b.broadcastLocked()
	b.mu.Unlock()
}

// unregisterRepo removes a worktree and its snapshot history outright. Used on
// shutdown; the live `git worktree remove` path goes through markRepoFinished
// instead so the tab lingers as a closable record.
func (b *broker) unregisterRepo(repoID string) {
	b.mu.Lock()
	if idx, ok := b.repoIndex[repoID]; ok {
		b.unregisterLocked(idx, repoID)
		b.broadcastLocked()
	}
	b.mu.Unlock()
}

// unregisterLocked drops a repo from the tab set and deletes its state. The
// caller must hold b.mu and is responsible for broadcasting.
func (b *broker) unregisterLocked(idx int, repoID string) {
	b.repos = append(b.repos[:idx], b.repos[idx+1:]...)
	delete(b.repoIndex, repoID)
	delete(b.repoStates, repoID)
	for id, i := range b.repoIndex {
		if i > idx {
			b.repoIndex[id] = i - 1
		}
	}
}

// markRepoFinished flips a worktree to inactive when its git working tree is
// removed. The tab and its snapshot history are KEPT so the user can still
// browse the frozen final state; the UI surfaces a close affordance and the
// teardown completes via closeRepo.
func (b *broker) markRepoFinished(repoID string) {
	b.mu.Lock()
	if idx, ok := b.repoIndex[repoID]; ok && b.repos[idx].Active {
		s := b.stateForLocked(repoID)
		b.archiveWorkingSetLocked(repoID, s, nil)
		b.repos[idx].Active = false
		b.broadcastLocked()
	}
	b.mu.Unlock()
}

// closeOutcome reports how a closeRepo request resolved, letting the HTTP layer
// map it to a status code without leaking transport concerns into the broker.
type closeOutcome int

const (
	closeOK closeOutcome = iota
	closeNotFound
	closeActive
)

// closeRepo tears down a finished tab at the user's request. Active worktrees
// are pinned: they cannot be closed while still being watched.
func (b *broker) closeRepo(repoID string) closeOutcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	idx, ok := b.repoIndex[repoID]
	if !ok {
		return closeNotFound
	}
	if b.repos[idx].Active {
		return closeActive
	}
	b.unregisterLocked(idx, repoID)
	b.broadcastLocked()
	return closeOK
}

// stateForLocked returns the per-repo state, creating it for an unregistered
// repoID. This keeps tests and single-repo callers ergonomic; the supervisor
// will registerRepo first in real flows.
func (b *broker) stateForLocked(repoID string) *repoState {
	if s, ok := b.repoStates[repoID]; ok {
		return s
	}
	s := &repoState{}
	b.repoStates[repoID] = s
	return s
}

func (b *broker) publish(repoID, dot string) {
	b.mu.Lock()
	s := b.stateForLocked(repoID)
	if len(s.history) > 0 && s.history[len(s.history)-1].DOT == dot {
		b.mu.Unlock()
		return
	}

	b.nextID++
	sessionStart := !s.sessionStarted
	s.sessionStarted = true
	s.history = append(s.history, protocol.GraphSnapshot{
		ID:           b.nextID,
		RepoID:       repoID,
		Timestamp:    time.Now().UTC(),
		DOT:          dot,
		SessionStart: sessionStart,
	})
	if len(s.history) > maxSnapshots {
		s.history = s.history[len(s.history)-maxSnapshots:]
	}
	s.hasState = true

	b.broadcastLocked()
	b.mu.Unlock()
}

func (b *broker) archiveWorkingSet(repoID string) {
	b.mu.Lock()
	s := b.stateForLocked(repoID)
	b.archiveWorkingSetLocked(repoID, s, nil)
	b.broadcastLocked()
	b.mu.Unlock()
}

func (b *broker) archiveWorkingSetWithCommitHistory(repoID string, commitHistory []vcs.CommitSummary) {
	b.mu.Lock()
	s := b.stateForLocked(repoID)
	b.archiveWorkingSetLocked(repoID, s, commitHistory)
	b.broadcastLocked()
	b.mu.Unlock()
}

func (b *broker) archiveWorkingSetLocked(repoID string, s *repoState, commitHistory []vcs.CommitSummary) {
	if len(s.history) > 0 {
		archivedSnapshots := make([]protocol.GraphSnapshot, len(s.history))
		copy(archivedSnapshots, s.history)
		b.nextCycleID++
		s.archivedCycles = append(s.archivedCycles, protocol.SnapshotCollection{
			ID:            b.nextCycleID,
			RepoID:        repoID,
			Timestamp:     time.Now().UTC(),
			Snapshots:     archivedSnapshots,
			CommitHistory: toProtocolCommitHistory(commitHistory),
		})
	}

	s.history = nil
	s.hasState = true
}

func (b *broker) clearWorkingSet(repoID string) {
	b.mu.Lock()
	s := b.stateForLocked(repoID)
	if len(s.history) == 0 && s.hasState {
		b.mu.Unlock()
		return
	}

	s.history = nil
	s.hasState = true
	b.broadcastLocked()
	b.mu.Unlock()
}

func (b *broker) broadcastLocked() {
	payload, ok := b.currentPayloadLocked()
	if !ok {
		return
	}
	for ch := range b.clients {
		pushLatestPayload(ch, payload)
	}
}

func (b *broker) currentPayloadLocked() (protocol.GraphStreamPayload, bool) {
	anyHasState := false
	for _, s := range b.repoStates {
		if s.hasState {
			anyHasState = true
			break
		}
	}
	if !anyHasState && len(b.repos) == 0 {
		return protocol.GraphStreamPayload{}, false
	}

	repos := make([]protocol.RepoDescriptor, len(b.repos))
	copy(repos, b.repos)

	working := b.collectWorkingLocked()
	past := b.collectPastLocked()

	var latestWorkingID int64
	for _, snap := range working {
		if snap.ID > latestWorkingID {
			latestWorkingID = snap.ID
		}
	}
	var latestPastID int64
	for _, coll := range past {
		if coll.ID > latestPastID {
			latestPastID = coll.ID
		}
	}

	return protocol.GraphStreamPayload{
		Repos:                  repos,
		WorkingSnapshots:       working,
		PastCollections:        past,
		LatestWorkingID:        latestWorkingID,
		LatestPastCollectionID: latestPastID,
	}, true
}

// collectWorkingLocked returns a flat, deterministic list of working snapshots
// across every repo: outer order follows repo registration order; inner order
// preserves per-repo history order.
func (b *broker) collectWorkingLocked() []protocol.GraphSnapshot {
	repoIDs := b.orderedRepoIDsLocked()
	working := []protocol.GraphSnapshot{}
	for _, id := range repoIDs {
		s := b.repoStates[id]
		if s == nil {
			continue
		}
		working = append(working, s.history...)
	}
	return working
}

func (b *broker) collectPastLocked() []protocol.SnapshotCollection {
	repoIDs := b.orderedRepoIDsLocked()
	past := []protocol.SnapshotCollection{}
	for _, id := range repoIDs {
		s := b.repoStates[id]
		if s == nil {
			continue
		}
		for _, cycle := range s.archivedCycles {
			snapshots := make([]protocol.GraphSnapshot, len(cycle.Snapshots))
			copy(snapshots, cycle.Snapshots)
			past = append(past, protocol.SnapshotCollection{
				ID:            cycle.ID,
				RepoID:        cycle.RepoID,
				Timestamp:     cycle.Timestamp,
				Snapshots:     snapshots,
				CommitHistory: copyCommitHistory(cycle.CommitHistory),
			})
		}
	}
	return past
}

func toProtocolCommitHistory(commits []vcs.CommitSummary) []protocol.CommitSummary {
	if len(commits) == 0 {
		return nil
	}
	history := make([]protocol.CommitSummary, 0, len(commits))
	for _, commit := range commits {
		history = append(history, protocol.CommitSummary{
			Hash:      commit.Hash,
			ShortHash: commit.ShortHash,
			Subject:   commit.Subject,
			Author:    commit.Author,
			Email:     commit.Email,
			Timestamp: commit.Timestamp,
		})
	}
	return history
}

func copyCommitHistory(commits []protocol.CommitSummary) []protocol.CommitSummary {
	if len(commits) == 0 {
		return nil
	}
	copied := make([]protocol.CommitSummary, len(commits))
	copy(copied, commits)
	return copied
}

// orderedRepoIDsLocked returns repo IDs in registration order, then any
// orphan repo states (registered via publish without registerRepo) sorted
// alphabetically for determinism in tests.
func (b *broker) orderedRepoIDsLocked() []string {
	ids := make([]string, 0, len(b.repoStates))
	seen := make(map[string]bool, len(b.repos))
	for _, r := range b.repos {
		ids = append(ids, r.ID)
		seen[r.ID] = true
	}
	var orphans []string
	for id := range b.repoStates {
		if !seen[id] {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	return append(ids, orphans...)
}

func pushLatestPayload(ch chan protocol.GraphStreamPayload, payload protocol.GraphStreamPayload) {
	select {
	case ch <- payload:
		return
	default:
	}

	select {
	case <-ch:
	default:
	}

	select {
	case ch <- payload:
	default:
	}
}

func newServer(b *broker, port int, repoPath string) *http.Server {
	mux := http.NewServeMux()

	// Serve index.html with page title injection
	mux.HandleFunc(protocol.RouteIndex, handleIndex(buildWatchPageTitle(repoPath)))

	// Serve all static assets from embedded dist directory
	distFS, err := getDistFS()
	if err != nil {
		panic(fmt.Sprintf("failed to get dist FS: %v", err))
	}
	mux.Handle("/assets/", http.FileServer(http.FS(distFS)))

	// Serve SSE endpoint (unchanged)
	mux.HandleFunc(protocol.RouteEvents, handleSSE(b))

	// Client→server: close a finished worktree tab.
	mux.HandleFunc(protocol.RouteCloseRepo, handleCloseRepo(b))

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}

func buildWatchPageTitle(repoPath string) string {
	repoName := strings.TrimSpace(filepath.Base(filepath.Clean(repoPath)))
	if repoName == "" || repoName == "." || repoName == string(filepath.Separator) {
		return watchPageTitleSuffix
	}

	return fmt.Sprintf("%s • %s", repoName, watchPageTitleSuffix)
}

func handleIndex(pageTitle string) http.HandlerFunc {
	view := struct {
		PageTitle string
	}{
		PageTitle: pageTitle,
	}

	return func(w http.ResponseWriter, _ *http.Request) {
		// Read index.html from embedded dist directory
		distFS, err := getDistFS()
		if err != nil {
			http.Error(w, "failed to load assets", http.StatusInternalServerError)
			return
		}

		indexFile, err := distFS.Open("index.html")
		if err != nil {
			http.Error(w, "failed to load index.html", http.StatusInternalServerError)
			return
		}
		defer indexFile.Close()

		indexContent, err := io.ReadAll(indexFile)
		if err != nil {
			http.Error(w, "failed to read index.html", http.StatusInternalServerError)
			return
		}

		// Execute template to inject page title
		tmpl, err := template.New("index").Parse(string(indexContent))
		if err != nil {
			http.Error(w, "failed to parse template", http.StatusInternalServerError)
			return
		}

		var rendered bytes.Buffer
		if err := tmpl.Execute(&rendered, view); err != nil {
			http.Error(w, "failed to render page", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(rendered.Bytes()); err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
		}
	}
}

// handleCloseRepo tears down a finished worktree tab. Active worktrees are
// pinned (409); unknown ids are 404. On success the broker broadcasts the
// updated tab set, so connected clients drop the tab via the normal SSE flow.
func handleCloseRepo(b *broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repoID := r.PathValue("id")
		switch b.closeRepo(repoID) {
		case closeOK:
			w.WriteHeader(http.StatusNoContent)
		case closeActive:
			http.Error(w, "worktree is still active", http.StatusConflict)
		case closeNotFound:
			http.Error(w, "unknown worktree", http.StatusNotFound)
		}
	}
}

func handleSSE(b *broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := b.subscribe()
		defer b.unsubscribe(ch)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case payload, ok := <-ch:
				if !ok {
					return
				}
				body, err := json.Marshal(payload)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "event: %s\n", protocol.SSEEventGraph)
				for _, line := range strings.Split(string(body), "\n") {
					fmt.Fprintf(w, "data: %s\n", line)
				}
				fmt.Fprintf(w, "\n")
				flusher.Flush()
			}
		}
	}
}
