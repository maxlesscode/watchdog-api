CREATE TABLE products (
    id               SERIAL PRIMARY KEY,
    name             TEXT NOT NULL,
    url              TEXT NOT NULL,
    actual_price     NUMERIC,
    target_price     NUMERIC,
    price_selector   TEXT,
    last_checked_at  TIMESTAMPTZ,
    last_alerted_at  TIMESTAMPTZ
);

CREATE TABLE price_history (
    id         SERIAL PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    price      NUMERIC NOT NULL,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
