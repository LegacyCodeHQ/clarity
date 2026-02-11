package diff

type diffMode string

const (
	diffModeWorkingTree diffMode = "working-tree"
	diffModeCommit      diffMode = "commit"
)

type commitComparison struct {
	baseRef   string
	targetRef string
	mode      diffMode
}
