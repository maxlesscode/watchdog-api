package models

import "time"

// PriceHistory records a single scraped price for a product.
type PriceHistory struct {
	ID        int       `json:"id"`
	ProductID int       `json:"product_id"`
	Price     float64   `json:"price"`
	CheckedAt time.Time `json:"checked_at"`
}
