package vcs

// FileDiff carries per-line change data for a single file. Line numbers are
// 1-indexed: Additions reference the post-image (new content), Deletions
// reference the pre-image (old content).
type FileDiff struct {
	Additions []int
	Deletions []int
	IsNew     bool
	IsDeleted bool
	IsRenamed bool
}
