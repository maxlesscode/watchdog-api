package models

import "time"

type Product struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	ActualPrice   float64    `json:"actual_price"`
	TargetPrice   float64    `json:"target_price"`
	PriceSelector string     `json:"price_selector,omitempty"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
}
