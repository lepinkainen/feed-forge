package opengraph

import (
	"log/slog"
	"time"
)

func (f *Fetcher) lookupCachedData(targetURL string) (cached *Data, expired *Data, skip bool) {
	if f.db == nil {
		return nil, nil, false
	}
	cached, err := f.db.GetCachedData(targetURL)
	if err != nil {
		slog.Warn("Error reading from cache", "url", targetURL, "error", err)
	}
	if cached != nil {
		slog.Debug("Found cached OpenGraph data", "url", targetURL)
		return cached, nil, false
	}

	expired, err = f.db.GetExpiredData(targetURL)
	if err != nil {
		slog.Warn("Error reading expired cache", "url", targetURL, "error", err)
	}

	hasFailure, err := f.db.HasRecentFailure(targetURL)
	if err != nil {
		slog.Warn("Error checking recent failures", "url", targetURL, "error", err)
	}
	if hasFailure {
		slog.Debug("Skipping URL due to recent failure", "url", targetURL)
		return nil, expired, true
	}
	return nil, expired, false
}

func (f *Fetcher) refreshExpired(expired *Data, targetURL string) *Data {
	now := time.Now()
	expired.FetchedAt = now
	expired.ExpiresAt = now.Add(time.Duration(DefaultCacheHours) * time.Hour)
	if f.db != nil {
		if cacheErr := f.db.SaveCachedData(expired, true); cacheErr != nil {
			slog.Warn("Failed to refresh OpenGraph cache expiry", "url", targetURL, "error", cacheErr)
		}
	}
	slog.Debug("OpenGraph data unchanged, refreshed cache expiry", "url", targetURL)
	return expired
}

func newFailurePlaceholder(targetURL string) *Data {
	now := time.Now()
	return &Data{
		URL:       targetURL,
		FetchedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	}
}
