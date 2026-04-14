package api

import (
	"context"
	"time"
)

// waitWithContext sleeps for d or returns ctx.Err() if the context is cancelled
// first. A non-positive d returns immediately (after checking cancellation).
func waitWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
