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

// hugoRefRe matches Hugo's `relref`/`ref` link shortcodes, e.g.
// `{{< relref "path/to/page.md" >}}` or `{{% ref 'page.md' %}}`. Authors write
// these inside Markdown link destinations (`[text]({{< relref "x.md" >}})`),
// where tree-sitter parses the destination as `{{<` and never exposes the
// quoted path, so a dedicated pass is needed to recover it.
var hugoRefRe = regexp.MustCompile(`(?i)\{\{[<%]\s*(?:relref|ref)\b\s+(?:"([^"]*)"|'([^']*)')`)

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

	addHugoRefLinks(sourceCode, tree, add)

	return links
}

// addHugoRefLinks recovers link targets from Hugo `relref`/`ref` shortcodes,
// which the Markdown grammar hides inside link destinations. Matches that fall
// inside code spans or fenced/indented code blocks are skipped so documentation
// that demonstrates the shortcode syntax does not produce spurious edges.
func addHugoRefLinks(src []byte, tree *tsmd.MarkdownTree, add func(string, bool)) {
	codeRanges := collectCodeRanges(tree)
	for _, m := range hugoRefRe.FindAllSubmatchIndex(src, -1) {
		if withinRanges(uint32(m[0]), codeRanges) {
			continue
		}
		path := submatch(src, m, 1)
		if path == "" {
			path = submatch(src, m, 2)
		}
		if path != "" {
			add(normalizeHugoRefPath(path), false)
		}
	}
}

// normalizeHugoRefPath converts a shortcode path to a form the resolver
// understands. Explicitly relative (`./`, `../`) and absolute (`/`) paths are
// left untouched; a bare content path (`docs/page.md`) is content-root relative
// in Hugo, so it is routed through site-absolute resolution with a leading `/`.
func normalizeHugoRefPath(path string) string {
	if strings.HasPrefix(path, "/") ||
		strings.HasPrefix(path, "./") ||
		strings.HasPrefix(path, "../") {
		return path
	}
	return "/" + path
}

// submatch returns the text of capture group n from a FindAllSubmatchIndex
// match, or "" if the group did not participate.
func submatch(src []byte, m []int, n int) string {
	start, end := m[2*n], m[2*n+1]
	if start < 0 || end < 0 {
		return ""
	}
	return string(src[start:end])
}

// collectCodeRanges returns the byte ranges of code spans and fenced/indented
// code blocks across the block and inline trees, used to exclude shortcode
// matches that appear inside code.
func collectCodeRanges(tree *tsmd.MarkdownTree) [][2]uint32 {
	var ranges [][2]uint32
	collect := func(n *sitter.Node) {
		var walk func(*sitter.Node)
		walk = func(node *sitter.Node) {
			if node == nil {
				return
			}
			switch node.Type() {
			case "fenced_code_block", "indented_code_block", "code_span":
				ranges = append(ranges, [2]uint32{node.StartByte(), node.EndByte()})
			}
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i))
			}
		}
		walk(n)
	}
	collect(tree.BlockTree().RootNode())
	for _, inline := range tree.InlineTrees() {
		collect(inline.RootNode())
	}
	return ranges
}

// withinRanges reports whether offset falls inside any of the given byte ranges.
func withinRanges(offset uint32, ranges [][2]uint32) bool {
	for _, r := range ranges {
		if offset >= r[0] && offset < r[1] {
			return true
		}
	}
	return false
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
