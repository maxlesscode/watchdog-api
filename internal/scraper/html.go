package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var nonNumeric = regexp.MustCompile(`[^\d.]`)

// HTMLScraper fetches prices from HTML pages.
type HTMLScraper struct {
	client *http.Client
}

// NewHTMLScraper returns an HTMLScraper using the provided HTTP client.
func NewHTMLScraper(client *http.Client) *HTMLScraper {
	return &HTMLScraper{client: client}
}

// FetchPrice retrieves the price from url. It tries schema.org JSON-LD first,
// then falls back to selector. Returns an error if both strategies fail.
func (s *HTMLScraper) FetchPrice(ctx context.Context, url, selector string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Watchdog/1.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("fetch %s: unexpected status %d", url, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("parse html %s: %w", url, err)
	}

	if price, ok := extractJSONLD(doc); ok {
		return price, nil
	}

	if selector == "" {
		return 0, fmt.Errorf("no json-ld price found and no selector provided for %s", url)
	}

	text := strings.TrimSpace(doc.Find(selector).First().Text())
	if text == "" {
		return 0, fmt.Errorf("selector %q matched no text on %s", selector, url)
	}

	return parsePrice(text)
}

type ldOffer struct {
	Price interface{} `json:"price"`
}

type ldProduct struct {
	Offers ldOffer `json:"offers"`
}

func extractJSONLD(doc *goquery.Document) (float64, bool) {
	var price float64
	var found bool

	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var ld ldProduct
		if err := json.Unmarshal([]byte(s.Text()), &ld); err != nil {
			return true
		}
		switch v := ld.Offers.Price.(type) {
		case float64:
			price = v
			found = true
			return false
		case string:
			if p, err := parsePrice(v); err == nil {
				price = p
				found = true
				return false
			}
		}
		return true
	})

	return price, found
}

// parsePrice converts a localized price string to float64.
// Handles EU format ("1.299,99") and US format ("1,299.99").
func parsePrice(s string) (float64, error) {
	s = strings.TrimSpace(s)

	lastDot := strings.LastIndex(s, ".")
	lastComma := strings.LastIndex(s, ",")

	var normalized string
	switch {
	case lastDot > lastComma:
		// US format: 1,299.99 — strip thousands comma
		normalized = strings.ReplaceAll(s, ",", "")
	case lastComma > lastDot:
		// EU format: 1.299,99 — strip thousands dot, decimal comma → dot
		normalized = strings.ReplaceAll(s, ".", "")
		normalized = strings.ReplaceAll(normalized, ",", ".")
	default:
		normalized = s
	}

	cleaned := nonNumeric.ReplaceAllString(normalized, "")
	if cleaned == "" {
		return 0, fmt.Errorf("no numeric value in price string %q", s)
	}

	p, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("parse price %q: %w", s, err)
	}
	return p, nil
}
