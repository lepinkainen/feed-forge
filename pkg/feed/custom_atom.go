package feed

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/feeds"
)

// CustomAtomCategory represents a category in Atom feed
type CustomAtomCategory struct {
	XMLName xml.Name `xml:"category"`
	Term    string   `xml:"term,attr"`
	Label   string   `xml:"label,attr,omitempty"`
	Scheme  string   `xml:"scheme,attr,omitempty"`
}

// CustomAtomEntry represents an entry in a custom Atom feed
type CustomAtomEntry struct {
	XMLName    xml.Name             `xml:"entry"`
	Xmlns      string               `xml:"xmlns,attr,omitempty"`
	Title      string               `xml:"title"`
	Updated    string               `xml:"updated"`
	Id         string               `xml:"id"`
	Categories []CustomAtomCategory `xml:"category"`
	Content    *feeds.AtomContent   `xml:"content,omitempty"`
	Rights     string               `xml:"rights,omitempty"`
	Source     string               `xml:"source,omitempty"`
	Published  string               `xml:"published,omitempty"`
	Links      []feeds.AtomLink     `xml:"link"`
	Summary    *feeds.AtomSummary   `xml:"summary,omitempty"`
	Author     *feeds.AtomAuthor    `xml:"author,omitempty"`
}

// CustomAtomFeed represents a custom Atom feed with category support
type CustomAtomFeed struct {
	XMLName  xml.Name           `xml:"feed"`
	Xmlns    string             `xml:"xmlns,attr"`
	Title    string             `xml:"title"`
	Id       string             `xml:"id"`
	Updated  string             `xml:"updated"`
	Link     *feeds.AtomLink    `xml:"link,omitempty"`
	Author   *feeds.AtomAuthor  `xml:"author,omitempty"`
	Subtitle string             `xml:"subtitle,omitempty"`
	Rights   string             `xml:"rights,omitempty"`
	Entries  []*CustomAtomEntry `xml:"entry"`
}

// GenerateCustomAtom creates a custom Atom feed with category support
func (g *Generator) GenerateCustomAtom(items []Item, itemCategories map[string][]string) (string, error) {
	// Create a standard feed first
	feed, err := g.Generate(items, Atom)
	if err != nil {
		return "", fmt.Errorf("failed to generate base feed: %w", err)
	}

	// Convert to custom Atom
	customFeed := g.convertToCustomAtom(feed, itemCategories)

	// Convert to XML
	xmlData, err := xml.MarshalIndent(customFeed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal custom atom feed: %w", err)
	}

	// Add XML header
	return xml.Header + string(xmlData), nil
}

// convertToCustomAtom converts a standard Feed to a CustomAtomFeed with proper categories
func (g *Generator) convertToCustomAtom(feed *feeds.Feed, itemCategories map[string][]string) *CustomAtomFeed {
	atom := &feeds.Atom{Feed: feed}
	standardAtomFeed := atom.AtomFeed()

	customFeed := &CustomAtomFeed{
		Xmlns:    "http://www.w3.org/2005/Atom",
		Title:    standardAtomFeed.Title,
		Id:       standardAtomFeed.Id,
		Updated:  standardAtomFeed.Updated,
		Link:     standardAtomFeed.Link,
		Author:   standardAtomFeed.Author,
		Subtitle: standardAtomFeed.Subtitle,
		Rights:   standardAtomFeed.Rights,
	}

	// Convert entries with categories
	for _, entry := range standardAtomFeed.Entries {
		customEntry := &CustomAtomEntry{
			Title:     entry.Title,
			Updated:   entry.Updated,
			Id:        entry.Id,
			Content:   entry.Content,
			Rights:    entry.Rights,
			Source:    entry.Source,
			Published: entry.Published,
			Links:     entry.Links,
			Summary:   entry.Summary,
			Author:    entry.Author,
		}

		// Add categories for this entry
		if categories, exists := itemCategories[entry.Id]; exists {
			for _, cat := range categories {
				customEntry.Categories = append(customEntry.Categories, CustomAtomCategory{
					Term:  cat,
					Label: cat,
				})
			}
		}

		customFeed.Entries = append(customFeed.Entries, customEntry)
	}

	return customFeed
}

