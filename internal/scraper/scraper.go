package scraper

import "context"

// Scraper fetches the current price for a product URL.
type Scraper interface {
	FetchPrice(ctx context.Context, url, selector string) (float64, error)
}
