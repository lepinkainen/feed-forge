# Adding a New Provider

This guide shows how to add a new feed provider to feed-forge using the provider registry pattern.

## Quick Start

Adding a new provider requires creating **one file** in `internal/yourprovider/` with:
1. Provider struct that embeds `providers.BaseProvider`
2. `Config` struct for factory configuration
3. `FetchItems()` and `GenerateFeed()` method implementations
4. `init()` function for self-registration
5. Factory function for the registry

## Step-by-Step Guide

### 1. Create Provider Package

Create `internal/yourprovider/provider.go`:

```go
package yourprovider

import (
    "fmt"
    "log/slog"

    "github.com/lepinkainen/feed-forge/pkg/feed"
    "github.com/lepinkainen/feed-forge/pkg/filesystem"
    "github.com/lepinkainen/feed-forge/pkg/providers"
)

// Provider implements the FeedProvider interface
type Provider struct {
    *providers.BaseProvider
    // Add your provider-specific fields
    MinScore int
    Limit    int
}

// Config holds provider configuration for the factory
type Config struct {
    MinScore int
    Limit    int
}
```

### 2. Implement Constructor

```go
// NewProvider creates a new provider instance
func NewProvider(minScore, limit int) providers.FeedProvider {
    // Configure database needs
    base, err := providers.NewBaseProvider(providers.DatabaseConfig{
        ContentDBName: "yourprovider.db", // Optional: provider-specific DB
        UseContentDB:  false,              // Set true if you need caching
    })
    if err != nil {
        slog.Error("Failed to initialize provider", "error", err)
        return nil
    }

    return &Provider{
        BaseProvider: base,
        MinScore:     minScore,
        Limit:        limit,
    }
}
```

### 3. Implement Factory and Registration

```go
// factory creates a provider from configuration (required by registry)
func factory(config any) (providers.FeedProvider, error) {
    cfg, ok := config.(*Config)
    if !ok {
        return nil, fmt.Errorf("invalid config type: expected *yourprovider.Config")
    }

    provider := NewProvider(cfg.MinScore, cfg.Limit)
    if provider == nil {
        return nil, fmt.Errorf("failed to create provider")
    }

    return provider, nil
}

// init registers the provider with the global registry
func init() {
    providers.MustRegister("your-provider", &providers.ProviderInfo{
        Name:        "your-provider",
        Description: "Generate RSS feeds from Your Provider",
        Version:     "1.0.0",
        Factory:     factory,
    })
}
```

### 4. Implement FeedProvider Interface

```go
// FetchItems implements the FeedProvider interface
func (p *Provider) FetchItems(limit int) ([]providers.FeedItem, error) {
    // 1. Fetch data from your source
    items, err := fetchFromYourAPI()
    if err != nil {
        return nil, err
    }

    // 2. Apply filters/limits
    filteredItems := filterItems(items, p.MinScore)

    if limit > 0 && len(filteredItems) > limit {
        filteredItems = filteredItems[:limit]
    }

    // 3. Convert to FeedItem interface
    feedItems := make([]providers.FeedItem, len(filteredItems))
    for i := range filteredItems {
        feedItems[i] = &filteredItems[i]
    }

    return feedItems, nil
}

// GenerateFeed implements the FeedProvider interface
func (p *Provider) GenerateFeed(outfile string, _ bool) error {
    // 1. Clean up expired OpenGraph cache entries
    if err := p.CleanupExpired(); err != nil {
        slog.Warn("Failed to cleanup expired entries", "error", err)
    }

    // 2. Fetch items
    feedItems, err := p.FetchItems(0) // 0 means no limit
    if err != nil {
        return err
    }

    // 3. Ensure output directory exists
    if err := filesystem.EnsureDirectoryExists(outfile); err != nil {
        return err
    }

    // 4. Configure feed metadata
    feedConfig := feed.Config{
        Title:       "Your Provider Feed",
        Link:        "https://yourprovider.com/",
        Description: "Feed description",
        Author:      "Feed Forge",
        ID:          "https://yourprovider.com/",
    }

    // 5. Generate Atom feed
    if err := feed.SaveAtomFeedToFileWithEmbeddedTemplate(
        feedItems,
        "yourprovider-atom", // Template name
        outfile,
        feedConfig,
        p.OgDB, // OpenGraph database for metadata
    ); err != nil {
        return err
    }

    feed.LogFeedGeneration(len(feedItems), outfile)
    return nil
}
```

