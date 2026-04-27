package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/metrics"
	"github.com/maxlesscode/watchdog/internal/models"
)

// --- mocks ---

type mockStore struct {
	products       []models.Product
	productsErr    error
	updatePriceErr error
	insertHistErr  error

	updatePriceCalls atomic.Int32
	insertHistCalls  atomic.Int32
	updateAlertCalls atomic.Int32
}

func (m *mockStore) GetAllProducts(_ context.Context) ([]models.Product, error) {
	return m.products, m.productsErr
}
func (m *mockStore) UpdateActualPrice(_ context.Context, _ database.UpdateActualPriceInput) error {
	m.updatePriceCalls.Add(1)
	return m.updatePriceErr
}
func (m *mockStore) InsertPriceHistory(_ context.Context, _ database.InsertPriceHistoryInput) error {
	m.insertHistCalls.Add(1)
	return m.insertHistErr
}
func (m *mockStore) UpdateLastAlerted(_ context.Context, _ database.UpdateLastAlertedInput) error {
	m.updateAlertCalls.Add(1)
	return nil
}
func (m *mockStore) GetProductByID(_ context.Context, _ int) (models.Product, error) {
	return models.Product{}, nil
}
func (m *mockStore) AddProduct(_ context.Context, _ models.Product) (int, error) { return 0, nil }
func (m *mockStore) UpdateProduct(_ context.Context, _ int, _ models.Product) (models.Product, error) {
	return models.Product{}, nil
}
func (m *mockStore) DeleteProduct(_ context.Context, _ int) error { return nil }
func (m *mockStore) Ping(_ context.Context) error                 { return nil }

type mockScraper struct {
	price float64
	err   error
	calls atomic.Int32
}

func (m *mockScraper) FetchPrice(_ context.Context, _, _ string) (float64, error) {
	m.calls.Add(1)
	return m.price, m.err
}

type mockNotifier struct {
	err   error
	calls atomic.Int32
}

func (m *mockNotifier) Notify(_ context.Context, _ models.Product) error {
	m.calls.Add(1)
	return m.err
}

// --- helper ---

func newScheduler(store *mockStore, sc *mockScraper, notif *mockNotifier) *Scheduler {
	cfg := Config{Store: store, Scraper: sc, Interval: time.Hour, Concurrency: 5}
	if notif != nil {
		cfg.Notifier = notif
	}
	return New(cfg)
}

// --- New() tests ---

func TestNew_DefaultsInterval(t *testing.T) {
	t.Parallel()
	s := New(Config{Store: &mockStore{}, Scraper: &mockScraper{}})
	if s.interval != time.Hour {
		t.Errorf("interval = %v, want 1h", s.interval)
	}
}

func TestNew_DefaultsConcurrency(t *testing.T) {
	t.Parallel()
	s := New(Config{Store: &mockStore{}, Scraper: &mockScraper{}})
	if s.concurrency != 20 {
		t.Errorf("concurrency = %d, want 20", s.concurrency)
	}
}

func TestNew_NilStorePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil Store, got none")
		}
	}()
	New(Config{Scraper: &mockScraper{}})
}

func TestNew_NilScraperPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil Scraper, got none")
		}
	}()
	New(Config{Store: &mockStore{}})
}

func TestNew_NilNotifierDoesNotPanic(t *testing.T) {
	t.Parallel()
	s := New(Config{Store: &mockStore{}, Scraper: &mockScraper{}, Notifier: nil})
	if s == nil {
		t.Error("expected non-nil scheduler")
	}
}

// --- fetchAndStore tests ---

func TestFetchAndStore_ScraperError_StopsEarly(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	s := newScheduler(store, &mockScraper{err: errors.New("fetch failed")}, nil)

	s.fetchAndStore(context.Background(), models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50})

	if n := store.updatePriceCalls.Load(); n != 0 {
		t.Errorf("UpdateActualPrice calls = %d, want 0 when scraper fails", n)
	}
}

