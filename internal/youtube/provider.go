package youtube

import (
	"fmt"
	"log/slog"

	"github.com/lepinkainen/feed-forge/pkg/database"
	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// Provider implements the FeedProvider interface for YouTube with DeArrow enrichment
type Provider struct {
	*providers.BaseProvider
	FeedURL       string
	FeedType      string // "channel" or "playlist"
	FeedID        string
	EnableDeArrow bool
	databases     *database.ProviderDatabases
	dearrowClient *DeArrowClient
}

// Config holds YouTube provider configuration for the factory
type Config struct {
	FeedType      string
	FeedID        string
	EnableDeArrow bool
}

// NewProvider creates a new YouTube provider with DeArrow enrichment
func NewProvider(feedType, feedID string, enableDeArrow bool) providers.FeedProvider {
	// Initialize databases
	databases, err := database.InitializeProviderDatabases("youtube.db", true)
	if err != nil {
		slog.Error("Failed to initialize YouTube databases", "error", err)
		return nil
	}

	base, err := providers.NewBaseProvider(providers.DatabaseConfig{
		ContentDBName: "youtube.db",
		UseContentDB:  true,
	})
	if err != nil {
		slog.Error("Failed to initialize YouTube base provider", "error", err)
		if closeErr := databases.Close(); closeErr != nil {
			slog.Error("Failed to close databases", "error", closeErr)
		}
		return nil
	}

	// Construct feed URL
	feedURL, err := ConstructFeedURL(feedType, feedID)
	if err != nil {
		slog.Error("Failed to construct feed URL", "error", err, "feedType", feedType, "feedID", feedID)
		if closeErr := databases.Close(); closeErr != nil {
			slog.Error("Failed to close databases", "error", closeErr)
		}
		if closeErr := base.Close(); closeErr != nil {
			slog.Error("Failed to close base provider", "error", closeErr)
		}
		return nil
	}

	// Initialize DeArrow client if enabled
	var dearrowClient *DeArrowClient
	if enableDeArrow {
		dearrowClient = NewDeArrowClient()
	}

	return &Provider{
		BaseProvider:  base,
		FeedURL:       feedURL,
		FeedType:      feedType,
		FeedID:        feedID,
		EnableDeArrow: enableDeArrow,
		databases:     databases,
		dearrowClient: dearrowClient,
	}
}

// factory creates a YouTube provider from configuration
func factory(config any) (providers.FeedProvider, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type for youtube provider: expected *youtube.Config")
	}

	provider := NewProvider(cfg.FeedType, cfg.FeedID, cfg.EnableDeArrow)
	if provider == nil {
		return nil, fmt.Errorf("failed to create youtube provider")
	}

	return provider, nil
}

func init() {
	providers.MustRegister("youtube", &providers.ProviderInfo{
		Name:        "youtube",
		Description: "Generate RSS feeds from YouTube channels/playlists with DeArrow enrichment",
		Version:     "1.0.0",
		Factory:     factory,
	})
}

// FetchItems implements the FeedProvider interface
func (p *Provider) FetchItems(limit int) ([]providers.FeedItem, error) {
	// Initialize DeArrow cache schema
	if err := initializeDeArrowCache(p.ContentDB); err != nil {
		return nil, err
	}

	// Fetch YouTube RSS feed
	ytFeed, err := FetchYouTubeFeed(p.FeedURL)
	if err != nil {
		return nil, err
	}

	// Convert feed entries to VideoItems with DeArrow enrichment
	feedItems := make([]providers.FeedItem, 0, len(ytFeed.Entries))

	for _, entry := range ytFeed.Entries {
		videoItem := &VideoItem{
			VideoID:       entry.VideoID,
			OriginalTitle: entry.Title,
			ChannelName:   entry.Author.Name,
			ChannelURL:    entry.Author.URI,
			VideoURL:      entry.Link.Href,
			ThumbnailURL:  entry.MediaGroup.Thumbnail.URL,
			Description:   entry.MediaGroup.Description,
			PublishedAt:   entry.Published,
			Views:         entry.MediaGroup.Community.Statistics.Views,
			AverageRating: entry.MediaGroup.Community.StarRating.Average,
			RatingCount:   entry.MediaGroup.Community.StarRating.Count,
		}

		// Enrich with DeArrow title if enabled
		if p.EnableDeArrow && p.dearrowClient != nil {
			enriched, err := p.enrichWithDeArrow(videoItem)
			if err != nil {
				slog.Warn("Failed to enrich with DeArrow", "videoID", entry.VideoID, "error", err)
				// Continue with original title
			} else {
				videoItem = enriched
			}
		}

		feedItems = append(feedItems, videoItem)

		// Apply limit if specified
		if limit > 0 && len(feedItems) >= limit {
			break
		}
	}

	slog.Debug("Fetched YouTube items",
		"count", len(feedItems),
		"deArrowEnabled", p.EnableDeArrow)

	return feedItems, nil
}

