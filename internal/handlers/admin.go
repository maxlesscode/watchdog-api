package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/maxlesscode/watchdog/internal/errors"
)

// AdminScrape triggers one full scrape cycle immediately and returns 202.
func (e *Env) AdminScrape(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close() //nolint:all

	if e.TriggerScrape == nil {
		errors.SendError(r.Context(), w, errors.ErrorInput{Code: http.StatusServiceUnavailable, Tech: errors.CodeInternalError, Message: "scraper not configured"})
		return
	}

	go e.TriggerScrape(context.WithoutCancel(r.Context()))

	slog.Info("manual scrape cycle triggered")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "scrape cycle started"}); err != nil {
		slog.Warn("failed to encode admin scrape response", "err", err)
	}
}