// SaveCustomAtomToFile saves a custom Atom feed to a file
func (g *Generator) SaveCustomAtomToFile(items []Item, itemCategories map[string][]string, outputPath string) error {
	// Generate custom Atom content
	atomContent, err := g.GenerateCustomAtom(items, itemCategories)
	if err != nil {
		return fmt.Errorf("failed to generate custom atom feed: %w", err)
	}

	// Ensure output directory exists
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(atomContent)
	if err != nil {
		return fmt.Errorf("failed to write custom atom feed: %w", err)
	}

	return nil
}

// GenerateEnhancedAtom creates an enhanced Atom feed with custom namespace support
func (g *Generator) GenerateEnhancedAtom(items []Item, customNamespace string) (string, error) {
	now := time.Now()

	var atom strings.Builder
	atom.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)

	// Add custom namespace if provided
	if customNamespace != "" {
		atom.WriteString(fmt.Sprintf(`<feed xmlns="http://www.w3.org/2005/Atom" xmlns:custom="%s">`, customNamespace))
	} else {
		atom.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom">`)
	}

	atom.WriteString(fmt.Sprintf(`<title>%s</title>`, EscapeXML(g.Title)))
	atom.WriteString(fmt.Sprintf(`<link href="%s"/>`, EscapeXML(g.Link)))
	atom.WriteString(fmt.Sprintf(`<id>%s</id>`, EscapeXML(g.Link)))
	atom.WriteString(fmt.Sprintf(`<updated>%s</updated>`, now.Format(time.RFC3339)))
	atom.WriteString(fmt.Sprintf(`<author><name>%s</name></author>`, EscapeXML(g.Author)))
	atom.WriteString(fmt.Sprintf(`<subtitle>%s</subtitle>`, EscapeXML(g.Description)))

	for _, item := range items {
		atom.WriteString(`<entry>`)
		atom.WriteString(fmt.Sprintf(`<title>%s</title>`, EscapeXML(item.Title)))
		atom.WriteString(fmt.Sprintf(`<link rel="alternate" type="text/html" href="%s"/>`, EscapeXML(item.Link)))
		atom.WriteString(fmt.Sprintf(`<id>%s</id>`, EscapeXML(item.ID)))
		atom.WriteString(fmt.Sprintf(`<updated>%s</updated>`, item.Created.Format(time.RFC3339)))
		atom.WriteString(fmt.Sprintf(`<published>%s</published>`, item.Created.Format(time.RFC3339)))
		atom.WriteString(fmt.Sprintf(`<author><name>%s</name></author>`, EscapeXML(item.Author)))

		// Add categories
		for _, category := range item.Categories {
			atom.WriteString(fmt.Sprintf(`<category term="%s" label="%s"/>`, EscapeXML(category), EscapeXML(category)))
		}

		atom.WriteString(fmt.Sprintf(`<content type="html">%s</content>`, EscapeXML(item.Description)))
		atom.WriteString(`</entry>`)
	}

	atom.WriteString(`</feed>`)
	return atom.String(), nil
}

// SaveEnhancedAtomToFile saves an enhanced Atom feed to a file
func (g *Generator) SaveEnhancedAtomToFile(items []Item, customNamespace, outputPath string) error {
	// Generate enhanced Atom content
	atomContent, err := g.GenerateEnhancedAtom(items, customNamespace)
	if err != nil {
		return fmt.Errorf("failed to generate enhanced atom feed: %w", err)
	}

	// Ensure output directory exists
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(atomContent)
	if err != nil {
		return fmt.Errorf("failed to write enhanced atom feed: %w", err)
	}

	return nil
}
