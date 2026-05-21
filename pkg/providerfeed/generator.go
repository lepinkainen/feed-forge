package providerfeed

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lepinkainen/feed-forge/pkg/feed"
	"github.com/lepinkainen/feed-forge/pkg/feedmeta"
	"github.com/lepinkainen/feed-forge/pkg/filesystem"
	"github.com/lepinkainen/feed-forge/pkg/httpcache"
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
			return handleFetchError(outfile, err)
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

func handleFetchError(outfile string, err error) error {
	if !errors.Is(err, httpcache.ErrNotModified) {
		return err
	}
	if _, statErr := os.Stat(outfile); statErr == nil {
		now := time.Now()
		if chtimeErr := os.Chtimes(outfile, now, now); chtimeErr != nil {
			return chtimeErr
		}
		slog.Debug("Feed unchanged, bumped mtime", "outfile", outfile)
		return nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	slog.Warn("Upstream unchanged but output file is missing", "outfile", outfile)
	return err
}
