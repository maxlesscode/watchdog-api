package errors_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apierrors "github.com/maxlesscode/watchdog/internal/errors"
)

func TestRequestIDRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := apierrors.WithRequestID(context.Background(), "req-abc-123")
	if got := apierrors.RequestIDFromContext(ctx); got != "req-abc-123" {
		t.Errorf("got %q, want %q", got, "req-abc-123")
	}
}

func TestRequestIDFromContext_Missing(t *testing.T) {
	t.Parallel()

	if got := apierrors.RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSendError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		ctx         context.Context
		in          apierrors.ErrorInput
		wantStatus  int
		wantCode    string
		wantMsg     string
		wantReqID   string
		wantDetails map[string]string
	}{
		{
			name:       "sets status code and Content-Type",
			ctx:        context.Background(),
			in:         apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeBadRequest, Message: "bad input"},
			wantStatus: http.StatusBadRequest,
			wantCode:   apierrors.CodeBadRequest,
			wantMsg:    "bad input",
		},
		{
			name:       "propagates request ID from context",
			ctx:        apierrors.WithRequestID(context.Background(), "trace-xyz"),
			in:         apierrors.ErrorInput{Code: http.StatusNotFound, Tech: apierrors.CodeNotFound, Message: "not found"},
			wantStatus: http.StatusNotFound,
			wantReqID:  "trace-xyz",
		},
		{
			name:       "omits request_id when not in context",
			ctx:        context.Background(),
			in:         apierrors.ErrorInput{Code: http.StatusInternalServerError, Tech: apierrors.CodeInternalError, Message: "oops"},
			wantStatus: http.StatusInternalServerError,
			wantReqID:  "",
		},
		{
			name: "includes details when provided",
			ctx:  context.Background(),
			in: apierrors.ErrorInput{
				Code:    http.StatusBadRequest,
				Tech:    apierrors.CodeValidationFailed,
				Message: "validation failed",
				Details: map[string]string{"name": "is required", "url": "is required"},
			},
			wantStatus:  http.StatusBadRequest,
			wantDetails: map[string]string{"name": "is required", "url": "is required"},
		},
		{
			name:       "omits details key when nil",
			ctx:        context.Background(),
			in:         apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeBadRequest, Message: "bad"},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			apierrors.SendError(tt.ctx, w, tt.in)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var envelope map[string]apierrors.APIError
			if err := json.NewDecoder(w.Body).Decode(&envelope); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			apiErr, ok := envelope["error"]
			if !ok {
				t.Fatal("response body missing top-level 'error' key")
			}

			if tt.wantCode != "" && apiErr.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", apiErr.Code, tt.wantCode)
			}
			if tt.wantMsg != "" && apiErr.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", apiErr.Message, tt.wantMsg)
			}
			if apiErr.RequestID != tt.wantReqID {
				t.Errorf("request_id = %q, want %q", apiErr.RequestID, tt.wantReqID)
			}
			for k, v := range tt.wantDetails {
				if apiErr.Details[k] != v {
					t.Errorf("details[%q] = %q, want %q", k, apiErr.Details[k], v)
				}
			}
			if tt.wantDetails == nil && len(apiErr.Details) > 0 {
				t.Errorf("expected no details, got %v", apiErr.Details)
			}
		})
	}
}
