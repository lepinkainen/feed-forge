package opengraph

import (
	"context"
	"log/slog"
	"sync"
)

// FetchConcurrent fetches OpenGraph data for multiple URLs concurrently.
func (f *Fetcher) FetchConcurrent(urls []string) map[string]*Data {
	return f.FetchConcurrentWithContext(context.Background(), urls)
}

// FetchConcurrentWithContext fetches OpenGraph data for multiple URLs concurrently.
func (f *Fetcher) FetchConcurrentWithContext(ctx context.Context, urls []string) map[string]*Data {
	if len(urls) == 0 {
		return make(map[string]*Data)
	}

	type result struct {
		url  string
		data *Data
	}

	results := make(chan result, len(urls))
	var wg sync.WaitGroup

	slog.Debug("Starting concurrent OpenGraph fetch", "total_urls", len(urls))

	for _, targetURL := range urls {
		if targetURL == "" {
			continue
		}

		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			slog.Debug("Processing URL for OpenGraph", "url", url)
			data, err := f.FetchDataWithContext(ctx, url)
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

	go func() {
		wg.Wait()
		close(results)
	}()

	dataMap := make(map[string]*Data)
	for res := range results {
		if res.data != nil {
			dataMap[res.url] = res.data
		}
	}

	slog.Debug("Completed concurrent OpenGraph fetch", "successful_fetches", len(dataMap))
	return dataMap
}
