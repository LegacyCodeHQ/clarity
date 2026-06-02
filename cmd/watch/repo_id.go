package watch

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

const primaryRepoID = "primary"

// repoIDFor returns the stable id used to key all snapshots/collections for a
// working tree. The primary worktree (when watch was launched from inside one)
// gets the literal id "primary"; every other tree gets "wt-<hash8>", an
// 8-character hex digest of its absolute path.
func repoIDFor(absPath string, isPrimary bool) string {
	if isPrimary {
		return primaryRepoID
	}
	sum := sha256.Sum256([]byte(absPath))
	return "wt-" + hex.EncodeToString(sum[:])[:8]
}

// repoLabel returns a human-readable tab label for a working tree.
func repoLabel(absPath, _ string) string {
	base := filepath.Base(filepath.Clean(absPath))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}
