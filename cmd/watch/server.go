package watch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxSnapshots = 250

type graphSnapshot struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	DOT       string    `json:"dot"`
}

type graphStreamPayload struct {
	WorkingSnapshots []graphSnapshot `json:"workingSnapshots"`
	PastSnapshots    []graphSnapshot `json:"pastSnapshots"`
	LatestWorkingID  int64           `json:"latestWorkingId"`
	LatestPastID     int64           `json:"latestPastId"`
}

// broker manages SSE client connections and broadcasts graph snapshots.
type broker struct {
	mu             sync.Mutex
	clients        map[chan graphStreamPayload]struct{}
	history        []graphSnapshot
	archivedCycles [][]graphSnapshot
	nextID         int64
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
		select {
		case ch <- payload:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *broker) reset() {
	b.mu.Lock()
	if len(b.history) > 0 {
		archived := make([]graphSnapshot, len(b.history))
		copy(archived, b.history)
		b.archivedCycles = append(b.archivedCycles, archived)
	}

	if len(b.history) == 0 && b.hasState {
		b.mu.Unlock()
		return
	}

	b.history = nil
	b.hasState = true
	payload := graphStreamPayload{
		WorkingSnapshots: []graphSnapshot{},
		PastSnapshots:    b.flattenPastSnapshotsLocked(),
		LatestWorkingID:  0,
		LatestPastID:     b.latestPastIDLocked(),
	}
	for ch := range b.clients {
		select {
		case ch <- payload:
		default:
		}
	}
	b.mu.Unlock()
}

func (b *broker) currentPayloadLocked() (graphStreamPayload, bool) {
	pastSnapshots := b.flattenPastSnapshotsLocked()
	latestPastID := b.latestPastIDLocked()
	if len(b.history) == 0 {
		if b.hasState {
			return graphStreamPayload{
				WorkingSnapshots: []graphSnapshot{},
				PastSnapshots:    pastSnapshots,
				LatestWorkingID:  0,
				LatestPastID:     latestPastID,
			}, true
		}
		return graphStreamPayload{}, false
	}

	snapshots := make([]graphSnapshot, len(b.history))
	copy(snapshots, b.history)

	return graphStreamPayload{
		WorkingSnapshots: snapshots,
		PastSnapshots:    pastSnapshots,
		LatestWorkingID:  b.history[len(b.history)-1].ID,
		LatestPastID:     latestPastID,
	}, true
}

func (b *broker) flattenPastSnapshotsLocked() []graphSnapshot {
	if len(b.archivedCycles) == 0 {
		return []graphSnapshot{}
	}

	total := 0
	for _, cycle := range b.archivedCycles {
		total += len(cycle)
	}

	flattened := make([]graphSnapshot, 0, total)
	for _, cycle := range b.archivedCycles {
		flattened = append(flattened, cycle...)
	}
	return flattened
}

func (b *broker) latestPastIDLocked() int64 {
	if len(b.archivedCycles) == 0 {
		return 0
	}
	lastCycle := b.archivedCycles[len(b.archivedCycles)-1]
	if len(lastCycle) == 0 {
		return 0
	}
	return lastCycle[len(lastCycle)-1].ID
}

func newServer(b *broker, port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/viewer.js", handleViewerJS)
	mux.HandleFunc("/events", handleSSE(b))

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(indexHTML)); err != nil {
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}

func handleViewerJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	if _, err := w.Write([]byte(viewerJS)); err != nil {
		http.Error(w, "failed to render script", http.StatusInternalServerError)
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
				fmt.Fprintf(w, "event: graph\n")
				for _, line := range strings.Split(string(body), "\n") {
					fmt.Fprintf(w, "data: %s\n", line)
				}
				fmt.Fprintf(w, "\n")
				flusher.Flush()
			}
		}
	}
}
