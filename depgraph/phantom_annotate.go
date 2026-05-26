package depgraph

import (
	"path/filepath"

	"github.com/LegacyCodeHQ/clarity/depgraph/languages/rust"
	"github.com/LegacyCodeHQ/clarity/vcs"
)

// AnnotateRustPhantomsShow populates PhantomMetadata for every .rs file in the
// graph whose contents include a `#[cfg(test)]` region. Intended for
// `clarity show` (point-in-time) usage where the presence of an in-file test
// region is enough to render the phantom node.
func (g *FileDependencyGraph) AnnotateRustPhantomsShow(contentReader vcs.ContentReader) {
	if contentReader == nil {
		return
	}
	for path, meta := range g.Meta.Files {
		if filepath.Ext(path) != ".rs" || meta.IsTest {
			continue
		}
		content, err := contentReader(path)
		if err != nil {
			continue
		}
		decision := rust.DecidePhantomShow(content)
		if !decision.HasPhantom {
			continue
		}
		meta.Phantom = &PhantomMetadata{Kind: "rust-test"}
		g.Meta.Files[path] = meta
	}
}

// AnnotateRustPhantomsWatch populates PhantomMetadata for .rs files in the
// graph using watch-mode rules: the phantom is added only when the file's
// test region has additions or deletions in the supplied diff. The file's
// own Stats are also rewritten to reflect the prod-side split.
func (g *FileDependencyGraph) AnnotateRustPhantomsWatch(
	diffs map[string]vcs.FileDiff,
	oldContent vcs.ContentReader,
	newContent vcs.ContentReader,
) {
	if newContent == nil {
		return
	}
	for path, meta := range g.Meta.Files {
		if filepath.Ext(path) != ".rs" || meta.IsTest {
			continue
		}
		diff, ok := diffs[path]
		if !ok {
			continue
		}
		var oldBytes []byte
		if oldContent != nil {
			if b, err := oldContent(path); err == nil {
				oldBytes = b
			}
		}
		newBytes, err := newContent(path)
		if err != nil {
			newBytes = nil
		}
		decision := rust.DecidePhantomWatch(oldBytes, newBytes, diff)

		prodStats := decision.ProdStats
		if diff.IsNew {
			prodStats.IsNew = true
		}
		meta.Stats = &prodStats

		if decision.HasPhantom {
			testStats := decision.TestStats
			if diff.IsNew {
				testStats.IsNew = true
			}
			meta.Phantom = &PhantomMetadata{
				Kind:        "rust-test",
				Stats:       &testStats,
				ProdChanged: decision.ProdChanged,
			}
		}
		g.Meta.Files[path] = meta
	}
}
