# Watchdog — MVP Roadmap

Price monitoring API: track product URLs, poll prices hourly, alert when price drops below target.

---

## Status Legend
- `[x]` Done
- `[ ]` To do
- `[~]` In progress

---

## Phase 0 — Foundation ✅

Core API skeleton. Already shipped.

- [x] Product CRUD (`GET /products`, `POST`, `PATCH`, `DELETE`)
- [x] PostgreSQL layer with `ProductStore` interface
- [x] API key middleware (`X-API-Key`)
- [x] Structured logging (`slog`, dual-writer dev/file)
- [x] Centralized error responses (`SendError`)
- [x] Health check (`GET /health`)
- [x] Input validation (`internal/validation`)

---

## Phase 1 — Price Scraper

Fetch actual prices from product URLs. Core value loop.

### 1.1 — Scraper interface
- [x] Define `Scraper` interface: `FetchPrice(ctx, url, selector string) (float64, error)`
- [x] Place in `internal/scraper/scraper.go`
- [x] Keep interface minimal — one method, easy to mock in tests

### 1.2 — HTML scraper implementation
- [x] Add `goquery` dependency (`github.com/PuerkitoBio/goquery`)
- [x] Implement `HTMLScraper` — parse price with CSS selector per-product
- [x] Add `PriceSelector` field to `Product` model (optional, fallback to generic heuristic)
- [x] Generic heuristic: scan for price schema JSON-LD (`$.offers.price`) first, fallback to common CSS patterns
- [x] Set explicit HTTP timeouts (5s), User-Agent header
- [x] Unit tests with `httptest.NewServer` serving fixture HTML

### 1.3 — DB: price history + metadata
- [ ] Add columns to `products` table: `last_checked_at TIMESTAMPTZ`, `price_selector TEXT`
- [ ] Create `price_history` table: `id, product_id, price, checked_at`
- [ ] Extend `ProductStore` interface: `UpdateActualPrice`, `InsertPriceHistory`
- [ ] Implement both in `internal/database/product.go`
- [ ] Auto-create `price_history` table in `StartDB`

### 1.4 — Hourly scheduler
- [ ] Create `internal/scheduler/scheduler.go`
- [ ] `Scheduler` struct holds `ProductStore`, `Scraper`, `Notifier` (Phase 2)
- [ ] `Run(ctx context.Context)` — starts ticker, runs price-fetch loop every hour
- [ ] Goroutine tied to `ctx` (CC-2); stops cleanly on `ctx.Done()`
- [ ] Fan-out with `errgroup` over all products (CC-4); log per-product errors, don't abort loop
- [ ] Wire into `main.go`: `go scheduler.Run(ctx)`

### 1.5 — Manual trigger endpoint (debug/dev)
- [ ] `POST /admin/scrape` — triggers one full scrape cycle immediately
- [ ] Guard with API key (already covered by middleware)

---

## Phase 2 — Notification System

Alert user when price drops below target.

### 2.1 — Notifier interface
- [ ] Define `Notifier` interface: `Notify(ctx context.Context, p models.Product) error`
- [ ] Place in `internal/notifier/notifier.go`

### 2.2 — Email notifier (SMTP)
- [ ] Implement `SMTPNotifier` using `net/smtp` (stdlib, no extra dep)
- [ ] Config from env: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `ALERT_EMAIL`
- [ ] Email body: product name, URL, current price, target price
- [ ] Validate SMTP config on startup (CFG-1)

### 2.3 — Alert check in scheduler
- [ ] After price update: if `actual_price <= target_price` → call `Notifier.Notify`
- [ ] Log alert sent with product ID, prices (OBS-1)

### 2.4 — Deduplication (no spam)
- [ ] Add `last_alerted_at TIMESTAMPTZ` column to `products`
- [ ] Only fire notification if `last_alerted_at` is null OR `last_alerted_at < last_checked_at - 24h`
- [ ] Update `last_alerted_at` after successful notify
- [ ] Extend `ProductStore` with `UpdateLastAlerted(ctx, id, t)`

---

## Phase 3 — Hardening & Observability

Production-grade reliability.

### 3.1 — Rate limiting
- [ ] Add rate limiter middleware: `golang.org/x/time/rate`
- [ ] Per-IP bucket, configurable via env (`RATE_LIMIT`, `RATE_BURST`)

### 3.2 — Request ID propagation
- [ ] Generate `X-Request-ID` in `LoggingMiddleware`, store in `ctx` (OBS-2)
- [ ] Pass through to logs and error responses

### 3.3 — Scraper resilience
- [ ] Retry with exponential backoff (max 3 attempts) per product
- [ ] Circuit-breaker per domain: pause scraping domain after N consecutive failures

### 3.4 — Metrics / pprof
- [ ] Expose `/debug/pprof` guarded by localhost-only check (OBS-3)
- [ ] Basic counters: `scrape_total`, `scrape_errors`, `alerts_sent`

### 3.5 — Tests
- [ ] Table-driven unit tests for scraper HTML parsing (T-1)
- [ ] Integration tests for DB layer using real Postgres (testcontainers or docker compose test target)
- [ ] Handler tests already partially done — expand coverage
- [ ] All tests pass with `-race` flag (T-2)

---

## Phase 4 — CI/CD

Reproducible builds, automated checks.

### 4.1 — GitHub Actions
- [ ] Workflow: lint → vet → test (`-race`) → build on every PR (CI-1)
- [ ] Cache `go mod download`
- [ ] `golangci-lint run` with project config

### 4.2 — Makefile targets
- [ ] `make lint`, `make test`, `make build`, `make docker-up`, `make scrape` (trigger manual cycle)

### 4.3 — Docker
- [ ] Multi-stage `Dockerfile`: builder → minimal runtime image
- [ ] Embed version via `-ldflags "-X main.version=$TAG"` (CI-2)
- [ ] Update `docker-compose.yaml` to include app service alongside Postgres

---

## Phase 5 — Multi-user (Post-MVP)

One API key per user, products scoped to owner.

- [ ] `users` table: `id, email, api_key, created_at`
- [ ] Add `owner_id` FK to `products`
- [ ] Middleware resolves API key → user; inject user into `ctx`
- [ ] All queries scoped to `owner_id`
- [ ] Per-user `ALERT_EMAIL` (stored in `users`)
- [ ] `POST /users` bootstrap endpoint (admin-only)

---

## Dependency Map

```
Phase 0 → Phase 1.1 → 1.2 → 1.3 → 1.4 → 1.5
                                    ↓
Phase 2.1 → 2.2 → 2.3 → 2.4
                    ↓
Phase 3 (parallel with Phase 2)
                    ↓
Phase 4 (wrap Phase 1–3)
                    ↓
Phase 5 (post-MVP, breaks API — major version bump)
```

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| `Scraper` + `Notifier` as interfaces | Mockable in tests; swap SMTP for webhook later without touching scheduler |
| `goquery` for scraping | Mature, BSD license, no headless browser needed for MVP |
| JSON-LD schema.org price first | Most e-commerce sites include it; more reliable than CSS class hunting |
| 24h dedup window for alerts | Avoid notification spam when price stays below target across multiple checks |
| SMTP via `net/smtp` stdlib | Zero extra dep for MVP; upgrade to go-mail if TLS/auth complexity grows |
| Hourly ticker in single goroutine | Simple, low resource; switch to distributed lock (Redis) only when multi-instance needed |
