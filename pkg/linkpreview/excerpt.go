package linkpreview

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// excerptBudget is the soft character ceiling for the paragraph excerpt gate.
const excerptBudget = 500

// findBody returns the <body> element within a node tree, or the node itself
// if none is found.
func findBody(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	if body := findBodyElement(n); body != nil {
		return body
	}
	return n
}

// findBodyElement returns the first <body> element in the tree, or nil if none
// exists. Unlike findBody it never substitutes an arbitrary node, so the search
// does not stop early at the first childless node it recurses into.
func findBodyElement(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := findBodyElement(c); b != nil {
			return b
		}
	}
	return nil
}

// bodyParagraphs returns the <p> elements within the body, in document order,
// including paragraphs nested inside block wrappers such as <div> or <article>.
// It does not descend into a <p> it has already collected.
func bodyParagraphs(root *html.Node) []*html.Node {
	body := findBody(root)
	if body == nil {
		return nil
	}
	var ps []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "p" {
				ps = append(ps, c)
				continue
			}
			walk(c)
		}
	}
	walk(body)
	return ps
}

// nodeText returns the trimmed concatenated text content of a node.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

// buildExcerpt applies the dual gate: accumulate whole <p> paragraphs while the
// cumulative character count stays under budget, never cutting mid-paragraph,
// always keeping at least the first non-empty paragraph. Empty paragraphs are
// skipped. Returns the rendered HTML of the chosen paragraphs.
func buildExcerpt(root *html.Node, budget int) string {
	var chosen []*html.Node
	total := 0
	for _, p := range bodyParagraphs(root) {
		t := len(nodeText(p))
		if t == 0 {
			continue // skip blank paragraphs
		}
		if len(chosen) > 0 && total+t > budget {
			break // soft ceiling: stop before exceeding, min one paragraph
		}
		chosen = append(chosen, p)
		total += t
	}
	var buf bytes.Buffer
	for _, p := range chosen {
		_ = html.Render(&buf, p)
	}
	// The excerpt is dropped raw into a <content type="html"><![CDATA[ ... ]]>
	// block. html.Render escapes ">" in text and attributes, but emits comment
	// and raw-text (script/style) content verbatim, so a stray "]]>" there would
	// close the CDATA early and make the whole feed malformed XML. Break the
	// terminator; an HTML feed reader decodes "&gt;" back to ">".
	return strings.ReplaceAll(buf.String(), "]]>", "]]&gt;")
}
