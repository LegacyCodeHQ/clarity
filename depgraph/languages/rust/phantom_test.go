package rust

import (
	"testing"

	"github.com/LegacyCodeHQ/clarity/vcs"
)

// Reusable Rust source fixtures keyed by shape.
//
// prodOnly: no tests anywhere.
// withTestMod: production code followed by a `#[cfg(test)] mod tests { ... }` block.
// withTestModAndHelper: same as above, plus a `#[cfg(test)]` helper at module scope.
// withDoctest: production code carrying a `///` doctest, no `#[cfg(test)]` region.
const (
	prodOnly = `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}
`

	// Lines 1..3 are prod. Lines 5..10 are the test region.
	withTestMod = `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn it_adds() { assert_eq!(add(1, 2), 3); }
}
`

	// Lines 1..3 prod. Lines 5..6 module-scope test helper. Lines 8..13 test mod.
	withTestModAndHelper = `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

#[cfg(test)]
fn make_pair() -> (i32, i32) { (1, 2) }

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn it_adds() { let (a, b) = make_pair(); assert_eq!(add(a, b), 3); }
}
`

	withDoctest = `/// Adds two numbers.
///
/// ` + "```" + `
/// assert_eq!(my_crate::add(1, 2), 3);
/// ` + "```" + `
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}
`
)