func TestFetchAndStore_ScraperError_IncrementsErrorMetric(t *testing.T) {
	t.Parallel()
	s := newScheduler(&mockStore{}, &mockScraper{err: errors.New("fetch failed")}, nil)

	before := metrics.ScrapeErrors.Value()
	s.fetchAndStore(context.Background(), models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50})

	// delta < 1 not != 1: global counter; parallel tests may add more than 1
	if delta := metrics.ScrapeErrors.Value() - before; delta < 1 {
		t.Errorf("ScrapeErrors delta = %d, want >= 1", delta)
	}
}

func TestFetchAndStore_UpdatePriceError_StopsEarly(t *testing.T) {
	t.Parallel()
	store := &mockStore{updatePriceErr: errors.New("db error")}
	notif := &mockNotifier{}
	s := newScheduler(store, &mockScraper{price: 30}, notif)

	s.fetchAndStore(context.Background(), models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50})

	if n := store.insertHistCalls.Load(); n != 0 {
		t.Errorf("InsertPriceHistory calls = %d, want 0 when UpdateActualPrice fails", n)
	}
	if n := notif.calls.Load(); n != 0 {
		t.Errorf("Notify calls = %d, want 0 when UpdateActualPrice fails", n)
	}
}

func TestFetchAndStore_PriceAboveTarget_NoNotify(t *testing.T) {
	t.Parallel()
	notif := &mockNotifier{}
	s := newScheduler(&mockStore{}, &mockScraper{price: 100}, notif)

	s.fetchAndStore(context.Background(), models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50})

	if n := notif.calls.Load(); n != 0 {
		t.Errorf("Notify calls = %d, want 0 when price > target", n)
	}
}

func TestFetchAndStore_PriceAtTarget_NotifiesFirstAlert(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	notif := &mockNotifier{}
	s := newScheduler(store, &mockScraper{price: 50}, notif)

	p := models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50, LastAlertedAt: nil}
	s.fetchAndStore(context.Background(), p)

	if n := notif.calls.Load(); n != 1 {
		t.Errorf("Notify calls = %d, want 1 on first alert (price == target)", n)
	}
	if n := store.updateAlertCalls.Load(); n != 1 {
		t.Errorf("UpdateLastAlerted calls = %d, want 1", n)
	}
}

func TestFetchAndStore_PriceBelowTarget_NotifiesFirstAlert(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	notif := &mockNotifier{}
	s := newScheduler(store, &mockScraper{price: 30}, notif)

	p := models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50, LastAlertedAt: nil}
	s.fetchAndStore(context.Background(), p)

	if n := notif.calls.Load(); n != 1 {
		t.Errorf("Notify calls = %d, want 1 when price < target and never alerted", n)
	}
}

func TestFetchAndStore_CooldownActive_NoNotify(t *testing.T) {
	t.Parallel()
	notif := &mockNotifier{}
	s := newScheduler(&mockStore{}, &mockScraper{price: 30}, notif)

	// Alerted 1 hour ago — still within the 24-hour cooldown.
	recent := time.Now().Add(-1 * time.Hour)
	p := models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50, LastAlertedAt: &recent}
	s.fetchAndStore(context.Background(), p)

	if n := notif.calls.Load(); n != 0 {
		t.Errorf("Notify calls = %d, want 0 within 24h cooldown", n)
	}
}

func TestFetchAndStore_CooldownExpired_Notifies(t *testing.T) {
	t.Parallel()
	notif := &mockNotifier{}
	s := newScheduler(&mockStore{}, &mockScraper{price: 30}, notif)

	// Alerted 25 hours ago — cooldown has expired.
	old := time.Now().Add(-25 * time.Hour)
	p := models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50, LastAlertedAt: &old}
	s.fetchAndStore(context.Background(), p)

	if n := notif.calls.Load(); n != 1 {
		t.Errorf("Notify calls = %d, want 1 when cooldown has expired", n)
	}
}

func TestFetchAndStore_NilNotifier_DoesNotPanic(t *testing.T) {
	t.Parallel()
	// Notifier is nil; price is below target. Must not panic.
	s := newScheduler(&mockStore{}, &mockScraper{price: 30}, nil)
	s.fetchAndStore(context.Background(), models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50})
}