### 5. Create Your Item Type

Your items must implement `providers.FeedItem` interface:

```go
type Item struct {
    ItemTitle        string
    ItemLink         string
    ItemCommentsLink string
    ItemAuthor       string
    ItemScore        int
    ItemComments     int
    ItemCreated      time.Time
    ItemCategories   []string
    ItemImage        string
    ItemContent      string
}

// Implement FeedItem interface
func (i *Item) Title() string         { return i.ItemTitle }
func (i *Item) Link() string          { return i.ItemLink }
func (i *Item) CommentsLink() string  { return i.ItemCommentsLink }
func (i *Item) Author() string        { return i.ItemAuthor }
func (i *Item) Score() int            { return i.ItemScore }
func (i *Item) CommentCount() int     { return i.ItemComments }
func (i *Item) CreatedAt() time.Time  { return i.ItemCreated }
func (i *Item) Categories() []string  { return i.ItemCategories }
func (i *Item) ImageURL() string      { return i.ItemImage }
func (i *Item) Content() string       { return i.ItemContent }
```

### 6. Update main.go (One-Time Setup)

Add CLI command structure:

```go
import (
    // Add your provider to imports
    "github.com/lepinkainen/feed-forge/internal/yourprovider"
)

var CLI struct {
    // ... existing commands ...

    YourProvider struct {
        Outfile  string `help:"Output file path" short:"o" default:"yourprovider.xml"`
        MinScore int    `help:"Minimum score" default:"50"`
        Limit    int    `help:"Maximum items" default:"30"`
    } `cmd:"your-provider" help:"Generate RSS feed from Your Provider."`
}
```

Add command handler in `main()`:

```go
case "your-provider":
    slog.Debug("Generating Your Provider feed...")

    providerConfig := &yourprovider.Config{
        MinScore: CLI.YourProvider.MinScore,
        Limit:    CLI.YourProvider.Limit,
    }

    var err error
    provider, err = providers.DefaultRegistry.CreateProvider("your-provider", providerConfig)
    if err != nil {
        slog.Error("Failed to create provider", "error", err)
        os.Exit(1)
    }

    if err := provider.GenerateFeed(CLI.YourProvider.Outfile, false); err != nil {
        slog.Error("Failed to generate feed", "error", err)
        os.Exit(1)
    }
```

## Database Configuration

### No Database (Stateless)
```go
providers.NewBaseProvider(providers.DatabaseConfig{
    ContentDBName: "",
    UseContentDB:  false,
})
// Only OpenGraph DB is initialized (automatic)
```

### With Content Database (Stateful Caching)
```go
providers.NewBaseProvider(providers.DatabaseConfig{
    ContentDBName: "yourprovider.db",
    UseContentDB:  true,
})
// Both OpenGraph DB and content DB are initialized
```

## YAML Configuration Support

Users can configure your provider via `config.yaml`:

```yaml
your-provider:
  min_score: 100
  limit: 50
```

Kong automatically loads these values and merges with CLI flags (CLI flags take precedence).

## Examples

See existing providers for complete examples:
- `internal/reddit-json/` - Simple stateless provider (no content DB)
- `internal/hackernews/` - Stateful provider with caching (uses content DB)
- `internal/fingerpori/` - Minimal provider

## Testing

Add tests in `internal/yourprovider/provider_test.go`:

```go
func TestProvider_FetchItems(t *testing.T) {
    provider := NewProvider(50, 30)
    if provider == nil {
        t.Fatal("Failed to create provider")
    }
    defer provider.Close()

    items, err := provider.FetchItems(10)
    if err != nil {
        t.Fatalf("FetchItems failed: %v", err)
    }

    if len(items) > 10 {
        t.Errorf("Expected max 10 items, got %d", len(items))
    }
}
```

## Summary

**Files to create:**
- `internal/yourprovider/provider.go` (main implementation)
- `internal/yourprovider/provider_test.go` (tests)
- `templates/yourprovider-atom.tmpl` (optional: custom template)

**Files to modify:**
- `cmd/feed-forge/main.go` (add CLI command + handler)

**Total effort:** One file with ~150-200 lines for basic provider, plus one-time main.go update.

The provider registry handles everything else automatically!
