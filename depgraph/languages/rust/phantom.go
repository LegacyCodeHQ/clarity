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
// code in a Rust source file. A test region is any item carrying an attribute
// that gates it to test builds:
//
//   - `#[cfg(test)]`, `#[cfg(all(test, ...))]`, `#[cfg(any(test, ...))]`
//     (and nested combinations), but NOT `#[cfg(not(test))]`
//   - `#[test]`, `#[tokio::test]`, `#[async_std::test]`, `#[rstest]`, `#[test_case]`
//
// Doctests inside `///` comments are NOT counted. Returns ok=false when no
// such region exists.
//
// Known limitation: the item-end finder uses naive brace counting and does
// not strip string literals or `// }` comments. Hand-crafted Rust that places
// a `}` inside a string in the test region can mis-balance the count.
func FindTestRegion(content []byte) (startLine, endLine int, ok bool) {
	if len(content) == 0 {
		return 0, 0, false
	}

	lines := strings.Split(string(content), "\n")
	minStart, maxEnd := -1, -1

	i := 0
	for i < len(lines) {
		if !isTestAttr(lines[i]) {
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

// isTestAttr reports whether a single source line is an attribute that gates
// the following item to test builds.
func isTestAttr(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "#[") || !strings.HasSuffix(line, "]") {
		return false
	}
	body := strings.TrimSpace(line[2 : len(line)-1])

	switch body {
	case "test", "tokio::test", "async_std::test", "rstest", "test_case":
		return true
	}

	if strings.HasPrefix(body, "cfg(") && strings.HasSuffix(body, ")") {
		return cfgEnablesTest(body[len("cfg(") : len(body)-1])
	}
	return false
}

// cfgEnablesTest reports whether a `cfg(...)` expression is satisfied in test
// mode — i.e. contains at least one positive `test` predicate not enclosed
// in an odd number of `not(...)` wrappers.
func cfgEnablesTest(expr string) bool {
	return hasPositiveTest(expr, 0)
}

func hasPositiveTest(expr string, negationDepth int) bool {
	i := 0
	for i < len(expr) {
		c := expr[i]
		if c == ' ' || c == '\t' || c == ',' {
			i++
			continue
		}
		if c == '"' {
			// Skip a string literal so `feature = "test"` doesn't match.
			i++
			for i < len(expr) && expr[i] != '"' {
				if expr[i] == '\\' && i+1 < len(expr) {
					i++
				}
				i++
			}
			if i < len(expr) {
				i++
			}
			continue
		}
		if c == '=' {
			// Skip past `=` and the following value so the value side of
			// a key=value pair never seeds a `test` match.
			i++
			for i < len(expr) && (expr[i] == ' ' || expr[i] == '\t') {
				i++
			}
			if i < len(expr) && expr[i] == '"' {
				continue
			}
			for i < len(expr) && isCfgIdentChar(expr[i]) {
				i++
			}
			continue
		}
		start := i
		for i < len(expr) && isCfgIdentChar(expr[i]) {
			i++
		}
		ident := expr[start:i]
		if ident == "" {
			i++
			continue
		}
		if i < len(expr) && expr[i] == '(' {
			end := matchingParen(expr, i)
			if end == -1 {
				return false
			}
			inner := expr[i+1 : end]
			switch ident {
			case "not":
				if hasPositiveTest(inner, negationDepth+1) {
					return true
				}
			case "all", "any":
				if hasPositiveTest(inner, negationDepth) {
					return true
				}
			}
			i = end + 1
			continue
		}
		if ident == "test" && negationDepth%2 == 0 {
			return true
		}
	}
	return false
}

func matchingParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isCfgIdentChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z',
		c >= 'A' && c <= 'Z',
		c >= '0' && c <= '9',
		c == '_', c == ':':
		return true
	}
	return false
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
