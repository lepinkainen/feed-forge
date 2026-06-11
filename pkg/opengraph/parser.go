package opengraph

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/lepinkainen/feed-forge/pkg/urlutils"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

func extractOpenGraphTags(n *html.Node, data *Data) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "meta":
			processMetaTag(n, data)
		case "title":
			if data.Title == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				data.Title = strings.TrimSpace(n.FirstChild.Data)
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractOpenGraphTags(c, data)
	}
}

func processMetaTag(n *html.Node, data *Data) {
	property, content, name := metaTagAttrs(n)
	applyOpenGraphProperty(data, property, content)
	applyMetaFallback(data, name, content)
}

func metaTagAttrs(n *html.Node) (property, content, name string) {
	for _, attr := range n.Attr {
		switch attr.Key {
		case "property":
			property = attr.Val
		case "content":
			content = attr.Val
		case "name":
			name = attr.Val
		}
	}
	return property, content, name
}

func applyOpenGraphProperty(data *Data, property, content string) {
	switch property {
	case "og:title":
		if data.Title == "" {
			data.Title = content
		}
	case "og:description":
		if data.Description == "" {
			data.Description = content
		}
	case "og:image":
		if data.Image == "" {
			data.Image = content
		}
	case "og:site_name":
		if data.SiteName == "" {
			data.SiteName = content
		}
	}
}

func applyMetaFallback(data *Data, name, content string) {
	if data.Description == "" {
		switch name {
		case "description", "twitter:description":
			data.Description = content
		}
	}
	if data.Image == "" && name == "twitter:image" {
		data.Image = content
	}
	if data.Title == "" && name == "twitter:title" {
		data.Title = content
	}
}

func cleanupData(data *Data, baseURL string) {
	if len(data.Description) > 500 {
		data.Description = data.Description[:497] + "..."
	}
	if len(data.Title) > 200 {
		data.Title = data.Title[:197] + "..."
	}

	if data.Image != "" {
		resolvedURL, err := urlutils.ResolveURL(baseURL, data.Image)
		switch {
		case err != nil:
			slog.Warn("Failed to resolve image URL, clearing", "url", data.Image, "error", err)
			data.Image = ""
		case !urlutils.IsValidURL(resolvedURL):
			slog.Warn("Invalid image URL found after resolution, clearing", "original", data.Image, "resolved", resolvedURL)
			data.Image = ""
		default:
			data.Image = resolvedURL
		}
	}

	data.Title = strings.TrimSpace(data.Title)
	data.Description = strings.TrimSpace(data.Description)
	data.SiteName = strings.TrimSpace(data.SiteName)

	data.Title = strings.ReplaceAll(data.Title, "\x00", "")
	data.Description = strings.ReplaceAll(data.Description, "\x00", "")
	data.SiteName = strings.ReplaceAll(data.SiteName, "\x00", "")
}

func convertToUTF8(body []byte, contentType string) (string, error) {
	utf8Reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		slog.Warn("Failed to detect charset, assuming UTF-8", "error", err)
		return string(body), nil
	}

	utf8Bytes, err := io.ReadAll(utf8Reader)
	if err != nil {
		return "", fmt.Errorf("failed to convert to UTF-8: %w", err)
	}
	return string(utf8Bytes), nil
}
