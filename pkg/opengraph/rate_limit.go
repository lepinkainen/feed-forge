package opengraph

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

func (f *Fetcher) applyDomainRateLimit(ctx context.Context, targetURL string) error {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
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
				return ctx.Err()
			}
			f.domainMutex.Lock()
		}
	}
	f.lastFetch[domain] = time.Now()
	f.domainMutex.Unlock()
	return nil
}
