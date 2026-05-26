package rust

import (
	"strings"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// PhantomDecision describes whether a Rust file should be rendered with a
// sibling "phantom" test node, and how additions/deletions split between the
// prod-side node and the phantom test node.
type PhantomDecision struct {
	HasPhantom  bool
	ProdChanged bool
	ProdStats   vcs.FileStats
	TestStats   vcs.FileStats
}

// FindTestRegion returns the inclusive 1-indexed line range covering test-only
// code in a Rust source file: the `#[cfg(test)] mod tests { ... }` block and
// any module-scope items gated by `#[cfg(test)]`. Doctests in `///` comments
// are NOT counted. Returns ok=false when no such region exists.
func FindTestRegion(content []byte) (startLine, endLine int, ok bool) {
	if len(content) == 0 {
		return 0, 0, false
	}

	lines := strings.Split(string(content), "\n")
	minStart, maxEnd := -1, -1

	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) != "#[cfg(test)]" {
			i++
			continue
		}

		attrStart := i + 1

		j := i + 1
		for j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "#[") {
			j++
		}
		if j >= len(lines) {
			break
		}

		itemEnd := findItemEnd(lines, j)
		if minStart == -1 || attrStart < minStart {
			minStart = attrStart
		}
		if itemEnd+1 > maxEnd {
			maxEnd = itemEnd + 1
		}
		i = itemEnd + 1
	}

	if minStart == -1 {
		return 0, 0, false
	}
	return minStart, maxEnd, true
}

// findItemEnd returns the 0-indexed line on which the Rust item starting at
// `start` ends. Brace-delimited items are matched by depth; statement items
// terminate at the first line containing `;`.
func findItemEnd(lines []string, start int) int {
	if strings.Contains(lines[start], "{") {
		depth := 0
		for k := start; k < len(lines); k++ {
			for _, ch := range lines[k] {
				switch ch {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						return k
					}
				}
			}
		}
		return len(lines) - 1
	}
	for k := start; k < len(lines); k++ {
		if strings.Contains(lines[k], ";") {
			return k
		}
	}
	return start
}

// SplitDiff attributes per-line additions and deletions to prod vs test using
// the test region found in oldContent (for deletions) and newContent (for
// additions). Either content may be nil/empty: oldContent is empty for new
// files; newContent is empty for deletions.
func SplitDiff(oldContent, newContent []byte, diff vcs.FileDiff) (prod, test vcs.FileStats) {
	addInTest := makeInRange(newContent)
	delInTest := makeInRange(oldContent)

	for _, ln := range diff.Additions {
		if addInTest(ln) {
			test.Additions++
		} else {
			prod.Additions++
		}
	}
	for _, ln := range diff.Deletions {
		if delInTest(ln) {
			test.Deletions++
		} else {
			prod.Deletions++
		}
	}
	return prod, test
}

func makeInRange(content []byte) func(int) bool {
	start, end, ok := FindTestRegion(content)
	if !ok {
		return func(int) bool { return false }
	}
	return func(ln int) bool { return ln >= start && ln <= end }
}

// DecidePhantomShow returns the phantom decision for `clarity show` (point-in-
// time) mode: the phantom is present iff newContent contains a test region.
func DecidePhantomShow(newContent []byte) PhantomDecision {
	_, _, ok := FindTestRegion(newContent)
	return PhantomDecision{HasPhantom: ok}
}

// DecidePhantomWatch returns the phantom decision for `clarity watch`
// (uncommitted) mode using the pre/post images and per-line diff. The phantom
// is present iff the test region has any additions or deletions.
func DecidePhantomWatch(oldContent, newContent []byte, diff vcs.FileDiff) PhantomDecision {
	prod, test := SplitDiff(oldContent, newContent, diff)
	return PhantomDecision{
		HasPhantom:  test.Additions > 0 || test.Deletions > 0,
		ProdChanged: prod.Additions > 0 || prod.Deletions > 0,
		ProdStats:   prod,
		TestStats:   test,
	}
}
