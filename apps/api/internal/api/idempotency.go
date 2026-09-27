package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/httpx"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

const idempotencyKeyHeader = "Idempotency-Key"

// idempotentHandler is a mutating handler that receives the already-read
// request body, so withIdempotency can hash it before any decoding
// happens.
type idempotentHandler func(w http.ResponseWriter, r *http.Request, body []byte) (statusCode int, responseBody any)

// storedResult is what gets persisted in idempotency_keys.result_reference:
// enough to replay the exact original HTTP response.
type storedResult struct {
	StatusCode int             `json:"statusCode"`
	Body       json.RawMessage `json:"body"`
}

// withIdempotency enforces this repo's idempotency rules for one mutating
// operation:
//   - a missing Idempotency-Key is a 400, not a silently-accepted mutation
//   - the same key + the same request (method+path+body) replays the
//     original response rather than re-running the handler
//   - the same key + a different request is a 409 — never silently
//     overwritten, never treated as the same operation
//   - a concurrent duplicate (the same key claimed a moment ago, still
//     being processed) is also a 409 rather than blocking indefinitely or
//     racing the in-flight request
//
// Every outcome is backed by the idempotency_keys table, not in-memory
// state, so it survives a process restart.
func withIdempotency(deps Deps, operation string, handler idempotentHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(idempotencyKeyHeader)
		if key == "" {
			httpx.WriteValidationError(w, "missing Idempotency-Key header", []httpx.FieldIssue{
				{Field: "Idempotency-Key", Issue: "is required for this request"},
			})
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			httpx.WriteValidationError(w, "could not read request body", []httpx.FieldIssue{
				{Field: "body", Issue: err.Error()},
			})
			return
		}

		requestHash := computeRequestHash(r.Method, r.URL.Path, body)

		_, err = deps.Store.InsertIdempotencyRecord(r.Context(), key, operation, requestHash)
		if err == nil {
			runAndPersist(deps, w, r, handler, body, key, operation)
			return
		}
		if !errors.Is(err, store.ErrIdempotencyKeyExists) {
			httpx.WriteInternal(w, deps.Logger, err, "InsertIdempotencyRecord")
			return
		}

		existing, err := deps.Store.GetIdempotencyRecord(r.Context(), key, operation)
		if err != nil {
			httpx.WriteInternal(w, deps.Logger, err, "GetIdempotencyRecord")
			return
		}

		if existing.RequestHash != requestHash {
			httpx.WriteConflict(w, "Idempotency-Key was already used with a different request")
			return
		}

		if existing.Status == "pending" || existing.ResultReference == nil {
			httpx.WriteConflict(w, "a request with this Idempotency-Key is already being processed")
			return
		}

		var result storedResult
		if err := json.Unmarshal([]byte(*existing.ResultReference), &result); err != nil {
			httpx.WriteInternal(w, deps.Logger, fmt.Errorf("decoding stored idempotent result: %w", err), "withIdempotency.replay")
			return
		}
		writeJSON(w, result.StatusCode, json.RawMessage(result.Body))
	}
}

func runAndPersist(deps Deps, w http.ResponseWriter, r *http.Request, handler idempotentHandler, body []byte, key, operation string) {
	rec := newCapturingResponseWriter()
	statusCode, responseBody := handler(rec, r, body)

	finalStatus := statusCode
	var finalBody any = responseBody
	if rec.wroteDirectly {
		// The handler wrote an error response (validation/not-found/etc.)
		// directly via one of the httpx helpers instead of returning a
		// success value — no workflow action was created. The claim is
		// released entirely (not just marked "failed") so a client that
		// fixes its request and retries with the same key can claim it
		// fresh, even though the corrected request now hashes differently.
		if err := deps.Store.DeleteIdempotencyRecord(r.Context(), key, operation); err != nil {
			deps.Logger.Error("failed to release idempotency claim after a failed attempt", "error", err)
		}
		copyRecordedResponse(w, rec)
		return
	}

	bodyJSON, err := json.Marshal(finalBody)
	if err != nil {
		httpx.WriteInternal(w, deps.Logger, err, "withIdempotency.marshalResult")
		return
	}
	stored, err := json.Marshal(storedResult{StatusCode: finalStatus, Body: bodyJSON})
	if err != nil {
		httpx.WriteInternal(w, deps.Logger, err, "withIdempotency.marshalStoredResult")
		return
	}
	storedStr := string(stored)
	if err := deps.Store.UpdateIdempotencyResult(r.Context(), key, operation, "completed", &storedStr); err != nil {
		httpx.WriteInternal(w, deps.Logger, err, "UpdateIdempotencyResult")
		return
	}

	writeJSON(w, finalStatus, finalBody)
}

func computeRequestHash(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// capturingResponseWriter lets withIdempotency detect whether the wrapped
// handler wrote a response itself (an error path, via the httpx helpers)
// rather than returning a (statusCode, body) pair through the normal
// idempotentHandler return values.
type capturingResponseWriter struct {
	header        http.Header
	statusCode    int
	body          []byte
	wroteDirectly bool
}

func newCapturingResponseWriter() *capturingResponseWriter {
	return &capturingResponseWriter{header: make(http.Header)}
}

func (c *capturingResponseWriter) Header() http.Header { return c.header }

func (c *capturingResponseWriter) Write(b []byte) (int, error) {
	c.wroteDirectly = true
	c.body = append(c.body, b...)
	return len(b), nil
}

func (c *capturingResponseWriter) WriteHeader(statusCode int) {
	c.wroteDirectly = true
	c.statusCode = statusCode
}

func copyRecordedResponse(w http.ResponseWriter, rec *capturingResponseWriter) {
	for k, values := range rec.header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	if rec.statusCode != 0 {
		w.WriteHeader(rec.statusCode)
	}
	_, _ = w.Write(rec.body)
}
