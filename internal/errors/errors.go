package errors

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

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

	CodeDatabaseError = "DATABASE_ERROR"
)

type APIError struct {
	Message string            `json:"message"`
	Code    string            `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}

func SendError(w http.ResponseWriter, code int, techCode string, message string, details ...map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	payload := APIError{
		Message: message,
		Code:    techCode,
	}

	if len(details) > 0 {
		payload.Details = details[0]
	}

	err := json.NewEncoder(w).Encode(map[string]APIError{"error": payload})
	if err != nil {
		slog.Warn("failed to send json error body", "err", err)
	}
}
