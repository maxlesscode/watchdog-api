# Watchdog

Price monitoring API. Add product URLs, set a target price — Watchdog polls prices hourly and emails you when the price drops to your target.

Built with Go, PostgreSQL, and zero frontend dependencies.

---

## Prerequisites

- Go 1.21+
- Docker + Docker Compose

---

## Quick start

```bash
# 1. Clone
git clone https://github.com/maxlesscode/watchdog.git
cd watchdog

# 2. Configure
cp .env.example .env
# Edit .env — set DB_* and API_KEY at minimum

# 3. Start Postgres
docker compose up -d

# 4. Run
go run cmd/watchdog/main.go
```

Server starts on `:9999`. Adminer UI at `http://localhost:8080`. Swagger UI at `http://localhost:8081`.

---

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DB_HOST` | yes | — | Postgres host |
| `DB_PORT` | yes | — | Postgres port |
| `DB_USER` | yes | — | Postgres user |
| `DB_PASSWORD` | no | — | Postgres password |
| `DB_NAME` | yes | — | Postgres database name |
| `DB_SSL_MODE` | no | `disable` | Postgres SSL mode (`disable`, `require`, `verify-full`) |
| `API_KEY` | yes | — | Secret key for `X-API-Key` header |
| `SMTP_HOST` | no | — | SMTP server host (enables email alerts) |
| `SMTP_PORT` | no | — | SMTP port (587 for STARTTLS) |
| `SMTP_USER` | no | — | SMTP login username |
| `SMTP_PASS` | no | — | SMTP login password |
| `ALERT_EMAIL` | no | — | Recipient address for price alerts |
| `IS_DEV` | no | — | Any non-empty value enables stdout logging |
| `LOG_PATH` | no | `watchdog.log` | Log file path (`logs/watchdog.log` when running via Docker) |
| `SERVER_ADDR` | no | `:9999` | HTTP listen address |
| `CORS_ORIGINS` | no | — | Allowed CORS origins, comma-separated (`*` for wildcard — dev only) |
| `APP_VERSION` | no | `dev` | Docker image version tag |
| `APP_PORT` | no | `9999` | Host port binding for Docker |

Email alerts are disabled when `SMTP_HOST` is unset. If `SMTP_HOST` is set, all five SMTP variables are required — the server will refuse to start if any are missing.

---

## API reference

All endpoints except `GET /health` require the `X-API-Key` header.

### Products

| Method | Path | Description |
|---|---|---|
| `GET` | `/products` | List all tracked products |
| `GET` | `/products/{id}` | Get product by ID |
| `POST` | `/products` | Add a new product |
| `PUT` | `/products/{id}` | Full-replace a product (all fields required) |
| `DELETE` | `/products/{id}` | Remove a product |
| `GET` | `/products/{id}/history` | Price history for a product (newest first) |

**POST / PUT body:**

```json
{
  "name": "Mechanical Keyboard",
  "url": "https://example.com/product/123",
  "target_price": 89.99,
  "price_selector": ".price"
}
```

`price_selector` is optional. If omitted, Watchdog tries to extract price from schema.org JSON-LD embedded in the page.

### Admin

| Method | Path | Description |
|---|---|---|
| `POST` | `/admin/scrape` | Trigger a scrape cycle immediately (202 Accepted) |

### Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Liveness + DB reachability check |

**Response:**
```json
{ "status": "up", "time": "2025-01-01T12:00:00Z" }
```

---

## Error responses

All errors share a consistent shape:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "validation failed",
    "details": { "name": "required" }
  }
}
```

---

## Project layout

```
cmd/watchdog/       main entry point
internal/
  database/         PostgresStore — ProductStore interface + SQL
  errors/           SendError helper, error codes
  handlers/         HTTP handlers (Env struct)
  logger/           slog dual-writer (file + optional stdout)
  metrics/          expvar counters (scrape_total, scrape_errors, alerts_sent)
  middleware/       APIKeyMiddleware, LoggingMiddleware
  models/           Product, PriceHistory structs
  netutil/          IsPrivateIP — SSRF guard for outbound scrape requests
  notifier/         Notifier interface, SMTPNotifier
  scheduler/        Hourly price-fetch loop
  scraper/          Scraper interface, HTMLScraper
  validation/       ValidateProduct
```

---

## How price scraping works

1. Scheduler ticks every hour (or on demand via `/admin/scrape`).
2. For each product, `HTMLScraper.FetchPrice` fetches the URL.
3. It first looks for a schema.org JSON-LD `offers.price` field.
4. Falls back to `price_selector` CSS selector if JSON-LD is absent.
5. Price is stored in `products.actual_price` and appended to `price_history`.
6. When `actual_price <= target_price` and SMTP is configured, an alert email is sent (with 24h deduplication).
