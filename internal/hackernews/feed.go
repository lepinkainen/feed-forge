package hackernews

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	"github.com/gorilla/feeds"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
)

// Use shared Atom types from pkg/feed
type AtomCategory = feed.AtomCategory
type CustomAtomEntry = feed.CustomAtomEntry
type CustomAtomFeed = feed.CustomAtomFeed

// generateRSSFeed creates an Atom RSS feed from the provided items with OpenGraph data
func generateRSSFeed(db *sql.DB, ogDB *opengraph.Database, items []HackerNewsItem, minPoints int, categoryMapper *CategoryMapper) string {
	slog.Debug("Generating RSS feed", "itemCount", len(items))
	now := time.Now()

	feedObj := &feeds.Feed{
		Title:       "Hacker News Top Stories",
		Description: "High-quality Hacker News stories, updated regularly",
		Link:        &feeds.Link{Href: "https://news.ycombinator.com/", Rel: "self", Type: "text/html"},
		Id:          "tag:news.ycombinator.com,2024:feed",
		Created:     now,
		Updated:     now,
	}

	// Track categories for each item (using CommentsLink as the ID)
	itemCategories := make(map[string][]string)

	domainRegex := regexp.MustCompile(`^https?://([^/]+)`)

	// Initialize OpenGraph fetcher
	ogFetcher := opengraph.NewFetcher(ogDB)
	slog.Debug("Initialized OpenGraph fetcher")

	// Collect all URLs that need OpenGraph data
	var urlsToFetch []string
	for _, item := range items {
		if item.ItemLink != "" {
			urlsToFetch = append(urlsToFetch, item.ItemLink)
		}
	}

	// Fetch OpenGraph data concurrently for all URLs
	slog.Debug("Fetching OpenGraph data concurrently", "urlCount", len(urlsToFetch))
	ogDataMap := ogFetcher.FetchConcurrent(urlsToFetch)
	slog.Debug("Completed concurrent OpenGraph fetching")

	for i := range items {
		item := &items[i]
		// Extract domain from the article link
		domain := ""
		if matches := domainRegex.FindStringSubmatch(item.ItemLink); len(matches) > 1 {
			domain = matches[1]
		}

		// Generate categories
		categories := categorizeContent(item.ItemTitle, domain, item.ItemLink, categoryMapper)
		pointCategory := categorizeByPoints(item.Points, minPoints)
		categories = append(categories, pointCategory)

		// Populate the item's Domain and Categories fields for the FeedItem interface
		item.Domain = domain
		item.ItemCategories = categories

		// Calculate post age
		postAge := calculatePostAge(item.ItemCreatedAt)

		// Calculate engagement ratio
		engagementRatio := float64(item.ItemCommentCount) / float64(item.Points)
		engagementText := ""
		if engagementRatio > 0.5 {
			engagementText = "🔥 High engagement"
		} else if engagementRatio > 0.3 {
			engagementText = "💬 Good discussion"
		}

		// Get pre-fetched OpenGraph data for the article
		var ogPreview string
		if item.ItemLink != "" {
			ogData := ogDataMap[item.ItemLink]
			if ogData != nil && (ogData.Title != "" || ogData.Description != "") {
				ogPreview = fmt.Sprintf(`<div style="margin-bottom: 16px; padding: 12px; background: #f9f9f9; border-radius: 6px; border-left: 3px solid #007acc;">
					<h4 style="margin: 0 0 8px 0; color: #007acc; font-size: 14px;">📄 Article Preview</h4>
					%s
					%s
					%s
				</div>`,
					func() string {
						if ogData.Title != "" && ogData.Title != item.ItemTitle {
							return fmt.Sprintf(`<p style="margin: 0 0 6px 0; font-weight: bold; color: #333;">%s</p>`, ogData.Title)
						}
						return ""
					}(),
					func() string {
						if ogData.Description != "" {
							return fmt.Sprintf(`<p style="margin: 0 0 6px 0; color: #666; line-height: 1.4; font-size: 13px;">%s</p>`, ogData.Description)
						}
						return ""
					}(),
					func() string {
						if ogData.Image != "" {
							return fmt.Sprintf(`<img src="%s" alt="Article image" style="max-width: 100%%; height: auto; border-radius: 4px; margin-top: 8px;" loading="lazy">`, ogData.Image)
						}
						return ""
					}())
			}
		}

		// Enhanced HTML description with categories
		categoryTags := ""
		if len(categories) > 0 {
			categoryTags = "<div style=\"margin-bottom: 8px; line-height: 1.8;\">"
			for i, cat := range categories {
				// Add space between tags for better RSS reader compatibility
				if i > 0 {
					categoryTags += " "
				}
				categoryTags += fmt.Sprintf("<span style=\"display: inline-block; background: #e5e5e5; color: #666; padding: 3px 8px; border-radius: 12px; font-size: 12px; margin-right: 6px; margin-bottom: 2px; white-space: nowrap;\">%s</span>", cat)
			}
			categoryTags += "</div>"
		}

		description := fmt.Sprintf(`<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.5;">
			<div style="margin-bottom: 12px; padding: 8px; background-color: #f6f6ef; border-left: 4px solid #ff6600;">
				<strong style="color: #ff6600;">%d points</strong> • 
				<strong style="color: #666;">%d comments</strong> • 
				<span style="color: #828282;">%s</span>
				%s
			</div>
			
			%s
			
			%s
			
			<div style="margin-bottom: 8px;">
				<strong>Source:</strong> <code style="background: #f4f4f4; padding: 2px 4px; border-radius: 3px;">%s</code>
			</div>
			
			<div style="margin-bottom: 12px;">
				<strong>Author:</strong> <span style="color: #666;">%s</span>
			</div>
			
			<div style="margin-top: 16px; padding-top: 12px; border-top: 1px solid #e5e5e5;">
				<a href="%s" style="display: inline-block; padding: 6px 12px; background-color: #ff6600; color: white; text-decoration: none; border-radius: 4px; margin-right: 8px;">💬 HN Discussion</a>
				<a href="%s" style="display: inline-block; padding: 6px 12px; background-color: #666; color: white; text-decoration: none; border-radius: 4px;">📖 Read Article</a>
			</div>
		</div>`,
			item.Points,
			item.ItemCommentCount,
			postAge,
			func() string {
				if engagementText != "" {
					return " • " + engagementText
				}
				return ""
			}(),
			categoryTags,
			ogPreview,
			domain,
			item.ItemAuthor,
			item.ItemCommentsLink,
			item.ItemLink)

		rssItem := &feeds.Item{
			Title: item.ItemTitle,
			Link:  &feeds.Link{Href: item.ItemCommentsLink, Rel: "alternate", Type: "text/html"},
			Id:    item.ItemCommentsLink,
			Author: &feeds.Author{
				Name: item.ItemAuthor,
			},
			Description: description,
			Created:     item.ItemCreatedAt,
		}

		// Store categories for this item (using the same ID as the rssItem)
		itemCategories[item.ItemCommentsLink] = categories

		feedObj.Items = append(feedObj.Items, rssItem)
	}

	// Generate custom Atom feed with proper categories
	customAtomFeed := feed.ConvertToCustomAtom(feedObj, itemCategories)

	// Convert to XML
	xmlData, err := xml.MarshalIndent(customAtomFeed, "", "  ")
	if err != nil {
		slog.Error("Failed to generate RSS feed", "error", err)
		os.Exit(1)
	}

	// Add XML header
	rss := xml.Header + string(xmlData)

	slog.Debug("RSS feed generated successfully", "feedSize", len(rss))
	return rss
}
