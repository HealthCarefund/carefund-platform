// Package httpx provides the API's shared HTTP error envelope. Every
// handler returns errors through this, never a bare status code or an ad
// hoc JSON shape, so clients get one consistent, discriminated error shape
// across the whole API (mirroring packages/types' ApiErrorShape on the
// frontend side, independently defined here for Go/JSON).
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorKind discriminates the cause of an API error response.
type ErrorKind string

const (
	ErrorKindValidation ErrorKind = "validation_error"
	ErrorKindNotFound   ErrorKind = "not_found"
	ErrorKindConflict   ErrorKind = "conflict"
	ErrorKindContract   ErrorKind = "contract_error"
	ErrorKindInternal   ErrorKind = "internal_error"
)

// FieldIssue is one field-level validation problem.
type FieldIssue struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// ErrorResponse is the JSON body returned for every non-2xx API response.
type ErrorResponse struct {
	Kind    ErrorKind    `json:"kind"`
	Message string       `json:"message"`
	Fields  []FieldIssue `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteValidationError responds 400 with field-level detail.
func WriteValidationError(w http.ResponseWriter, message string, fields []FieldIssue) {
	writeJSON(w, http.StatusBadRequest, ErrorResponse{Kind: ErrorKindValidation, Message: message, Fields: fields})
}

// WriteNotFound responds 404.
func WriteNotFound(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusNotFound, ErrorResponse{Kind: ErrorKindNotFound, Message: message})
}

// WriteConflict responds 409 (e.g. an idempotency key reused with a
// different request body).
func WriteConflict(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusConflict, ErrorResponse{Kind: ErrorKindConflict, Message: message})
}

// WriteContractError responds 502: the request was well-formed, but the
// chain itself rejected the underlying call. Message should carry the
// exact on-chain error code, never a guessed explanation.
func WriteContractError(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadGateway, ErrorResponse{Kind: ErrorKindContract, Message: message})
}

// WriteInternal logs the real error server-side (safe: no request bodies,
// headers, or secrets — just the error and enough context to find it in
// the logs) and responds 500 with a message that reveals nothing about
// the failure's internals.
func WriteInternal(w http.ResponseWriter, logger *slog.Logger, err error, context string) {
	logger.Error("internal error", "context", context, "error", err)
	writeJSON(w, http.StatusInternalServerError, ErrorResponse{
		Kind:    ErrorKindInternal,
		Message: "an internal error occurred",
	})
}