// enrichWithDeArrow enriches a video item with DeArrow title if available
func (p *Provider) enrichWithDeArrow(item *VideoItem) (*VideoItem, error) {
	// Check cache first
	cached, err := getCachedTitle(p.ContentDB, item.VideoID)
	if err != nil {
		return nil, fmt.Errorf("failed to check cache: %w", err)
	}

	// Use cached title if available and fresh
	if cached != nil && !shouldRefreshCache(cached) {
		if cached.Trusted && cached.DeArrowTitle.Valid && cached.DeArrowTitle.String != "" {
			item.EnrichedTitle = cached.DeArrowTitle.String
			item.UseDeArrowTitle = true
			slog.Debug("Using cached DeArrow title", "videoID", item.VideoID, "title", item.EnrichedTitle)
		}
		return item, nil
	}

	// Fetch from DeArrow API
	dearrowTitle, trusted, err := p.dearrowClient.FetchTitle(item.VideoID)
	if err != nil {
		// Save empty cache entry to avoid repeated failures
		if saveErr := saveCachedTitle(p.ContentDB, item.VideoID, item.OriginalTitle, "", 0, false, false); saveErr != nil {
			slog.Warn("Failed to save empty cache entry", "videoID", item.VideoID, "error", saveErr)
		}
		return nil, fmt.Errorf("failed to fetch DeArrow title: %w", err)
	}

	// Save to cache (even if empty)
	votes := 0
	locked := false
	if saveErr := saveCachedTitle(p.ContentDB, item.VideoID, item.OriginalTitle, dearrowTitle, votes, locked, trusted); saveErr != nil {
		slog.Warn("Failed to save DeArrow title to cache", "videoID", item.VideoID, "error", saveErr)
	}

	// Use DeArrow title if trusted
	if trusted && dearrowTitle != "" {
		item.EnrichedTitle = dearrowTitle
		item.UseDeArrowTitle = true
		slog.Debug("Using fresh DeArrow title", "videoID", item.VideoID, "title", item.EnrichedTitle)
	}

	return item, nil
}

// GenerateFeed implements the FeedProvider interface
func (p *Provider) GenerateFeed(outfile string, _ bool) error {
	// reauth parameter is ignored for YouTube feeds (no authentication needed)

	// Clean up expired entries using base provider
	if err := p.CleanupExpired(); err != nil {
		slog.Warn("Failed to cleanup expired entries", "error", err)
	}

	// Fetch items using the shared FetchItems method
	feedItems, err := p.FetchItems(0) // 0 means no limit
	if err != nil {
		return err
	}

	// Ensure output directory exists
	if err := filesystem.EnsureDirectoryExists(outfile); err != nil {
		return err
	}

	// Define feed configuration
	var feedTitle, feedLink string
	if p.FeedType == "channel" {
		feedTitle = fmt.Sprintf("YouTube Channel Feed")
		feedLink = fmt.Sprintf("https://www.youtube.com/channel/%s", p.FeedID)
	} else {
		feedTitle = fmt.Sprintf("YouTube Playlist Feed")
		feedLink = fmt.Sprintf("https://www.youtube.com/playlist?list=%s", p.FeedID)
	}

	if p.EnableDeArrow {
		feedTitle += " (DeArrow Enhanced)"
	}

	feedConfig := feed.Config{
		Title:       feedTitle,
		Link:        feedLink,
		Description: "YouTube feed generated by Feed Forge with DeArrow enrichment",
		Author:      "Feed Forge",
		ID:          feedLink,
	}

	// Generate Atom feed using embedded templates with local override
	if err := feed.SaveAtomFeedToFileWithEmbeddedTemplate(feedItems, "youtube-atom", outfile, feedConfig, p.OgDB); err != nil {
		return err
	}

	feed.LogFeedGeneration(len(feedItems), outfile)
	return nil
}
