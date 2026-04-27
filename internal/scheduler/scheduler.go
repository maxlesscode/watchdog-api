package scheduler

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/metrics"
	"github.com/maxlesscode/watchdog/internal/models"
	"github.com/maxlesscode/watchdog/internal/notifier"
	"github.com/maxlesscode/watchdog/internal/scraper"
)

// Config holds Scheduler dependencies and options.
type Config struct {
	Store    database.ProductStore
	Scraper  scraper.Scraper
	Notifier notifier.Notifier
	Interval time.Duration // defaults to 1h
}

// Scheduler periodically scrapes prices for all tracked products.
type Scheduler struct {
	store    database.ProductStore
	scraper  scraper.Scraper
	notifier notifier.Notifier
	interval time.Duration
}

// New returns a Scheduler from cfg. Interval defaults to 1h if zero.
func New(cfg Config) *Scheduler {
	interval := cfg.Interval
	if interval == 0 {
		interval = time.Hour
	}
	return &Scheduler{
		store:    cfg.Store,
		scraper:  cfg.Scraper,
		notifier: cfg.Notifier,
		interval: interval,
	}
}

// Run starts the hourly ticker loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	slog.Info("scheduler started", "interval", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.RunCycle(ctx)
		}
	}
}

// RunCycle executes one full price-fetch pass over all products.
func (s *Scheduler) RunCycle(ctx context.Context) {
	products, err := s.store.GetAllProducts(ctx)
	if err != nil {
		slog.Error("scheduler: fetch products failed", "err", err)
		return
	}

	var g errgroup.Group
	for _, p := range products {
		g.Go(func() error {
			s.fetchAndStore(ctx, p)
			return nil // log per-product errors; never abort the cycle
		})
	}
	_ = g.Wait()
}

func (s *Scheduler) fetchAndStore(ctx context.Context, p models.Product) {
	now := time.Now().UTC()

	price, err := s.scraper.FetchPrice(ctx, p.URL, p.PriceSelector)
	if err != nil {
		slog.Error("scheduler: scrape failed", "product_id", p.ID, "url", p.URL, "err", err)
		metrics.ScrapeErrors.Add(1)
		return
	}

	if err := s.store.UpdateActualPrice(ctx, database.UpdateActualPriceInput{
		ID:        p.ID,
		Price:     price,
		CheckedAt: now,
	}); err != nil {
		slog.Error("scheduler: update price failed", "product_id", p.ID, "err", err)
		metrics.ScrapeErrors.Add(1)
		return
	}

	if err := s.store.InsertPriceHistory(ctx, database.InsertPriceHistoryInput{
		ProductID: p.ID,
		Price:     price,
		CheckedAt: now,
	}); err != nil {
		slog.Error("scheduler: insert history failed", "product_id", p.ID, "err", err)
	}

	metrics.ScrapeTotal.Add(1)
	slog.Info("scheduler: price updated", "product_id", p.ID, "price", price)

	p.ActualPrice = price
	if price <= p.TargetPrice && s.notifier != nil &&
		(p.LastAlertedAt == nil || p.LastAlertedAt.Before(now.Add(-24*time.Hour))) {
		if err := s.notifier.Notify(ctx, p); err != nil {
			slog.Error("scheduler: notify failed", "product_id", p.ID, "err", err)
			return
		}
		if err := s.store.UpdateLastAlerted(ctx, database.UpdateLastAlertedInput{
			ID:        p.ID,
			AlertedAt: now,
		}); err != nil {
			slog.Error("scheduler: update last_alerted_at failed", "product_id", p.ID, "err", err)
		}
		metrics.AlertsSent.Add(1)
		slog.Info("scheduler: alert sent", "product_id", p.ID, "actual_price", price, "target_price", p.TargetPrice)
	}
}
