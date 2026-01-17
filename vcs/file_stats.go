package vcs

// FileStats represents statistics for a file (additions and deletions)
type FileStats struct {
	Additions int
	Deletions int
	IsNew     bool
}
