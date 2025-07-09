package opengraph

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// Fetcher handles OpenGraph metadata fetching with rate limiting and caching
type Fetcher struct {
	client      *http.Client
	db          *Database
	cache       map[string]*Data
	cacheMutex  sync.RWMutex
	domainMutex sync.Mutex
	lastFetch   map[string]time.Time
	semaphore   chan struct{}
	urlMutexes  sync.Map
}

// NewFetcher creates a new OpenGraph fetcher
func NewFetcher(db *Database) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		db:        db,
		cache:     make(map[string]*Data),
		lastFetch: make(map[string]time.Time),
		semaphore: make(chan struct{}, 5), // Max 5 concurrent fetches
	}
}

// FetchData fetches OpenGraph data from a URL with caching
func (f *Fetcher) FetchData(targetURL string) (*Data, error) {
	// Validate URL format
	if !isValidURL(targetURL) {
		return nil, fmt.Errorf("invalid URL format: %s", targetURL)
	}

	// Check if it's a blocked URL
	if f.isBlockedURL(targetURL) {
		slog.Debug("Skipping blocked URL", "url", targetURL)
		return nil, nil
	}

	// Check database cache first
	if f.db != nil {
		cached, err := f.db.GetCachedData(targetURL)
		if err != nil {
			slog.Warn("Error reading from cache", "url", targetURL, "error", err)
		}
		if cached != nil {
			slog.Debug("Found cached OpenGraph data", "url", targetURL)
			return cached, nil
		}

		// Check for recent failures
		hasFailure, err := f.db.HasRecentFailure(targetURL)
		if err != nil {
			slog.Warn("Error checking recent failures", "url", targetURL, "error", err)
		}
		if hasFailure {
			slog.Debug("Skipping URL due to recent failure", "url", targetURL)
			return nil, nil
		}
	}

	// Fetch fresh data
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	data, err := f.fetchFreshData(ctx, targetURL)
	fetchSuccess := err == nil && data != nil

	if err != nil {
		slog.Debug("Failed to fetch OpenGraph data", "url", targetURL, "error", err)
		// Create empty data for caching the failure
		if data == nil {
			data = &Data{
				URL:       targetURL,
				FetchedAt: time.Now(),
				ExpiresAt: time.Now().Add(1 * time.Hour), // Shorter expiry for failures
			}
		}
	} else if data != nil {
		f.cleanupData(data)
		slog.Debug("Successfully fetched OpenGraph data", "url", targetURL, "title", data.Title)
	}

	// Cache the result (success or failure)
	if f.db != nil && data != nil {
		if err := f.db.SaveCachedData(data, fetchSuccess); err != nil {
			slog.Warn("Failed to cache OpenGraph data", "url", targetURL, "error", err)
		}
	}

	if fetchSuccess {
		return data, nil
	}

	return nil, err
}

// fetchFreshData fetches fresh OpenGraph data from a URL
func (f *Fetcher) fetchFreshData(ctx context.Context, targetURL string) (*Data, error) {
	// Get or create a mutex for this URL to prevent concurrent fetches
	urlMutexInterface, _ := f.urlMutexes.LoadOrStore(targetURL, &sync.Mutex{})
	urlMutex := urlMutexInterface.(*sync.Mutex)

	urlMutex.Lock()
	defer urlMutex.Unlock()

	// Acquire semaphore slot
	select {
	case f.semaphore <- struct{}{}:
		defer func() { <-f.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Apply domain-based rate limiting
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	domain := parsedURL.Host

	f.domainMutex.Lock()
	if lastFetch, exists := f.lastFetch[domain]; exists {
		timeSinceLastFetch := time.Since(lastFetch)
		if timeSinceLastFetch < time.Second {
			sleepTime := time.Second - timeSinceLastFetch
			f.domainMutex.Unlock()
			slog.Debug("Rate limiting domain", "domain", domain, "sleep", sleepTime)
			select {
			case <-time.After(sleepTime):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			f.domainMutex.Lock()
		}
	}
	f.lastFetch[domain] = time.Now()
	f.domainMutex.Unlock()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FeedForge/1.0; OpenGraph fetcher)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")

	slog.Debug("Fetching OpenGraph data", "url", targetURL)

	// Make the request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") &&
		!strings.Contains(strings.ToLower(contentType), "application/xhtml") {
		return nil, fmt.Errorf("not an HTML page: %s", contentType)
	}

	// Handle compression
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}

	// Read response body with size limit
	const maxBodySize = 1024 * 1024 // 1MB limit
	body, err := io.ReadAll(io.LimitReader(reader, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Convert to UTF-8
	htmlContent, err := f.convertToUTF8(body, contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert content to UTF-8: %w", err)
	}

	// Parse HTML and extract OpenGraph data
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract OpenGraph data
	now := time.Now()
	data := &Data{
		URL:       targetURL,
		FetchedAt: now,
		ExpiresAt: now.Add(time.Duration(DefaultCacheHours) * time.Hour),
	}

	f.extractOpenGraphTags(doc, data)
	f.applyFallbacks(data, htmlContent)

	slog.Debug("Extracted OpenGraph data", "url", targetURL, "title", data.Title, "hasDescription", data.Description != "")

	return data, nil
}

// extractOpenGraphTags recursively extracts OpenGraph meta tags from HTML
func (f *Fetcher) extractOpenGraphTags(n *html.Node, data *Data) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "meta":
			f.processMetaTag(n, data)
		case "title":
			if data.Title == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				data.Title = strings.TrimSpace(n.FirstChild.Data)
			}
		}
	}

	// Recursively process child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		f.extractOpenGraphTags(c, data)
	}
}