func TestIsTestAttr(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"#[cfg(test)]", true},
		{"  #[cfg(test)]  ", true},
		{"#[test]", true},
		{"#[tokio::test]", true},
		{"#[async_std::test]", true},
		{"#[rstest]", true},
		{"#[test_case]", true},
		{`#[cfg(all(test, feature = "x"))]`, true},
		{`#[cfg(any(test, debug_assertions))]`, true},
		{"#[cfg(all(feature = \"x\", test))]", true},
		{"#[cfg(not(test))]", false},
		{`#[cfg(all(not(test), feature = "x"))]`, false},
		{"#[cfg(any(not(test), not(production)))]", false},
		{"#[cfg(all(any(test, debug_assertions), feature = \"x\"))]", true},
		{`#[cfg(feature = "test")]`, false},
		{"#[derive(Debug)]", false},
		{"fn main() {}", false},
		{"// #[cfg(test)]", false},
	}
	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
			got := isTestAttr(tc.line)
			if got != tc.want {
				t.Fatalf("isTestAttr(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestFindTestRegion_GatedByCfgAll(t *testing.T) {
	content := `pub fn add(a: i32, b: i32) -> i32 { a + b }

#[cfg(all(test, feature = "extras"))]
mod tests {
    #[test]
    fn it_works() {}
}
`
	start, end, ok := FindTestRegion([]byte(content))
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if start != 3 || end != 7 {
		t.Fatalf("range = [%d, %d], want [3, 7]", start, end)
	}
}

func TestFindTestRegion_ModuleScopeTestFn(t *testing.T) {
	content := `pub fn add(a: i32, b: i32) -> i32 { a + b }

#[test]
fn it_adds() { assert_eq!(add(1, 2), 3); }
`
	start, end, ok := FindTestRegion([]byte(content))
	if !ok {
		t.Fatalf("expected ok=true for module-scope #[test]")
	}
	if start != 3 || end != 4 {
		t.Fatalf("range = [%d, %d], want [3, 4]", start, end)
	}
}

func TestFindTestRegion_CfgNotTestIgnored(t *testing.T) {
	content := `#[cfg(not(test))]
fn only_in_prod() {}
`
	if _, _, ok := FindTestRegion([]byte(content)); ok {
		t.Fatalf("expected ok=false for cfg(not(test))")
	}
}

func TestFindTestRegion(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantOK    bool
		wantStart int
		wantEnd   int
	}{
		{
			name:    "no tests returns ok=false",
			content: prodOnly,
			wantOK:  false,
		},
		{
			name:      "cfg(test) mod tests block is the region",
			content:   withTestMod,
			wantOK:    true,
			wantStart: 5,
			wantEnd:   10,
		},
		{
			name:      "cfg(test) helper at module scope is included",
			content:   withTestModAndHelper,
			wantOK:    true,
			wantStart: 5,
			wantEnd:   13,
		},
		{
			name:    "doctests in /// comments are not a test region",
			content: withDoctest,
			wantOK:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end, ok := FindTestRegion([]byte(tc.content))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("range = [%d, %d], want [%d, %d]", start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

// TestSplitDiff covers matrix scenarios 11-14 (attribution).
func TestSplitDiff(t *testing.T) {
	tests := []struct {
		name       string
		oldContent string
		newContent string
		diff       vcs.FileDiff
		wantProd   vcs.FileStats
		wantTest   vcs.FileStats
	}{
		{
			name:       "scenario 11: changes inside mod tests block attribute to test",
			oldContent: withTestMod,
			newContent: withTestMod,
			// Lines 7,8 (inside test mod) are additions; line 9 is a deletion.
			diff: vcs.FileDiff{
				Additions: []int{7, 8},
				Deletions: []int{9},
			},
			wantProd: vcs.FileStats{},
			wantTest: vcs.FileStats{Additions: 2, Deletions: 1},
		},
		{
			name:       "scenario 12: cfg(test) helper at module scope attributes to test",
			oldContent: withTestModAndHelper,
			newContent: withTestModAndHelper,
			// Line 6 (inside the cfg(test) helper) and line 12 (inside mod tests).
			diff: vcs.FileDiff{
				Additions: []int{6, 12},
				Deletions: []int{},
			},
			wantProd: vcs.FileStats{},
			wantTest: vcs.FileStats{Additions: 2},
		},
		{
			name:       "scenario 13: doctest in /// attributes to prod",
			oldContent: withDoctest,
			newContent: withDoctest,
			// Line 4 sits inside the /// doctest block — must count as prod.
			diff: vcs.FileDiff{
				Additions: []int{4},
			},
			wantProd: vcs.FileStats{Additions: 1},
			wantTest: vcs.FileStats{},
		},
		{
			name:       "scenario 14: move from prod to test (deletion in prod, addition in test)",
			oldContent: withTestMod,
			newContent: withTestMod,
			// Deletion at line 2 (prod), addition at line 8 (inside test mod).
			diff: vcs.FileDiff{
				Additions: []int{8},
				Deletions: []int{2},
			},
			wantProd: vcs.FileStats{Deletions: 1},
			wantTest: vcs.FileStats{Additions: 1},
		},
		{
			name:       "mixed: additions span both regions",
			oldContent: withTestMod,
			newContent: withTestMod,
			diff: vcs.FileDiff{
				Additions: []int{2, 7, 8},
				Deletions: []int{1},
			},
			wantProd: vcs.FileStats{Additions: 1, Deletions: 1},
			wantTest: vcs.FileStats{Additions: 2},
		},
		{
			name: "prod between disjoint top-level test functions stays prod",
			oldContent: `#[test]
fn first_test() { assert!(true); }

pub fn convert(input: i32) -> i32 {
    let value = input + 1;
    value
}

#[test]
fn second_test() { assert_eq!(convert(1), 2); }
`,
			newContent: `#[test]
fn first_test() { assert!(true); }

pub fn convert(input: i32) -> i32 {
    let value = input.saturating_add(1);
    value.max(0)
}

#[test]
fn second_test() { assert_eq!(convert(1), 2); }
`,
			diff: vcs.FileDiff{
				Additions: []int{5, 6},
				Deletions: []int{5, 6},
			},
			wantProd: vcs.FileStats{Additions: 2, Deletions: 2},
			wantTest: vcs.FileStats{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prod, test := SplitDiff([]byte(tc.oldContent), []byte(tc.newContent), tc.diff)
			if prod != tc.wantProd {
				t.Errorf("prod = %+v, want %+v", prod, tc.wantProd)
			}
			if test != tc.wantTest {
				t.Errorf("test = %+v, want %+v", test, tc.wantTest)
			}
		})
	}
}

// TestDecidePhantomShow covers matrix scenarios 1-2 (`clarity show`).
func TestDecidePhantomShow(t *testing.T) {
	tests := []struct {
		name       string
		newContent string
		wantHas    bool
	}{
		{
			name:       "scenario 1: file with no tests has no phantom",
			newContent: prodOnly,
			wantHas:    false,
		},
		{
			name:       "scenario 2: file with cfg(test) mod has phantom",
			newContent: withTestMod,
			wantHas:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecidePhantomShow([]byte(tc.newContent))
			if got.HasPhantom != tc.wantHas {
				t.Fatalf("HasPhantom = %v, want %v", got.HasPhantom, tc.wantHas)
			}
		})
	}
}

// TestDecidePhantomWatch covers matrix scenarios 3-10 (`clarity watch`).
func TestDecidePhantomWatch(t *testing.T) {
	tests := []struct {
		name       string
		oldContent string
		newContent string
		diff       vcs.FileDiff
		want       PhantomDecision
	}{
		{
			name:       "scenario 3: prod changed, no tests in file -> no phantom, prod changed",
			oldContent: prodOnly,
			newContent: prodOnly,
			diff: vcs.FileDiff{
				Additions: []int{2},
			},
			want: PhantomDecision{
				HasPhantom:  false,
				ProdChanged: true,
				ProdStats:   vcs.FileStats{Additions: 1},
				TestStats:   vcs.FileStats{},
			},
		},
		{
			name:       "scenario 4: prod changed, tests exist but untouched -> no phantom",
			oldContent: withTestMod,
			newContent: withTestMod,
			diff: vcs.FileDiff{
				Additions: []int{2},
			},
			want: PhantomDecision{
				HasPhantom:  false,
				ProdChanged: true,
				ProdStats:   vcs.FileStats{Additions: 1},
				TestStats:   vcs.FileStats{},
			},
		},
		{
			name:       "scenario 5: prod and tests both changed -> phantom present, prod changed",
			oldContent: withTestMod,
			newContent: withTestMod,
			diff: vcs.FileDiff{
				Additions: []int{2, 8},
			},
			want: PhantomDecision{
				HasPhantom:  true,
				ProdChanged: true,
				ProdStats:   vcs.FileStats{Additions: 1},
				TestStats:   vcs.FileStats{Additions: 1},
			},
		},
		{
			name:       "scenario 6: only tests changed -> phantom present, prod unchanged context",
			oldContent: withTestMod,
			newContent: withTestMod,
			diff: vcs.FileDiff{
				Additions: []int{8},
			},
			want: PhantomDecision{
				HasPhantom:  true,
				ProdChanged: false,
				ProdStats:   vcs.FileStats{},
				TestStats:   vcs.FileStats{Additions: 1},
			},
		},
		{
			name:       "scenario 7: new file with tests -> phantom present, prod-side additions counted",
			oldContent: "",
			newContent: withTestMod,
			diff: vcs.FileDiff{
				// Whole file is new: lines 1..10 are additions.
				Additions: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				IsNew:     true,
			},
			want: PhantomDecision{
				HasPhantom:  true,
				ProdChanged: true,
				// Prod = lines 1..4 (3 code lines + 1 blank); test = lines 5..10.
				ProdStats: vcs.FileStats{Additions: 4},
				TestStats: vcs.FileStats{Additions: 6},
			},
		},
		{
			name:       "scenario 8: new file without tests -> no phantom",
			oldContent: "",
			newContent: prodOnly,
			diff: vcs.FileDiff{
				Additions: []int{1, 2, 3},
				IsNew:     true,
			},
			want: PhantomDecision{
				HasPhantom:  false,
				ProdChanged: true,
				ProdStats:   vcs.FileStats{Additions: 3},
				TestStats:   vcs.FileStats{},
			},
		},
		{
			name:       "scenario 9: deleted file that had tests -> phantom present (deletion)",
			oldContent: withTestMod,
			newContent: "",
			diff: vcs.FileDiff{
				Deletions: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				IsDeleted: true,
			},
			want: PhantomDecision{
				HasPhantom:  true,
				ProdChanged: true,
				ProdStats:   vcs.FileStats{Deletions: 4},
				TestStats:   vcs.FileStats{Deletions: 6},
			},
		},
		{
			name:       "scenario 10: deleted file without tests -> no phantom",
			oldContent: prodOnly,
			newContent: "",
			diff: vcs.FileDiff{
				Deletions: []int{1, 2, 3},
				IsDeleted: true,
			},
			want: PhantomDecision{
				HasPhantom:  false,
				ProdChanged: true,
				ProdStats:   vcs.FileStats{Deletions: 3},
				TestStats:   vcs.FileStats{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DecidePhantomWatch([]byte(tc.oldContent), []byte(tc.newContent), tc.diff)
			if got != tc.want {
				t.Fatalf("decision = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDecidePhantomWatch_Rename covers matrix scenario 15: a rename with no
// content change emits no counts and no phantom signal of its own. The rename
// indicator itself lives on the diff metadata, not the phantom decision.
func TestDecidePhantomWatch_Rename(t *testing.T) {
	got := DecidePhantomWatch([]byte(withTestMod), []byte(withTestMod), vcs.FileDiff{IsRenamed: true})
	want := PhantomDecision{
		HasPhantom:  false,
		ProdChanged: false,
		ProdStats:   vcs.FileStats{},
		TestStats:   vcs.FileStats{},
	}
	if got != want {
		t.Fatalf("decision = %+v, want %+v", got, want)
	}
}
