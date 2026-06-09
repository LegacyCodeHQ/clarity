package depgraph

import (
	"path/filepath"
	"testing"
)

func TestCollectDependencyGraphFiles_GroupsFilesByExtension(t *testing.T) {
	tmpDir := t.TempDir()
	kotlinScriptPath := filepath.Join(tmpDir, "build.kts")

	_, _, filesByExtension, err := collectDependencyGraphFiles([]string{kotlinScriptPath})
	if err != nil {
		t.Fatalf("collectDependencyGraphFiles() error = %v", err)
	}

	if len(filesByExtension[".kts"]) != 1 {
		t.Fatalf("filesByExtension[\".kts\"] count = %d, want 1", len(filesByExtension[".kts"]))
	}
}