// processMetaTag processes individual meta tags
func (f *Fetcher) processMetaTag(n *html.Node, data *Data) {
	var property, content, name string

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

	// Process OpenGraph properties
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

	// Process fallback meta tags
	if data.Description == "" {
		switch name {
		case "description":
			data.Description = content
		case "twitter:description":
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

// applyFallbacks applies fallback strategies for missing OpenGraph data
func (f *Fetcher) applyFallbacks(data *Data, htmlContent string) {
	// If no description, try to extract from first paragraph
	if data.Description == "" {
		data.Description = f.extractFirstParagraph(htmlContent)
	}

	// If no site name, try to extract from domain
	if data.SiteName == "" && data.URL != "" {
		if u, err := url.Parse(data.URL); err == nil {
			data.SiteName = u.Host
		}
	}
}

// extractFirstParagraph extracts the first paragraph from HTML content
func (f *Fetcher) extractFirstParagraph(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var findFirstP func(*html.Node) string
	findFirstP = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "p" {
			var text strings.Builder
			var extractText func(*html.Node)
			extractText = func(node *html.Node) {
				if node.Type == html.TextNode {
					text.WriteString(node.Data)
				}
				for c := node.FirstChild; c != nil; c = c.NextSibling {
					extractText(c)
				}
			}
			extractText(n)

			result := strings.TrimSpace(text.String())
			if len(result) > 20 {
				return result
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := findFirstP(c); result != "" {
				return result
			}
		}
		return ""
	}

	return findFirstP(doc)
}

// cleanupData validates and cleans up OpenGraph data
func (f *Fetcher) cleanupData(data *Data) {
	// Truncate long descriptions
	if len(data.Description) > 500 {
		data.Description = data.Description[:497] + "..."
	}

	// Truncate long titles
	if len(data.Title) > 200 {
		data.Title = data.Title[:197] + "..."
	}

	// Validate image URL
	if data.Image != "" && !isValidURL(data.Image) {
		slog.Warn("Invalid image URL found, clearing", "url", data.Image)
		data.Image = ""
	}

	// Clean up whitespace and normalize
	data.Title = strings.TrimSpace(data.Title)
	data.Description = strings.TrimSpace(data.Description)
	data.SiteName = strings.TrimSpace(data.SiteName)

	// Remove any null bytes or control characters
	data.Title = strings.ReplaceAll(data.Title, "\x00", "")
	data.Description = strings.ReplaceAll(data.Description, "\x00", "")
	data.SiteName = strings.ReplaceAll(data.SiteName, "\x00", "")
}

// convertToUTF8 converts response body to UTF-8 string with proper encoding detection
func (f *Fetcher) convertToUTF8(body []byte, contentType string) (string, error) {
	reader := strings.NewReader(string(body))

	// Use charset package to detect and convert encoding
	utf8Reader, err := charset.NewReader(reader, contentType)
	if err != nil {
		// If charset detection fails, assume UTF-8
		slog.Warn("Failed to detect charset, assuming UTF-8", "error", err)
		return string(body), nil
	}

	// Read the UTF-8 converted content
	utf8Bytes, err := io.ReadAll(utf8Reader)
	if err != nil {
		return "", fmt.Errorf("failed to convert to UTF-8: %w", err)
	}

	return string(utf8Bytes), nil
}

// isValidURL checks if a URL is valid
func isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// isBlockedURL checks if a URL is from a domain that blocks external access
func (f *Fetcher) isBlockedURL(targetURL string) bool {
	// Check if it's a Reddit URL
	if strings.Contains(targetURL, "reddit.com") || strings.Contains(targetURL, "redd.it") {
		return true
	}

	blockedDomains := []string{
		"x.com",
		"twitter.com",
		"facebook.com",
		"instagram.com",
		"linkedin.com",
		"i.redd.it",
		"v.redd.it",
		"reddit.com/gallery",
	}

	for _, domain := range blockedDomains {
		if strings.Contains(targetURL, domain) {
			return true
		}
	}
	return false
}

// FetchConcurrent fetches OpenGraph data for multiple URLs concurrently
func (f *Fetcher) FetchConcurrent(urls []string) map[string]*Data {
	if len(urls) == 0 {
		return make(map[string]*Data)
	}

	type result struct {
		url  string
		data *Data
	}

	results := make(chan result, len(urls))
	var wg sync.WaitGroup

	// Limit concurrent requests
	const maxConcurrent = 5
	semaphore := make(chan struct{}, maxConcurrent)

	slog.Info("Starting concurrent OpenGraph fetch", "total_urls", len(urls))

	for _, targetURL := range urls {
		if targetURL == "" {
			continue
		}

		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			slog.Debug("Processing URL for OpenGraph", "url", url)
			data, err := f.FetchData(url)
			if err != nil {
				slog.Debug("Failed to fetch OpenGraph data for URL", "url", url, "error", err)
				data = nil
			}

			if data != nil {
				slog.Debug("OpenGraph data obtained", "url", url, "title", data.Title)
			} else {
				slog.Debug("No OpenGraph data obtained", "url", url)
			}

			results <- result{url: url, data: data}
		}(targetURL)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	dataMap := make(map[string]*Data)
	for res := range results {
		if res.data != nil {
			dataMap[res.url] = res.data
		}
	}

	slog.Info("Completed concurrent OpenGraph fetch", "successful_fetches", len(dataMap))
	return dataMap
}
