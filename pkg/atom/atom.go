// Package atom provides shared XML decoding and types for Atom providers.
// Providers retain their own entry fields and source-specific normalization.
package atom

import (
	"encoding"
	"encoding/xml"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/xmlutil"
)

// Feed is an Atom envelope with provider-specific entries.
//
// The XMLName requires the root <feed> element to be in the Atom namespace, as
// RFC 4287 mandates. Embedding Feed in a provider struct propagates this
// requirement, so a document with an un-namespaced root is rejected.
type Feed[E any] struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
	Entries []E      `xml:"entry"`
}

// Link identifies a related resource.
type Link struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

// AlternateHref returns the first non-empty href whose rel is "alternate" or
// empty, which Atom treats as alternate. It returns "" when none is present so
// callers can apply their own fallback.
func AlternateHref(links []Link) string {
	for _, l := range links {
		if (l.Rel == "alternate" || l.Rel == "") && l.Href != "" {
			return l.Href
		}
	}
	return ""
}

// Person identifies an Atom author or contributor.
type Person struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}

// Text is an Atom text construct. Text holds the decoded character data for
// type "text" and "html"; Inner holds the raw child XML that an "xhtml"
// construct carries instead.
type Text struct {
	Type  string `xml:"type,attr"`
	Text  string `xml:",chardata"`
	Inner string `xml:",innerxml"`
}

var xhtmlDivRegex = regexp.MustCompile(`(?s)\A\s*<div[^>]*>(.*)</div>\s*\z`)

// HTML returns the construct as HTML markup: "html" as-is, "xhtml" as its
// child markup without the mandatory wrapping <div>, and plain text escaped.
func (t Text) HTML() string {
	switch t.Type {
	case "html":
		return t.Text
	case "xhtml":
		if m := xhtmlDivRegex.FindStringSubmatch(t.Inner); m != nil {
			return strings.TrimSpace(m[1])
		}
		return strings.TrimSpace(t.Inner)
	default:
		return html.EscapeString(t.Text)
	}
}

// Category identifies a topic associated with an entry.
type Category struct {
	Term string `xml:"term,attr"`
}

// Time is a lenient Atom date construct. Decoding never fails: RFC 3339 with
// or without fractional seconds and the minute-precision form some feeds emit
// all parse, and anything else yields the zero time so one malformed entry
// cannot abort the whole feed. Callers check IsZero for the invalid case.
type Time struct{ time.Time }

var _ encoding.TextUnmarshaler = (*Time)(nil)

var timeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04Z07:00"}

// UnmarshalText implements encoding.TextUnmarshaler.
func (t *Time) UnmarshalText(text []byte) error {
	value := strings.TrimSpace(string(text))
	for _, layout := range timeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			t.Time = parsed
			return nil
		}
	}
	t.Time = time.Time{}
	return nil
}

// Decode reads an Atom document using its declared charset. It decodes XML
// entities once; HTML decoding and fallible metadata belong to each provider.
func Decode[T any](body []byte) (*T, error) {
	return xmlutil.Decode[T](body)
}
