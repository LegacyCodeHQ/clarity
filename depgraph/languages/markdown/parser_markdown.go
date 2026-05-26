package markdown

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	tsmd "github.com/smacker/go-tree-sitter/markdown"
)

// MarkdownLink represents a link, image, or reference-definition target
// extracted from a Markdown file.
type MarkdownLink struct {
	path    string
	isImage bool
}

// Path returns the raw destination text from the Markdown source, with any
// URL fragment, query string, or surrounding quotes stripped.
func (l MarkdownLink) Path() string {
	return l.path
}

// IsImage reports whether the link came from an image syntax (`![alt](path)`).
func (l MarkdownLink) IsImage() bool {
	return l.isImage
}

// htmlAttrRe extracts `src="..."` or `href="..."` from the raw text of an
// `html_tag` node. Tree-sitter delineates HTML tag boundaries but does not
// expose attribute names/values as named children, so we still need a small
// regex to pull the destination out. Scoping by the html_tag node guarantees
// we never read attributes from inside code blocks or other syntax.
var htmlAttrRe = regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// MarkdownLinks reads a Markdown file and returns its extracted link targets.
func MarkdownLinks(filePath string) ([]MarkdownLink, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return ParseMarkdownLinks(sourceCode), nil
}

// ParseMarkdownLinks extracts link, image, and reference-definition targets
// from Markdown source using tree-sitter. Code spans, fenced code blocks, and
// other non-link content are correctly excluded by the grammar.
func ParseMarkdownLinks(sourceCode []byte) []MarkdownLink {
	tree, err := tsmd.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil
	}

	var links []MarkdownLink
	add := func(dest string, isImage bool) {
		if link, ok := normalizeLink(dest, isImage); ok {
			links = append(links, link)
		}
	}

	walkBlocks(tree.BlockTree().RootNode(), sourceCode, add)
	for _, inline := range tree.InlineTrees() {
		walkInline(inline.RootNode(), sourceCode, add)
	}

	return links
}

// walkBlocks walks the block-level tree to extract destinations from
// reference-definitions (`[label]: dest`). Inline-level constructs are handled
// in a separate pass over the inline trees.
func walkBlocks(n *sitter.Node, src []byte, add func(string, bool)) {
	if n == nil {
		return
	}
	switch n.Type() {
	case "link_reference_definition":
		if dest := findChildText(n, "link_destination", src); dest != "" {
			add(dest, false)
		}
		return
	case "html_block":
		extractHTMLAttrs(string(src[n.StartByte():n.EndByte()]), add)
		return
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		walkBlocks(n.Child(i), src, add)
	}
}

// extractHTMLAttrs pulls `src` / `href` attribute values from a raw HTML
// fragment. Used for both block-level `html_block` and inline-level `html_tag`
// nodes, since tree-sitter does not parse HTML attributes into named children.
func extractHTMLAttrs(html string, add func(string, bool)) {
	isImage := htmlFragmentIsImage(html)
	for _, m := range htmlAttrRe.FindAllStringSubmatch(html, -1) {
		value := m[1]
		if value == "" {
			value = m[2]
		}
		if value != "" {
			add(value, isImage)
		}
	}
}

// htmlFragmentIsImage reports whether the first tag in an HTML fragment is an
// `<img>` element, so the resulting link is flagged as an image.
func htmlFragmentIsImage(html string) bool {
	trimmed := strings.TrimLeft(html, " \t\n<")
	return strings.HasPrefix(strings.ToLower(trimmed), "img")
}

// walkInline walks an inline tree to extract destinations from images, inline
// links, and `src` / `href` attributes inside `html_tag` nodes.
func walkInline(n *sitter.Node, src []byte, add func(string, bool)) {
	if n == nil {
		return
	}
	switch n.Type() {
	case "image":
		if dest := findChildText(n, "link_destination", src); dest != "" {
			add(dest, true)
		}
		return
	case "inline_link":
		if dest := findChildText(n, "link_destination", src); dest != "" {
			add(dest, false)
		}
		return
	case "html_tag":
		extractHTMLAttrs(string(src[n.StartByte():n.EndByte()]), add)
		return
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		walkInline(n.Child(i), src, add)
	}
}

// findChildText returns the text of the first descendant matching nodeType,
// or "" if none exists.
func findChildText(n *sitter.Node, nodeType string, src []byte) string {
	if n == nil {
		return ""
	}
	if n.Type() == nodeType {
		return string(src[n.StartByte():n.EndByte()])
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		if result := findChildText(n.Child(i), nodeType, src); result != "" {
			return result
		}
	}
	return ""
}

// normalizeLink trims a destination string and rejects values that can't refer
// to a project file (external URLs, bare anchors, mail/tel schemes).
func normalizeLink(dest string, isImage bool) (MarkdownLink, bool) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return MarkdownLink{}, false
	}
	if idx := strings.IndexAny(dest, "#?"); idx >= 0 {
		dest = dest[:idx]
	}
	if dest == "" {
		return MarkdownLink{}, false
	}
	lower := strings.ToLower(dest)
	for _, scheme := range []string{"http://", "https://", "mailto:", "tel:", "ftp://", "ftps://", "file://", "data:"} {
		if strings.HasPrefix(lower, scheme) {
			return MarkdownLink{}, false
		}
	}
	return MarkdownLink{path: dest, isImage: isImage}, true
}
