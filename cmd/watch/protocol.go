package watch

import "time"

const (
	routeIndex  = "/"
	routeEvents = "/events"
)

const sseEventGraph = "graph"

// graphSnapshot is the atom in the watch protocol timeline.
type graphSnapshot struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	DOT       string    `json:"dot"`
}

// graphStreamPayload is the wire payload for SSE "graph" events.
type graphStreamPayload struct {
	WorkingSnapshots       []graphSnapshot      `json:"workingSnapshots"`
	PastCollections        []snapshotCollection `json:"pastCollections"`
	LatestWorkingID        int64                `json:"latestWorkingId"`
	LatestPastCollectionID int64                `json:"latestPastCollectionId"`
}

// snapshotCollection represents an archived batch of working snapshots.
type snapshotCollection struct {
	ID        int64           `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Snapshots []graphSnapshot `json:"snapshots"`
}
