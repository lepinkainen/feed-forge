package feed

import (
	"log/slog"
)

// LogFeedGeneration logs the completion of feed generation
func LogFeedGeneration(itemCount int, filename string) {
	slog.Debug("RSS feed saved", "count", itemCount, "filename", filename)
}
