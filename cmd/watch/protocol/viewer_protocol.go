package protocol

import "time"

const (
	RouteIndex  = "/"
	RouteEvents = "/events"
	// RouteCloseRepo tears down a finished (inactive) worktree tab. The {id}
	// path value is the RepoDescriptor ID. This is the one client→server
	// message in the watch protocol; everything else flows server→client.
	RouteCloseRepo = "POST /repos/{id}/close"
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
	// Active reports whether the worktree is still being watched. It flips to
	// false when the underlying git worktree is removed: the tab stays visible
	// as a frozen, read-only record and becomes user-closable. The primary
	// worktree stays active for the life of the watch session.
	Active bool `json:"active"`
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

// CommitSummary describes a commit that caused a working snapshot cycle to
// freeze into an archived collection.
type CommitSummary struct {
	Hash      string    `json:"hash"`
	ShortHash string    `json:"shortHash"`
	Subject   string    `json:"subject"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
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
	ID            int64           `json:"id"`
	RepoID        string          `json:"repoId"`
	Timestamp     time.Time       `json:"timestamp"`
	Snapshots     []GraphSnapshot `json:"snapshots"`
	CommitHistory []CommitSummary `json:"commitHistory,omitempty"`
}
