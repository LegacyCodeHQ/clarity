package html

import (
	"context"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	tshtml "github.com/smacker/go-tree-sitter/html"
)

var (
	htmlLanguage   = tshtml.GetLanguage()
	htmlParserPool = sync.Pool{
		New: func() any {
			parser := sitter.NewParser()
			parser.SetLanguage(htmlLanguage)
			return parser
		},
	}
)

// linkAttributes are the HTML attributes whose values point at another file
// (`<a href>`, `<link href>`, `<img src>`, `<script src>`, `<iframe src>`).
var linkAttributes = map[string]bool{
	"href": true,
	"src":  true,
}

// HTMLLinkLocation is one file reference from an HTML element attribute.
type HTMLLinkLocation struct {
	Element   string
	Attribute string
	Path      string
	Line      int
}

// ParseHTMLLinks extracts file-reference destinations from `href` / `src`
// attributes in HTML source. Values are normalized: external URLs, anchors, and
// non-file references are dropped. Attribute parsing is driven by tree-sitter,
// so destinations inside comments or element text are never matched.
func ParseHTMLLinks(sourceCode []byte) []string {
	locations := ParseHTMLLinkLocations(sourceCode)
	var links []string
	seen := make(map[string]bool)
	for _, location := range locations {
		if seen[location.Path] {
			continue
		}
		seen[location.Path] = true
		links = append(links, location.Path)
	}
	return links
}

// ParseHTMLLinkLocations extracts element, attribute, destination, and 1-based
// source line for every project-file-like HTML reference.
func ParseHTMLLinkLocations(sourceCode []byte) []HTMLLinkLocation {
	tree, err := parseHTML(sourceCode)
	if err != nil {
		return nil
	}
	defer tree.Close()

	var links []HTMLLinkLocation
	for _, attr := range findNodesOfType(tree.RootNode(), "attribute") {
		name := strings.ToLower(attributeName(attr, sourceCode))
		if !linkAttributes[name] {
			continue
		}
		dest, ok := normalizeLink(attributeValue(attr, sourceCode))
		if !ok {
			continue
		}
		links = append(links, HTMLLinkLocation{
			Element:   htmlAttributeElement(attr, sourceCode),
			Attribute: name,
			Path:      dest,
			Line:      int(attr.StartPoint().Row) + 1,
		})
	}
	return links
}

func htmlAttributeElement(attr *sitter.Node, sourceCode []byte) string {
	if attr == nil {
		return ""
	}
	parent := attr.Parent()
	if parent == nil {
		return ""
	}
	if tag := findFirstChildOfType(parent, "tag_name"); tag != nil {
		return strings.ToLower(tag.Content(sourceCode))
	}
	return ""
}

// attributeName returns the lowercased name of an `attribute` node.
func attributeName(attr *sitter.Node, src []byte) string {
	if n := findFirstChildOfType(attr, "attribute_name"); n != nil {
		return n.Content(src)
	}
	return ""
}

// attributeValue returns the attribute's value with surrounding quotes removed,
// handling both quoted (`href="x"`) and unquoted (`href=x`) forms.
func attributeValue(attr *sitter.Node, src []byte) string {
	if q := findFirstChildOfType(attr, "quoted_attribute_value"); q != nil {
		if v := findFirstChildOfType(q, "attribute_value"); v != nil {
			return v.Content(src)
		}
		return "" // empty quoted value (href="")
	}
	if v := findFirstChildOfType(attr, "attribute_value"); v != nil {
		return v.Content(src)
	}
	return ""
}

// normalizeLink trims a destination and rejects values that cannot refer to a
// project file: empty/anchor-only values, external and protocol-relative URLs,
// and server-side template expressions (Hugo/Go templates, JSP, etc.).
func normalizeLink(dest string) (string, bool) {
	dest = strings.TrimSpace(dest)
	if dest == "" || strings.HasPrefix(dest, "#") {
		return "", false
	}
	if strings.HasPrefix(dest, "//") { // protocol-relative external URL
		return "", false
	}
	if strings.Contains(dest, "{{") || strings.Contains(dest, "${") {
		return "", false // unresolved template expression
	}
	if idx := strings.IndexAny(dest, "#?"); idx >= 0 {
		dest = dest[:idx]
	}
	if dest == "" {
		return "", false
	}
	lower := strings.ToLower(dest)
	for _, scheme := range []string{
		"http://", "https://", "mailto:", "tel:", "ftp://", "ftps://",
		"file://", "data:", "javascript:",
	} {
		if strings.HasPrefix(lower, scheme) {
			return "", false
		}
	}
	return dest, true
}

func parseHTML(sourceCode []byte) (*sitter.Tree, error) {
	parser, _ := htmlParserPool.Get().(*sitter.Parser)
	if parser == nil {
		parser = sitter.NewParser()
		parser.SetLanguage(htmlLanguage)
	}
	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
	htmlParserPool.Put(parser)
	return tree, err
}

func findFirstChildOfType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type() == nodeType {
			return child
		}
	}
	return nil
}

func findNodesOfType(node *sitter.Node, nodeType string) []*sitter.Node {
	if node == nil {
		return nil
	}
	var nodes []*sitter.Node
	if node.Type() == nodeType {
		nodes = append(nodes, node)
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		nodes = append(nodes, findNodesOfType(node.NamedChild(i), nodeType)...)
	}
	return nodes
}
