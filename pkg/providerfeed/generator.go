package providerfeed

import (
	"fmt"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/opengraph"
	"github.com/lepinkainen/feed-forge/pkg/providers"
)

// BuildGenerator creates a shared GenerateFeed implementation for providers.
func BuildGenerator(
	fetchItems func(limit int) ([]providers.FeedItem, error),
	preview *providers.PreviewInfo,
	configFunc func() feedmeta.Config,
	ogDB *opengraph.Database,
) func(string) error {
	return func(outfile string) error {
		if fetchItems == nil {
			return fmt.Errorf("feed generator is not configured")
		}
		if preview == nil {
			return fmt.Errorf("preview metadata is not configured")
		}

		feedItems, err := fetchItems(0)
		if err != nil {
			return err
		}

		if err := filesystem.EnsureDirectoryExists(outfile); err != nil {
			return err
		}

		cfg := preview.Config
		if configFunc != nil {
			cfg = configFunc()
		}

		if err := feed.SaveAtomFeedToFileWithEmbeddedTemplate(feedItems, preview.TemplateName, outfile, cfg, ogDB); err != nil {
			return err
		}

		feed.LogFeedGeneration(len(feedItems), outfile)
		return nil
	}
}
