package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// slotRecheckInterval bounds how long the slot loop sleeps between schedule
// checks, so a reloaded slot list takes effect within a minute instead of only
// after the previously computed wake-up.
const slotRecheckInterval = time.Minute

// daemon runs the serve command: an internal scheduler that replaces the cron
// entries for generate, bulletin-fetch, and the bulletin digest slots.
//
// The configuration lives behind an atomic pointer; every loop iteration loads
// the current snapshot, so a SIGHUP reload takes effect from the next run
// without racing in-flight work. bulletinMu serializes the bulletin stages: a
// second concurrent Generate would summarise the same backlog twice (double
// model spend), and Fetch writing during Generate's commit risks SQLITE_BUSY.
type daemon struct {
	configPath string
	cfg        atomic.Pointer[appConfig]
	bulletinMu sync.Mutex

	now func() time.Time

	// Job implementations, injectable for tests.
	generateFn func(*appConfig) error
	fetchFn    func(context.Context, *appConfig) error
	digestFn   func(context.Context, *appConfig) error
}

func newDaemon(configPath string, cfg *appConfig) *daemon {
	d := &daemon{
		configPath: configPath,
		now:        time.Now,
		generateFn: generateAll,
		fetchFn:    runBulletinFetch,
		digestFn:   runBulletinDigest,
	}
	d.cfg.Store(cfg)
	return d
}

// runBulletinDigest is one digest slot: generate the bulletin, then publish it,
// preserving the `bulletin-generate && bulletin-publish` chaining of the cron
// setup — a failed generate must not re-render the feed.
func runBulletinDigest(ctx context.Context, cfg *appConfig) error {
	if err := runBulletinGenerate(ctx, cfg, ""); err != nil {
		return fmt.Errorf("bulletin generate: %w", err)
	}
	if err := runBulletinPublish(ctx, cfg, bulletinFeedName); err != nil {
		return fmt.Errorf("bulletin publish: %w", err)
	}
	return nil
}

// runServe is the serve command: load and validate the configuration, then run
// the scheduler loops until SIGTERM/SIGINT. A clean shutdown returns nil so the
// systemd unit's Restart=on-failure does not restart a deliberate stop.
func runServe(configPath string) error {
	// #nosec G304 G703 -- the config path is an explicit CLI input.
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	cfg, err := validateConfig(data)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	d := newDaemon(configPath, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	slog.Info("serve: starting",
		"config", configPath,
		"generate-tick", cfg.Serve.GenerateTick,
		"bulletin-fetch-interval", cfg.Serve.BulletinFetchInterval,
		"bulletin-slots", fmt.Sprint(cfg.Serve.BulletinSlots),
		"bulletin-feeds", len(cfg.Bulletin.Feeds))

	var wg sync.WaitGroup
	loops := []func(context.Context){
		func(ctx context.Context) { d.reloadLoop(ctx, hup) },
		d.generateLoop,
		d.fetchLoop,
		d.slotLoop,
	}
	for _, loop := range loops {
		wg.Go(func() { loop(ctx) })
	}
	wg.Wait()

	slog.Info("serve: shut down cleanly")
	return nil
}

// reloadLoop applies SIGHUP configuration reloads: validate the file, swap the
// snapshot on success, keep the previous configuration and log on failure.
func (d *daemon) reloadLoop(ctx context.Context, hup <-chan os.Signal) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-hup:
			d.reload()
		}
	}
}

func (d *daemon) reload() {
	// #nosec G304 G703 -- the config path is an explicit CLI input.
	data, err := os.ReadFile(d.configPath)
	var cfg *appConfig
	if err == nil {
		cfg, err = validateConfig(data)
	}
	if err != nil {
		slog.Error("serve: config reload failed, keeping previous configuration", "error", err)
		return
	}
	if old := d.cfg.Load(); cfg.CacheDir != old.CacheDir {
		slog.Warn("serve: cache-dir changed in config; a restart is required for it to take effect")
	}
	d.cfg.Store(cfg)
	slog.Info("serve: configuration reloaded")
}

// generateLoop runs generateAll immediately (per-provider mtime gating makes a
// redundant start cheap) and then on every generate-tick. A run can never
// overlap itself: the next tick starts counting when the run completes.
func (d *daemon) generateLoop(ctx context.Context) {
	for {
		cfg := d.cfg.Load()
		if err := d.generateFn(cfg); err != nil {
			slog.Error("serve: generate run failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.Serve.GenerateTick):
		}
	}
}

// fetchLoop runs bulletin-fetch immediately and then on every
// bulletin-fetch-interval.
func (d *daemon) fetchLoop(ctx context.Context) {
	for {
		cfg := d.cfg.Load()
		d.runFetch(ctx, cfg)
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.Serve.BulletinFetchInterval):
		}
	}
}

// runFetch runs one bulletin-fetch pass. It only tries the bulletin mutex: when
// a digest slot holds it, this pass is skipped — fetch is idempotent and the
// next interval is close.
func (d *daemon) runFetch(ctx context.Context, cfg *appConfig) {
	if len(cfg.Bulletin.Feeds) == 0 {
		return
	}
	if !d.bulletinMu.TryLock() {
		slog.Info("serve: bulletin fetch skipped; another bulletin stage is running")
		return
	}
	defer d.bulletinMu.Unlock()
	if err := d.fetchFn(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("serve: bulletin fetch failed", "error", err)
	}
}

// slotLoop fires the bulletin digest at the configured time-of-day slots. It
// wakes at least every slotRecheckInterval so a reloaded slot list applies
// promptly, and recomputes the next slot after each firing — a slot can never
// overlap itself.
func (d *daemon) slotLoop(ctx context.Context) {
	for {
		cfg := d.cfg.Load()
		now := d.now()

		wait := slotRecheckInterval
		fire := false
		if len(cfg.Serve.BulletinSlots) > 0 {
			target := nextSlotFire(now, cfg.Serve.BulletinSlots)
			if until := target.Sub(now); until <= slotRecheckInterval {
				wait = until
				fire = true
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if fire {
			d.runDigest(ctx, d.cfg.Load())
		}
	}
}

// runDigest runs one digest slot under the bulletin mutex. Unlike fetch it
// blocks on the mutex: a slot must not be lost to an in-flight fetch, and fetch
// runs are bounded (feed count × polite delay).
func (d *daemon) runDigest(ctx context.Context, cfg *appConfig) {
	if len(cfg.Bulletin.Feeds) == 0 {
		return
	}
	d.bulletinMu.Lock()
	defer d.bulletinMu.Unlock()
	if err := d.digestFn(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("serve: bulletin digest failed", "error", err)
	}
}
