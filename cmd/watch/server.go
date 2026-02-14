package watch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxSnapshots = 250

const watchPageTitleSuffix = "clarity watch"

// broker manages SSE client connections and broadcasts graph snapshots.
type broker struct {
	mu             sync.Mutex
	clients        map[chan graphStreamPayload]struct{}
	history        []graphSnapshot
	archivedCycles []snapshotCollection
	nextID         int64
	nextCycleID    int64
	hasState       bool
}

func newBroker() *broker {
	return &broker{
		clients: make(map[chan graphStreamPayload]struct{}),
	}
}

func (b *broker) subscribe() chan graphStreamPayload {
	ch := make(chan graphStreamPayload, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	payload, ok := b.currentPayloadLocked()
	if ok {
		ch <- payload
	}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan graphStreamPayload) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *broker) publish(dot string) {
	b.mu.Lock()
	if len(b.history) > 0 && b.history[len(b.history)-1].DOT == dot {
		b.mu.Unlock()
		return
	}

	b.nextID++
	b.history = append(b.history, graphSnapshot{
		ID:        b.nextID,
		Timestamp: time.Now().UTC(),
		DOT:       dot,
	})
	if len(b.history) > maxSnapshots {
		b.history = b.history[len(b.history)-maxSnapshots:]
	}
	b.hasState = true

	payload, _ := b.currentPayloadLocked()
	for ch := range b.clients {
		pushLatestPayload(ch, payload)
	}
	b.mu.Unlock()
}

func (b *broker) archiveWorkingSet() {
	b.mu.Lock()
	if len(b.history) > 0 {
		archivedSnapshots := make([]graphSnapshot, len(b.history))
		copy(archivedSnapshots, b.history)
		b.nextCycleID++
		b.archivedCycles = append(b.archivedCycles, snapshotCollection{
			ID:        b.nextCycleID,
			Timestamp: time.Now().UTC(),
			Snapshots: archivedSnapshots,
		})
	}

	b.history = nil
	b.hasState = true
	payload, _ := b.currentPayloadLocked()
	for ch := range b.clients {
		pushLatestPayload(ch, payload)
	}
	b.mu.Unlock()
}

func (b *broker) clearWorkingSet() {
	b.mu.Lock()
	if len(b.history) == 0 && b.hasState {
		b.mu.Unlock()
		return
	}

	b.history = nil
	b.hasState = true
	payload, _ := b.currentPayloadLocked()
	for ch := range b.clients {
		pushLatestPayload(ch, payload)
	}
	b.mu.Unlock()
}

func (b *broker) currentPayloadLocked() (graphStreamPayload, bool) {
	pastCollections := b.copyArchivedCyclesLocked()
	latestPastCollectionID := b.latestPastCollectionIDLocked()
	if len(b.history) == 0 {
		if b.hasState {
			return graphStreamPayload{
				WorkingSnapshots:       []graphSnapshot{},
				PastCollections:        pastCollections,
				LatestWorkingID:        0,
				LatestPastCollectionID: latestPastCollectionID,
			}, true
		}
		return graphStreamPayload{}, false
	}

	snapshots := make([]graphSnapshot, len(b.history))
	copy(snapshots, b.history)

	return graphStreamPayload{
		WorkingSnapshots:       snapshots,
		PastCollections:        pastCollections,
		LatestWorkingID:        b.history[len(b.history)-1].ID,
		LatestPastCollectionID: latestPastCollectionID,
	}, true
}

func (b *broker) copyArchivedCyclesLocked() []snapshotCollection {
	if len(b.archivedCycles) == 0 {
		return []snapshotCollection{}
	}

	copied := make([]snapshotCollection, len(b.archivedCycles))
	for i, cycle := range b.archivedCycles {
		snapshots := make([]graphSnapshot, len(cycle.Snapshots))
		copy(snapshots, cycle.Snapshots)
		copied[i] = snapshotCollection{
			ID:        cycle.ID,
			Timestamp: cycle.Timestamp,
			Snapshots: snapshots,
		}
	}
	return copied
}

func (b *broker) latestPastCollectionIDLocked() int64 {
	if len(b.archivedCycles) == 0 {
		return 0
	}
	return b.archivedCycles[len(b.archivedCycles)-1].ID
}

func pushLatestPayload(ch chan graphStreamPayload, payload graphStreamPayload) {
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
	mux.HandleFunc(routeIndex, handleIndex(buildWatchPageTitle(repoPath)))

	// Serve all static assets from embedded dist directory
	distFS, err := getDistFS()
	if err != nil {
		panic(fmt.Sprintf("failed to get dist FS: %v", err))
	}
	mux.Handle("/assets/", http.FileServer(http.FS(distFS)))

	// Serve SSE endpoint (unchanged)
	mux.HandleFunc(routeEvents, handleSSE(b))

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
				fmt.Fprintf(w, "event: %s\n", sseEventGraph)
				for _, line := range strings.Split(string(body), "\n") {
					fmt.Fprintf(w, "data: %s\n", line)
				}
				fmt.Fprintf(w, "\n")
				flusher.Flush()
			}
		}
	}
}
