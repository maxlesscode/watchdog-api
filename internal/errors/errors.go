package errors

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

type ctxKey int

const requestIDKey ctxKey = 0

// WithRequestID stores a request ID in ctx for propagation to error responses.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext retrieves the request ID stored by WithRequestID.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

const (
	CodeInternalError = "INTERNAL_SERVER_ERROR"
	CodeBadRequest    = "BAD_REQUEST"
	CodeNotFound      = "RESOURCE_NOT_FOUND"

	CodeAuthRequired     = "AUTH_REQUIRED"
	CodeAuthInvalidToken = "AUTH_INVALID_TOKEN"
	CodeAuthForbidden    = "FORBIDDEN_ACCESS"

	CodeValidationFailed = "VALIDATION_FAILED"
	CodeAlreadyExists    = "RESOURCE_ALREADY_EXISTS"
	CodeInvalidID        = "INVALID_RESOURCE_ID"

	CodeDatabaseError      = "DATABASE_ERROR"
	CodeRateLimitExceeded  = "RATE_LIMIT_EXCEEDED"
)

type APIError struct {
	Message   string            `json:"message"`
	Code      string            `json:"code"`
	RequestID string            `json:"request_id,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

// ErrorInput holds parameters for SendError.
type ErrorInput struct {
	Code    int
	Tech    string
	Message string
	Details map[string]string
}

func SendError(ctx context.Context, w http.ResponseWriter, in ErrorInput) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(in.Code)

	payload := APIError{
		Message:   in.Message,
		Code:      in.Tech,
		RequestID: RequestIDFromContext(ctx),
		Details:   in.Details,
	}

	if err := json.NewEncoder(w).Encode(map[string]APIError{"error": payload}); err != nil {
		slog.Warn("failed to send json error body", "err", err)
	}
}
