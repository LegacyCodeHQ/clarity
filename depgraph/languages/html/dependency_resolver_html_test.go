package html

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveHTMLLinkPath_RelativeFile(t *testing.T) {
	supplied := map[string]bool{
		"/project/site/about.html":   true,
		"/project/site/css/app.css":  true,
		"/project/site/img/logo.svg": true,
	}

	got := ResolveHTMLLinkPath("/project/site/index.html", "./about.html", supplied)
	assert.Equal(t, []string{"/project/site/about.html"}, got)

	got = ResolveHTMLLinkPath("/project/site/index.html", "css/app.css", supplied)
	assert.Equal(t, []string{filepath.Clean("/project/site/css/app.css")}, got)
}

func TestResolveHTMLLinkPath_ParentRelative(t *testing.T) {
	supplied := map[string]bool{
		"/project/site/shared/base.css": true,
	}

	got := ResolveHTMLLinkPath("/project/site/pages/index.html", "../shared/base.css", supplied)
	assert.Equal(t, []string{filepath.Clean("/project/site/shared/base.css")}, got)
}

// Server-absolute links ("/css/app.css") don't share the filesystem root, so
// they are resolved by unique suffix match against the supplied files.
func TestResolveHTMLLinkPath_ServerAbsolute(t *testing.T) {
	supplied := map[string]bool{
		"/project/site/css/app.css": true,
		"/project/site/index.html":  true,
	}

	got := ResolveHTMLLinkPath("/project/site/index.html", "/css/app.css", supplied)
	assert.Equal(t, []string{"/project/site/css/app.css"}, got)
}

func TestResolveHTMLLinkPath_ServerAbsoluteNoMatch(t *testing.T) {
	supplied := map[string]bool{
		"/project/site/index.html": true,
	}

	got := ResolveHTMLLinkPath("/project/site/index.html", "/org/documents/epl-v10.html", supplied)
	assert.Nil(t, got)
}

// An ambiguous server-absolute suffix (same path served from two roots) must
// not fan out to both files.
func TestResolveHTMLLinkPath_ServerAbsoluteAmbiguous(t *testing.T) {
	supplied := map[string]bool{
		"/project/a/css/app.css": true,
		"/project/b/css/app.css": true,
	}

	got := ResolveHTMLLinkPath("/project/a/index.html", "/css/app.css", supplied)
	assert.Nil(t, got)
}

func TestResolveHTMLLinkPath_MissingTarget(t *testing.T) {
	supplied := map[string]bool{
		"/project/site/index.html": true,
	}

	got := ResolveHTMLLinkPath("/project/site/index.html", "./missing.html", supplied)
	assert.Nil(t, got)
}

// A link that resolves back to the source file must not create a self-edge.
func TestResolveHTMLLinkPath_SelfReferenceFiltered(t *testing.T) {
	supplied := map[string]bool{
		"/project/site/index.html": true,
	}

	got := ResolveHTMLLinkPath("/project/site/index.html", "./index.html", supplied)
	assert.Nil(t, got)
}
