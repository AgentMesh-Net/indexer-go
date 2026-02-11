package util

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AgentMesh-Net/indexer-go/internal/store"
)

// APIError represents a structured error response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the top-level error envelope.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a structured error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{
		Error: APIError{Code: code, Message: message},
	})
}

// ParseLimit extracts the limit query parameter with default and max bounds.
func ParseLimit(r *http.Request, defaultLimit, maxLimit int) int {
	s := r.URL.Query().Get("limit")
	if s == "" {
		return defaultLimit
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return defaultLimit
	}
	if n > maxLimit {
		return maxLimit
	}
	return n
}

// ParseCursor decodes the opaque cursor query parameter.
func ParseCursor(r *http.Request) *store.Cursor {
	s := r.URL.Query().Get("cursor")
	if s == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	var c store.Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil
	}
	if c.CreatedAt == "" || c.ObjectID == "" {
		return nil
	}
	return &c
}

// EncodeCursor encodes a cursor into an opaque string.
func EncodeCursor(c *store.Cursor) string {
	if c == nil {
		return ""
	}
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}
