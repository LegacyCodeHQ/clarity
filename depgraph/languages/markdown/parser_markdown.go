package markdown

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
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

var (
	// inlineLinkRe matches `[text](dest)` and `![alt](dest)`. The dest captures
	// up to the next whitespace or closing paren so titles (`"..."`) are dropped.
	inlineLinkRe = regexp.MustCompile(`(!?)\[[^\]]*\]\(\s*<?([^)\s>]+)>?[^)]*\)`)
	// referenceDefRe matches `[label]: dest` at line start (after optional spaces).
	referenceDefRe = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*<?([^\s>]+)>?`)
	// autolinkRe matches `<dest>` autolinks containing a path-like value.
	autolinkRe = regexp.MustCompile(`<([^>\s]+\.[A-Za-z0-9]+)>`)
)

// MarkdownLinks reads a Markdown file and returns its extracted link targets.
func MarkdownLinks(filePath string) ([]MarkdownLink, error) {
	sourceCode, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return ParseMarkdownLinks(sourceCode), nil
}

// ParseMarkdownLinks extracts link, image, and reference-definition targets
// from Markdown source, skipping fenced code blocks and inline code spans.
func ParseMarkdownLinks(sourceCode []byte) []MarkdownLink {
	var links []MarkdownLink

	scanner := bufio.NewScanner(bytes.NewReader(sourceCode))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inFence := false
	var fenceMarker string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimLeft(line, " \t")

		if inFence {
			if strings.HasPrefix(trimmed, fenceMarker) {
				inFence = false
				fenceMarker = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			inFence = true
			fenceMarker = "```"
			continue
		}
		if strings.HasPrefix(trimmed, "~~~") {
			inFence = true
			fenceMarker = "~~~"
			continue
		}

		stripped := stripInlineCode(line)

		if m := referenceDefRe.FindStringSubmatch(stripped); m != nil {
			if link, ok := normalizeLink(m[1], false); ok {
				links = append(links, link)
			}
			continue
		}

		for _, m := range inlineLinkRe.FindAllStringSubmatch(stripped, -1) {
			isImage := m[1] == "!"
			if link, ok := normalizeLink(m[2], isImage); ok {
				links = append(links, link)
			}
		}

		for _, m := range autolinkRe.FindAllStringSubmatch(stripped, -1) {
			if link, ok := normalizeLink(m[1], false); ok {
				links = append(links, link)
			}
		}
	}

	return links
}

// stripInlineCode removes backtick-delimited spans from a line so links inside
// inline code aren't extracted.
func stripInlineCode(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	inCode := false
	for i := 0; i < len(line); i++ {
		if line[i] == '`' {
			inCode = !inCode
			continue
		}
		if !inCode {
			b.WriteByte(line[i])
		}
	}
	return b.String()
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
