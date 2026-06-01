package protocol

import "time"

const (
	RouteIndex  = "/"
	RouteEvents = "/events"
)

const SSEEventGraph = "graph"

// RepoDescriptor names a working tree visible to the watch session.
// One descriptor per tab in the viewer.
type RepoDescriptor struct {
	// ID is a stable identifier for the tree across reconnects.
	// "primary" for the primary worktree in primary mode; "wt-<hash8>" otherwise.
	ID string `json:"id"`
	// Path is the absolute path to the working tree on disk.
	Path string `json:"path"`
	// Label is a human-readable name for tab display.
	Label string `json:"label"`
	// IsPrimary marks the primary worktree of the repository.
	IsPrimary bool `json:"isPrimary"`
}

// GraphSnapshot is the atom in the watch protocol timeline.
type GraphSnapshot struct {
	ID        int64     `json:"id"`
	RepoID    string    `json:"repoId"`
	Timestamp time.Time `json:"timestamp"`
	DOT       string    `json:"dot"`
	// SessionStart marks the first snapshot recorded for a worktree in this
	// watch session — the state that already existed when the watcher attached.
	// Changes made before this point are not in the timeline. Set once per repo
	// and preserved across commit/archive cycles.
	SessionStart bool `json:"sessionStart,omitempty"`
}

// GraphStreamPayload is the wire payload for SSE "graph" events.
type GraphStreamPayload struct {
	Repos                  []RepoDescriptor     `json:"repos"`
	WorkingSnapshots       []GraphSnapshot      `json:"workingSnapshots"`
	PastCollections        []SnapshotCollection `json:"pastCollections"`
	LatestWorkingID        int64                `json:"latestWorkingId"`
	LatestPastCollectionID int64                `json:"latestPastCollectionId"`
}

// SnapshotCollection represents an archived batch of working snapshots.
type SnapshotCollection struct {
	ID        int64           `json:"id"`
	RepoID    string          `json:"repoId"`
	Timestamp time.Time       `json:"timestamp"`
	Snapshots []GraphSnapshot `json:"snapshots"`
}
