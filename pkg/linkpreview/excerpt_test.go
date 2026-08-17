package linkpreview

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestBuildExcerptNeutralizesCDATATerminator guards the Atom feed against
// corruption: the excerpt is rendered raw inside <content type="html">
// <![CDATA[ ... ]]></content>, so an excerpt that emits the literal "]]>"
// would close the CDATA early and make the whole feed malformed XML for every
// subscriber. html.Render escapes ">" in text and attribute values, but it
// emits comment and raw-text (script/style) content verbatim, so those can
// still carry a raw terminator.
func TestBuildExcerptNeutralizesCDATATerminator(t *testing.T) {
	// Build a <body> the way go-trafilatura returns its ContentNode: a real
	// body element whose paragraph carries visible text plus an HTML comment
	// containing the CDATA terminator.
	body := &html.Node{Type: html.ElementNode, Data: "body"}
	p := &html.Node{Type: html.ElementNode, Data: "p"}
	p.AppendChild(&html.Node{Type: html.TextNode, Data: "visible paragraph text"})
	p.AppendChild(&html.Node{Type: html.CommentNode, Data: " array]]>end "})
	body.AppendChild(p)

	got := buildExcerpt(body, excerptBudget)
	if got == "" {
		t.Fatalf("buildExcerpt() = empty, want the paragraph")
	}
	if strings.Contains(got, "]]>") {
		t.Fatalf("buildExcerpt() emits a raw CDATA terminator, corrupting the feed: %q", got)
	}
}

// TestBuildExcerptFromParsedDocument covers the findBody fallback: a full
// html.Parse document is <html><head></head><body>...</body></html>. The
// body-finder must reach the real <body> element, not stop at the first
// childless node (the <head>) and return an empty excerpt.
func TestBuildExcerptFromParsedDocument(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(
		`<html><head></head><body><p>lead paragraph text</p></body></html>`,
	))
	if err != nil {
		t.Fatalf("html.Parse() error = %v", err)
	}
	got := buildExcerpt(doc, excerptBudget)
	if !strings.Contains(got, "lead paragraph text") {
		t.Fatalf("buildExcerpt(document) = %q, want the lead paragraph", got)
	}
}

// TestBuildExcerptFindsNestedParagraphs covers content whose paragraphs are
// wrapped in block elements rather than sitting as direct children of the
// body. The excerpt must still find them instead of coming back empty.
func TestBuildExcerptFindsNestedParagraphs(t *testing.T) {
	body := &html.Node{Type: html.ElementNode, Data: "body"}
	div := &html.Node{Type: html.ElementNode, Data: "div"}
	p := &html.Node{Type: html.ElementNode, Data: "p"}
	p.AppendChild(&html.Node{Type: html.TextNode, Data: "nested paragraph text"})
	div.AppendChild(p)
	body.AppendChild(div)

	got := buildExcerpt(body, excerptBudget)
	if !strings.Contains(got, "nested paragraph text") {
		t.Fatalf("buildExcerpt(nested) = %q, want the nested paragraph", got)
	}
}
