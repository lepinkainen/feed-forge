package hackernews

import (
	"fmt"
	"strings"
	"time"
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

// calculatePostAge returns a human-readable time difference from the given time to now
func calculatePostAge(createdAt time.Time) string {
	now := time.Now()
	diff := now.Sub(createdAt)

	switch {
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		if minutes < 1 {
			return "just now"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	default:
		weeks := int(diff.Hours() / (24 * 7))
		return fmt.Sprintf("%d weeks ago", weeks)
	}
}
