# Watch Protocol (Server <-> Web Viewer)

This document defines the protocol used by `clarity watch` between the Go HTTP server and the browser viewer.

## Transport

- Browser -> Server:
  - `GET /` returns `viewer.html`.
  - `GET /viewer.js` returns viewer JavaScript.
  - `GET /events` opens an SSE stream.
- Server -> Browser:
  - SSE event name: `graph`
  - Event payload: JSON object (schema below)

Notes:
- The browser does not send custom JSON commands today; communication from browser to server is request-level (`GET`) plus SSE reconnection behavior from `EventSource`.
- The server may replay the latest state immediately on subscribe.

## SSE Payload Schema

```json
{
  "workingSnapshots": [
    {
      "id": 1,
      "timestamp": "2026-02-12T10:00:00Z",
      "dot": "digraph { A -> B; }"
    }
  ],
  "pastCollections": [
    {
      "id": 1,
      "timestamp": "2026-02-12T09:00:00Z",
      "snapshots": [
        {
          "id": 10,
          "timestamp": "2026-02-12T08:59:00Z",
          "dot": "digraph { X -> Y; }"
        }
      ]
    }
  ],
  "latestWorkingId": 1,
  "latestPastCollectionId": 1
}
```

Field rules:
- `workingSnapshots` is the active timeline for current uncommitted changes.
- `pastCollections` is the wire field for archived sessions: previously completed working sets.
- `dot` is Graphviz DOT source rendered by the viewer.
- `latestWorkingId` and `latestPastCollectionId` are monotonic markers for newest IDs in each lane.

## Compatibility Expectations

- The viewer must tolerate malformed payloads and ignore invalid snapshots/session entries.
- Missing arrays should be treated as empty arrays.
- Unknown fields should be ignored by both sides.

## Source of Truth in Code

- Server payload structs and SSE emit path:
  - `/Users/ragunath/GolandProjects/clarity/cmd/watch/protocol.go`
  - `/Users/ragunath/GolandProjects/clarity/cmd/watch/server.go`
- Viewer protocol normalization (wire -> internal model):
  - `/Users/ragunath/GolandProjects/clarity/cmd/watch/viewer_protocol.mjs`
- Viewer state transitions (internal model only):
  - `/Users/ragunath/GolandProjects/clarity/cmd/watch/viewer_state.mjs`
- Viewer state contract tests:
  - `/Users/ragunath/GolandProjects/clarity/cmd/watch/viewer_state.test.mjs`
  - `/Users/ragunath/GolandProjects/clarity/cmd/watch/viewer_protocol.test.mjs`
