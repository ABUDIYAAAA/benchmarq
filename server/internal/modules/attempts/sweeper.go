package attempts

import (
	"context"
	"log/slog"
	"time"
)

// SweepInterval is how often the server settles attempts on its own.
const SweepInterval = 10 * time.Second

// RunSweeper settles due attempts every interval until ctx is cancelled.
// Clocks are exact without it, since every request settles its attempt; the
// sweeper makes sure attempts nobody is polling still end and get graded.
func RunSweeper(ctx context.Context, svc Service, logger *slog.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := svc.Sweep(ctx)
			if err != nil && ctx.Err() == nil {
				logger.Error("attempts: sweep failed", "error", err)
			} else if n > 0 {
				logger.Info("attempts: settled attempts", "count", n)
			}
		}
	}
}
