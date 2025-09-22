package hackernews

import (
	"fmt"
	"strings"
)

// categorizeContent analyzes content and returns applicable categories based on domain and title
func categorizeContent(title, domain, url string, categoryMapper *CategoryMapper) []string {
	var categories []string

	// Add the raw domain as a category first
	if domain != "" {
		categories = append(categories, domain)
	}

	// Check for configured domain-based categories
	if domain != "" && categoryMapper != nil {
		if category := categoryMapper.GetCategoryForDomain(domain); category != "" {
			categories = append(categories, category)
		}
	}

	// Content type detection
	titleLower := strings.ToLower(title)
	switch {
	case strings.HasPrefix(titleLower, "show hn:"):
		categories = append(categories, "Show HN")
	case strings.HasPrefix(titleLower, "ask hn:"):
		categories = append(categories, "Ask HN")
	case strings.Contains(titleLower, "pdf") || strings.HasSuffix(url, ".pdf"):
		categories = append(categories, "PDF")
	case strings.Contains(titleLower, "video"):
		categories = append(categories, "Video")
	case strings.Contains(titleLower, "book") || strings.Contains(titleLower, "ebook"):
		categories = append(categories, "Book")
	}

	return categories
}

// categorizeByPoints returns a category label based on point count and threshold
func categorizeByPoints(points int, minPoints int) string {
	switch {
	case points >= 500:
		return "Viral 500+"
	case points >= 200:
		return "Hot 200+"
	case points >= 100:
		return "High Score 100+"
	case points >= minPoints*2:
		return fmt.Sprintf("High Score %d+", minPoints*2)
	case points >= minPoints:
		return fmt.Sprintf("Popular %d+", minPoints)
	default:
		return "Rising"
	}
}
