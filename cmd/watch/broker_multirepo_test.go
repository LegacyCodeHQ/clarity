package watch

import (
	"testing"
	"time"

	"github.com/LegacyCodeHQ/clarity/cmd/watch/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroker_RegisterRepo_EmitsTabsToSubscribers(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.registerRepo(protocol.RepoDescriptor{
		ID:        "primary",
		Path:      "/repo",
		Label:     "clarity-cli",
		IsPrimary: true,
	})

	select {
	case got := <-ch:
		require.Len(t, got.Repos, 1)
		assert.Equal(t, "primary", got.Repos[0].ID)
		assert.True(t, got.Repos[0].IsPrimary)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for tab descriptor")
	}
}

func TestBroker_PublishToMultipleRepos_FlatPayloadTaggedByRepoID(t *testing.T) {
	b := newBroker()
	b.registerRepo(protocol.RepoDescriptor{ID: "primary", Path: "/repo", Label: "primary", IsPrimary: true})
	b.registerRepo(protocol.RepoDescriptor{ID: "wt-aaaaaaaa", Path: "/tmp/wt", Label: "wt", IsPrimary: false})

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.publish("primary", "digraph primary { A; }")
	b.publish("wt-aaaaaaaa", "digraph wt { B; }")

	// Drain until we see both repos represented.
	deadline := time.After(2 * time.Second)
	var last protocol.GraphStreamPayload
	for {
		select {
		case got := <-ch:
			last = got
			if len(got.WorkingSnapshots) == 2 {
				goto done
			}
		case <-deadline:
			t.Fatalf("timed out; last payload had %d snapshots", len(last.WorkingSnapshots))
		}
	}
done:
	require.Len(t, last.WorkingSnapshots, 2)
	repoIDs := []string{last.WorkingSnapshots[0].RepoID, last.WorkingSnapshots[1].RepoID}
	assert.ElementsMatch(t, []string{"primary", "wt-aaaaaaaa"}, repoIDs)

	for _, s := range last.WorkingSnapshots {
		if s.RepoID == "primary" {
			assert.Equal(t, "digraph primary { A; }", s.DOT)
		} else {
			assert.Equal(t, "digraph wt { B; }", s.DOT)
		}
	}
}

func TestBroker_MarksSessionStartOncePerRepo(t *testing.T) {
	b := newBroker()
	b.registerRepo(protocol.RepoDescriptor{ID: "primary", Path: "/repo", IsPrimary: true})

	// The first snapshot recorded for a repo is the session start: it captures
	// whatever already existed in the working tree when the watcher attached.
	b.publish("primary", "digraph { A; }")
	// Subsequent live snapshots are not session starts.
	b.publish("primary", "digraph { A; B; }")

	ch := b.subscribe()
	got := <-ch
	require.Len(t, got.WorkingSnapshots, 2)
	assert.True(t, got.WorkingSnapshots[0].SessionStart, "first snapshot should be marked session start")
	assert.False(t, got.WorkingSnapshots[1].SessionStart, "second snapshot should not be a session start")
	b.unsubscribe(ch)

	// A commit archives the working set; the next snapshot belongs to a new
	// cycle but is still mid-session — it must NOT be a fresh session start.
	b.archiveWorkingSet("primary")
	b.publish("primary", "digraph { C; }")

	ch2 := b.subscribe()
	got2 := <-ch2
	defer b.unsubscribe(ch2)
	require.Len(t, got2.WorkingSnapshots, 1)
	assert.False(t, got2.WorkingSnapshots[0].SessionStart, "post-archive snapshot is mid-session, not a session start")
	// The archived collection keeps the original session-start marker so the
	// boundary stays visible when browsing past cycles.
	require.Len(t, got2.PastCollections, 1)
	require.NotEmpty(t, got2.PastCollections[0].Snapshots)
	assert.True(t, got2.PastCollections[0].Snapshots[0].SessionStart, "archived first snapshot retains session start")
}

func TestBroker_ArchiveOnlyAffectsThatRepo(t *testing.T) {
	b := newBroker()
	b.registerRepo(protocol.RepoDescriptor{ID: "primary", Path: "/repo", IsPrimary: true})
	b.registerRepo(protocol.RepoDescriptor{ID: "wt-aaaaaaaa", Path: "/tmp/wt", IsPrimary: false})

	b.publish("primary", "digraph p {}")
	b.publish("wt-aaaaaaaa", "digraph w {}")

	b.archiveWorkingSet("primary")

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	select {
	case got := <-ch:
		// wt's working snapshot should remain; primary's was archived.
		require.Len(t, got.WorkingSnapshots, 1)
		assert.Equal(t, "wt-aaaaaaaa", got.WorkingSnapshots[0].RepoID)
		require.Len(t, got.PastCollections, 1)
		assert.Equal(t, "primary", got.PastCollections[0].RepoID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for payload")
	}
}

func TestBroker_UnregisterRepo_DropsTabAndHistory(t *testing.T) {
	b := newBroker()
	b.registerRepo(protocol.RepoDescriptor{ID: "primary", Path: "/repo", IsPrimary: true})
	b.registerRepo(protocol.RepoDescriptor{ID: "wt-aaaaaaaa", Path: "/tmp/wt", IsPrimary: false})
	b.publish("primary", "digraph p {}")
	b.publish("wt-aaaaaaaa", "digraph w {}")

	b.unregisterRepo("wt-aaaaaaaa")

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	select {
	case got := <-ch:
		require.Len(t, got.Repos, 1)
		assert.Equal(t, "primary", got.Repos[0].ID)
		require.Len(t, got.WorkingSnapshots, 1)
		assert.Equal(t, "primary", got.WorkingSnapshots[0].RepoID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for payload after unregister")
	}
}
