package opengraph

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func (f *Fetcher) fetchWithExpiredHint(ctx context.Context, targetURL string, expired *Data) (*Data, error) {
	var etag, lastModified string
	if expired != nil {
		etag = expired.ETag
		lastModified = expired.LastModified
	}
	if etag == "" && lastModified == "" {
		return f.fetchFreshData(ctx, targetURL)
	}
	return f.fetchFreshDataConditional(ctx, targetURL, etag, lastModified)
}

func (f *Fetcher) fetchFreshData(ctx context.Context, targetURL string) (*Data, error) {
	return f.fetchFreshDataConditional(ctx, targetURL, "", "")
}

func (f *Fetcher) fetchFreshDataConditional(ctx context.Context, targetURL, etag, lastModified string) (*Data, error) {
	v, err, _ := f.fetchGroup.Do(targetURL, func() (any, error) {
		return f.doFetchConditional(ctx, targetURL, etag, lastModified)
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(*Data), nil
}

func (f *Fetcher) doFetchConditional(ctx context.Context, targetURL, etag, lastModified string) (*Data, error) {
	select {
	case f.semaphore <- struct{}{}:
		defer func() { <-f.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if err := f.applyDomainRateLimit(ctx, targetURL); err != nil {
		return nil, err
	}

	req, useProxy, err := f.buildFetchRequest(ctx, targetURL, etag, lastModified)
	if err != nil {
		return nil, err
	}
	if useProxy {
		slog.Debug("Fetching OpenGraph data via proxy", "url", targetURL, "proxy", f.proxy.URL)
	} else {
		slog.Debug("Fetching OpenGraph data", "url", targetURL)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode == http.StatusNotModified {
		return nil, errNotModified
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isAcceptableHTMLContentType(contentType) {
		return nil, fmt.Errorf("not an HTML page: %s", contentType)
	}

	htmlContent, err := f.readAndDecodeBody(resp, contentType)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	now := time.Now()
	data := &Data{
		URL:          targetURL,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Duration(DefaultCacheHours) * time.Hour),
	}
	extractOpenGraphTags(doc, data)
	slog.Debug("Extracted OpenGraph data", "url", targetURL, "title", data.Title, "hasDescription", data.Description != "")
	return data, nil
}

func (f *Fetcher) buildFetchRequest(ctx context.Context, targetURL, etag, lastModified string) (*http.Request, bool, error) {
	requestURL := targetURL
	useProxy := f.proxy != nil && isProxiableRedditURL(targetURL)
	if useProxy {
		requestURL = f.proxy.URL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, http.NoBody)
	if err != nil {
		return nil, useProxy, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FeedForge/1.0; OpenGraph fetcher)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Connection", "keep-alive")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
	if useProxy {
		req.Header.Set("X-Proxy-Secret", f.proxy.Secret)
		req.Header.Set("X-Target-URL", targetURL)
	}
	return req, useProxy, nil
}

func isAcceptableHTMLContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

func (f *Fetcher) readAndDecodeBody(resp *http.Response, contentType string) (string, error) {
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() {
			if closeErr := gz.Close(); closeErr != nil {
				slog.Error("Failed to close reader", "error", closeErr)
			}
		}()
		reader = gz
	default:
		reader = resp.Body
	}

	const maxBodySize = 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(reader, maxBodySize))
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	htmlContent, err := convertToUTF8(body, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to convert content to UTF-8: %w", err)
	}
	return htmlContent, nil
}
