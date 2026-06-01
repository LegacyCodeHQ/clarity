package watch

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LegacyCodeHQ/clarity/cmd/watch/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The close endpoint is the first client→server message in the watch protocol.
// It lets the UI tear down a finished (inactive) tab; active tabs are pinned.
func TestCloseEndpoint_RemovesFinishedTab(t *testing.T) {
	b := newBroker()
	b.registerRepo(protocol.RepoDescriptor{ID: "primary", Path: "/repo", IsPrimary: true, Active: true})
	b.registerRepo(protocol.RepoDescriptor{ID: "wt-aaaaaaaa", Path: "/tmp/wt", Active: true})
	b.publish("primary", "digraph p {}")
	b.publish("wt-aaaaaaaa", "digraph w {}")
	b.markRepoFinished("wt-aaaaaaaa")

	srv := newServer(b, 0, "/repo")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/repos/wt-aaaaaaaa/close", nil))

	assert.Equal(t, http.StatusNoContent, rec.Code)

	b.mu.Lock()
	_, stillPresent := b.repoIndex["wt-aaaaaaaa"]
	b.mu.Unlock()
	assert.False(t, stillPresent, "finished tab should be gone after close")
}

func TestCloseEndpoint_RefusesActiveTab(t *testing.T) {
	b := newBroker()
	b.registerRepo(protocol.RepoDescriptor{ID: "primary", Path: "/repo", IsPrimary: true, Active: true})
	b.publish("primary", "digraph p {}")

	srv := newServer(b, 0, "/repo")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/repos/primary/close", nil))

	assert.Equal(t, http.StatusConflict, rec.Code, "closing an active tab is a conflict")

	b.mu.Lock()
	_, stillPresent := b.repoIndex["primary"]
	b.mu.Unlock()
	require.True(t, stillPresent, "active tab must survive a refused close")
}

func TestCloseEndpoint_UnknownTabReturns404(t *testing.T) {
	b := newBroker()
	b.registerRepo(protocol.RepoDescriptor{ID: "primary", Path: "/repo", IsPrimary: true, Active: true})

	srv := newServer(b, 0, "/repo")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/repos/wt-missing/close", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
