package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteAgentsFileCreatesWithTemplate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "AGENTS.md")

	created, updated, err := writeAgentsFile(filename)
	require.NoError(t, err)
	require.True(t, created)
	require.True(t, updated)

	contents, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Contains(t, string(contents), strings.TrimSpace(setupTemplate))
}

func TestWriteAgentsFileSkipsWhenSanityGraphMentioned(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "AGENTS.md")
	initial := "# Existing\n\nUse sanity graph to visualize changes.\n"

	err := os.WriteFile(filename, []byte(initial), 0644)
	require.NoError(t, err)

	created, updated, err := writeAgentsFile(filename)
	require.NoError(t, err)
	require.False(t, created)
	require.False(t, updated)

	contents, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, initial, string(contents))
}

func TestWriteAgentsFileAppendsWhenMissing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "AGENTS.md")
	initial := "# Existing\n"

	err := os.WriteFile(filename, []byte(initial), 0644)
	require.NoError(t, err)

	created, updated, err := writeAgentsFile(filename)
	require.NoError(t, err)
	require.False(t, created)
	require.True(t, updated)

	contents, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(contents), initial))
	require.Contains(t, string(contents), strings.TrimSpace(setupTemplate))
}
