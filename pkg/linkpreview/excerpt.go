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
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := findBody(c); b != nil {
			return b
		}
	}
	return n
}

// topParagraphs returns the direct-child <p> elements of the body, in order.
func topParagraphs(root *html.Node) []*html.Node {
	body := findBody(root)
	if body == nil {
		return nil
	}
	var ps []*html.Node
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "p" {
			ps = append(ps, c)
		}
	}
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
	for _, p := range topParagraphs(root) {
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
	return buf.String()
}