func TestFetchAndStore_NotifyError_DoesNotUpdateLastAlerted(t *testing.T) {
	t.Parallel()
	store := &mockStore{}
	notif := &mockNotifier{err: errors.New("smtp down")}
	s := newScheduler(store, &mockScraper{price: 30}, notif)

	p := models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50, LastAlertedAt: nil}
	s.fetchAndStore(context.Background(), p)

	if n := store.updateAlertCalls.Load(); n != 0 {
		t.Errorf("UpdateLastAlerted calls = %d, want 0 when Notify fails", n)
	}
}

func TestFetchAndStore_SuccessfulScrape_IncrementsScrapeTotal(t *testing.T) {
	t.Parallel()
	s := newScheduler(&mockStore{}, &mockScraper{price: 100}, nil)

	before := metrics.ScrapeTotal.Value()
	s.fetchAndStore(context.Background(), models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50})

	// delta < 1 not != 1: global counter; parallel tests may add more than 1
	if delta := metrics.ScrapeTotal.Value() - before; delta < 1 {
		t.Errorf("ScrapeTotal delta = %d, want >= 1", delta)
	}
}

// --- RunCycle tests ---

func TestRunCycle_StoreError_NothingScraped(t *testing.T) {
	t.Parallel()
	sc := &mockScraper{}
	s := newScheduler(&mockStore{productsErr: errors.New("db down")}, sc, nil)

	s.RunCycle(context.Background())

	if n := sc.calls.Load(); n != 0 {
		t.Errorf("scraper calls = %d, want 0 when store fails", n)
	}
}

func TestRunCycle_ScrapesAllProducts(t *testing.T) {
	t.Parallel()
	store := &mockStore{products: []models.Product{
		{ID: 1, URL: "https://a.example.com", TargetPrice: 100},
		{ID: 2, URL: "https://b.example.com", TargetPrice: 100},
		{ID: 3, URL: "https://c.example.com", TargetPrice: 100},
	}}
	sc := &mockScraper{price: 50}
	s := newScheduler(store, sc, nil)

	s.RunCycle(context.Background())

	if n := sc.calls.Load(); n != 3 {
		t.Errorf("scraper calls = %d, want 3 (one per product)", n)
	}
}

func TestRunCycle_ContextCancelled_StopsDispatching(t *testing.T) {
	t.Parallel()

	// 30 products with concurrency=2: the semaphore fills, forcing RunCycle
	// to select on ctx.Done() vs sem. Pre-cancelling exercises the cancel branch.
	products := make([]models.Product, 30)
	for i := range products {
		products[i] = models.Product{ID: i + 1, URL: "https://example.com", TargetPrice: 100}
	}

	sc := &mockScraper{price: 50}
	store := &mockStore{products: products}
	s := New(Config{Store: store, Scraper: sc, Interval: time.Hour, Concurrency: 2})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so the cycle exits immediately

	s.RunCycle(ctx) // must not hang
}

func TestFetchAndStore_InsertHistoryError_StillNotifies(t *testing.T) {
	t.Parallel()
	store := &mockStore{insertHistErr: errors.New("history insert failed")}
	notif := &mockNotifier{}
	s := newScheduler(store, &mockScraper{price: 30}, notif)

	p := models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50, LastAlertedAt: nil}
	s.fetchAndStore(context.Background(), p)

	if n := notif.calls.Load(); n != 1 {
		t.Errorf("Notify calls = %d, want 1 — InsertPriceHistory error must not suppress alert", n)
	}
}

func TestFetchAndStore_SuccessfulAlert_IncrementsAlertsSent(t *testing.T) {
	t.Parallel()
	s := newScheduler(&mockStore{}, &mockScraper{price: 30}, &mockNotifier{})

	before := metrics.AlertsSent.Value()
	p := models.Product{ID: 1, URL: "https://example.com", TargetPrice: 50, LastAlertedAt: nil}
	s.fetchAndStore(context.Background(), p)

	// delta < 1 not != 1: global counter; parallel tests may add more than 1
	if delta := metrics.AlertsSent.Value() - before; delta < 1 {
		t.Errorf("AlertsSent delta = %d, want >= 1", delta)
	}
}
