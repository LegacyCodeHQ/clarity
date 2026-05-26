package markdown

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func extractPaths(links []MarkdownLink) []string {
	paths := make([]string, len(links))
	for i, l := range links {
		paths[i] = l.Path()
	}
	return paths
}

func TestParseMarkdownLinks_InlineLinksAndImages(t *testing.T) {
	source := `
# Title

See [the guide](./guide.md) and [api](api/index.md "API docs").

![diagram](./assets/diagram.png)
`
	links := ParseMarkdownLinks([]byte(source))
	paths := extractPaths(links)

	assert.Contains(t, paths, "./guide.md")
	assert.Contains(t, paths, "api/index.md")
	assert.Contains(t, paths, "./assets/diagram.png")
	assert.Len(t, links, 3)

	for _, l := range links {
		if l.Path() == "./assets/diagram.png" {
			assert.True(t, l.IsImage())
		} else {
			assert.False(t, l.IsImage())
		}
	}
}

func TestParseMarkdownLinks_SkipsExternalAndAnchors(t *testing.T) {
	source := `
[home](https://example.com)
[mail](mailto:foo@bar.com)
[anchor](#section)
[local](./other.md#section)
`
	paths := extractPaths(ParseMarkdownLinks([]byte(source)))

	assert.NotContains(t, paths, "https://example.com")
	assert.NotContains(t, paths, "mailto:foo@bar.com")
	assert.NotContains(t, paths, "#section")
	assert.Contains(t, paths, "./other.md")
}

func TestParseMarkdownLinks_ReferenceDefinitions(t *testing.T) {
	source := `
See [the guide][g] and [api][a].

[g]: ./guide.md
[a]: api/index.md "API"
`
	paths := extractPaths(ParseMarkdownLinks([]byte(source)))

	assert.Contains(t, paths, "./guide.md")
	assert.Contains(t, paths, "api/index.md")
}

func TestParseMarkdownLinks_IgnoresFencedAndInlineCode(t *testing.T) {
	source := "Real [link](./real.md)\n" +
		"Inline `code [fake](./fake.md)` here.\n" +
		"```\n" +
		"[fenced](./fenced.md)\n" +
		"```\n"

	paths := extractPaths(ParseMarkdownLinks([]byte(source)))

	assert.Contains(t, paths, "./real.md")
	assert.NotContains(t, paths, "./fake.md")
	assert.NotContains(t, paths, "./fenced.md")
}

func TestResolveMarkdownLinkPath_RelativeFile(t *testing.T) {
	supplied := map[string]bool{
		"/project/docs/guide.md":     true,
		"/project/docs/api/index.md": true,
	}

	got := ResolveMarkdownLinkPath("/project/docs/intro.md", "./guide.md", supplied)
	assert.Equal(t, []string{"/project/docs/guide.md"}, got)

	got = ResolveMarkdownLinkPath("/project/docs/intro.md", "api/index.md", supplied)
	assert.Equal(t, []string{filepath.Clean("/project/docs/api/index.md")}, got)
}

func TestResolveMarkdownLinkPath_DirectoryReadme(t *testing.T) {
	supplied := map[string]bool{
		"/project/docs/api/README.md": true,
	}

	got := ResolveMarkdownLinkPath("/project/docs/intro.md", "./api/", supplied)
	assert.Equal(t, []string{filepath.Clean("/project/docs/api/README.md")}, got)
}

func TestResolveMarkdownLinkPath_MissingTarget(t *testing.T) {
	supplied := map[string]bool{
		"/project/docs/guide.md": true,
	}

	got := ResolveMarkdownLinkPath("/project/docs/intro.md", "./missing.md", supplied)
	assert.Nil(t, got)
}

// Static-site generators (VitePress, Docusaurus, Astro Starlight, ...) link
// between docs with site-absolute URLs starting with "/". These should resolve
// against the project regardless of where the source file lives.
func TestResolveMarkdownLinkPath_SiteAbsoluteLink(t *testing.T) {
	supplied := map[string]bool{
		"/project/src/api/sfc-spec.md":         true,
		"/project/src/guide/scaling-up/sfc.md": true,
		"/project/src/guide/scaling-up/ssr.md": true,
		"/project/src/api/general.md":          true,
	}

	got := ResolveMarkdownLinkPath("/project/src/api/sfc-spec.md", "/guide/scaling-up/sfc", supplied)
	assert.Equal(t, []string{"/project/src/guide/scaling-up/sfc.md"}, got)

	got = ResolveMarkdownLinkPath("/project/src/api/sfc-spec.md", "/api/general", supplied)
	assert.Equal(t, []string{"/project/src/api/general.md"}, got)
}

// VitePress renders pages as .html, and authors often write links with the
// rendered extension. Strip the rendered extension before resolving so the
// underlying .md source is found.
func TestResolveMarkdownLinkPath_HtmlExtensionStripped(t *testing.T) {
	supplied := map[string]bool{
		"/project/src/guide/scaling-up/tooling.md": true,
	}

	got := ResolveMarkdownLinkPath(
		"/project/src/tutorial/description.md",
		"/guide/scaling-up/tooling.html",
		supplied)
	assert.Equal(t, []string{"/project/src/guide/scaling-up/tooling.md"}, got)
}

// A site-absolute link that doesn't match any supplied file must not produce
// a false match (e.g. a substring collision with an unrelated file).
func TestResolveMarkdownLinkPath_SiteAbsoluteNoMatch(t *testing.T) {
	supplied := map[string]bool{
		"/project/src/api/sfc-spec.md": true,
	}

	got := ResolveMarkdownLinkPath("/project/src/api/ssr.md", "/guide/missing/page", supplied)
	assert.Nil(t, got)
}
