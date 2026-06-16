package html

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHTMLLinks_ExtractsHrefAndSrc(t *testing.T) {
	source := `
<!DOCTYPE html>
<html>
  <head>
    <link href="css/app.css" rel="stylesheet">
    <script src="js/main.js"></script>
  </head>
  <body>
    <a href="./about.html">About</a>
    <img src="img/logo.svg" alt="logo">
    <iframe src="embed/widget.html"></iframe>
  </body>
</html>
`
	links := ParseHTMLLinks([]byte(source))

	assert.Contains(t, links, "css/app.css")
	assert.Contains(t, links, "js/main.js")
	assert.Contains(t, links, "./about.html")
	assert.Contains(t, links, "img/logo.svg")
	assert.Contains(t, links, "embed/widget.html")
}

func TestParseHTMLLinks_SkipsExternalAnchorAndTemplate(t *testing.T) {
	source := `
<a href="https://example.com">external</a>
<a href="//cdn.example.com/lib.js">protocol-relative</a>
<a href="mailto:foo@bar.com">mail</a>
<a href="#section">anchor</a>
<a href="javascript:void(0)">js</a>
<link href="{{ .Site.BaseURL }}css/site.css" rel="stylesheet">
<img src="${cdn}/logo.png">
<a href="./real.html">local</a>
`
	links := ParseHTMLLinks([]byte(source))

	assert.NotContains(t, links, "https://example.com")
	assert.NotContains(t, links, "//cdn.example.com/lib.js")
	assert.NotContains(t, links, "mailto:foo@bar.com")
	assert.NotContains(t, links, "#section")
	assert.NotContains(t, links, "javascript:void(0)")
	for _, l := range links {
		assert.NotContains(t, l, "{{")
		assert.NotContains(t, l, "${")
	}
	assert.Contains(t, links, "./real.html")
}

// Fragments and query strings are stripped so the link resolves to the
// underlying file (e.g. `page.html#anchor` -> `page.html`).
func TestParseHTMLLinks_StripsFragmentAndQuery(t *testing.T) {
	source := `<a href="docs/guide.html#install">g</a><a href="search.html?q=x">s</a>`
	links := ParseHTMLLinks([]byte(source))

	assert.Contains(t, links, "docs/guide.html")
	assert.Contains(t, links, "search.html")
}

// Destinations that merely look like attributes but live inside comments or
// element text must not be extracted; tree-sitter scopes attribute parsing to
// real tags.
func TestParseHTMLLinks_IgnoresCommentsAndText(t *testing.T) {
	source := `
<!-- <a href="commented.html">x</a> -->
<p>Write href="text.html" inline and it is not a link.</p>
<a href="real.html">real</a>
`
	links := ParseHTMLLinks([]byte(source))

	assert.NotContains(t, links, "commented.html")
	assert.NotContains(t, links, "text.html")
	assert.Contains(t, links, "real.html")
}

func TestParseHTMLLinks_EmptyAndMissingValues(t *testing.T) {
	source := `<a href="">empty</a><img src>`
	links := ParseHTMLLinks([]byte(source))

	assert.Empty(t, links)
}
