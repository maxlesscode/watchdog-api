package metrics

import "expvar"

var (
	ScrapeTotal  = expvar.NewInt("scrape_total")
	ScrapeErrors = expvar.NewInt("scrape_errors")
	AlertsSent   = expvar.NewInt("alerts_sent")
)
