package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/AgentMesh-Net/indexer-go/internal/core/envelope"
	"github.com/AgentMesh-Net/indexer-go/internal/store"
	"github.com/AgentMesh-Net/indexer-go/internal/util"
)

// PostObject returns a handler that validates and stores an envelope of the given type.
func (h *handlers) PostObject(expectedType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, h.maxBody+1))
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid_request", "failed to read body")
			return
		}
		if int64(len(body)) > h.maxBody {
			util.WriteError(w, http.StatusRequestEntityTooLarge, "invalid_request", "body too large")
			return
		}

		var env envelope.Envelope
		if err := json.Unmarshal(body, &env); err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid_request", "invalid JSON: "+err.Error())
			return
		}

		if err := env.ValidateBasic(); err != nil {
			code := errorCode(err)
			util.WriteError(w, http.StatusBadRequest, code, err.Error())
			return
		}

		if env.ObjectType != expectedType {
			util.WriteError(w, http.StatusBadRequest, "invalid_request",
				"object_type must be "+expectedType+" for this endpoint")
			return
		}

		if err := env.Verify(); err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid_signature", err.Error())
			return
		}

		if err := h.repo.InsertObject(r.Context(), &env); err != nil {
			if errors.Is(err, store.ErrConflict) {
				util.WriteError(w, http.StatusConflict, "conflict", "object_id already exists")
				return
			}
			util.WriteError(w, http.StatusInternalServerError, "internal", "failed to store object")
			return
		}

		util.WriteJSON(w, http.StatusCreated, env)
	}
}

// ListObjects returns a handler that lists objects of the given type with pagination.
func (h *handlers) ListObjects(objectType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := util.ParseLimit(r, 50, 200)
		cursor := util.ParseCursor(r)

		items, next, err := h.repo.ListObjects(r.Context(), objectType, limit, cursor)
		if err != nil {
			util.WriteError(w, http.StatusInternalServerError, "internal", "failed to list objects")
			return
		}

		resp := map[string]any{
			"items": items,
		}
		if next != nil {
			resp["next_cursor"] = util.EncodeCursor(next)
		}
		util.WriteJSON(w, http.StatusOK, resp)
	}
}

func errorCode(err error) string {
	msg := err.Error()
	if contains(msg, "object_version") {
		return "unsupported_version"
	}
	if contains(msg, "signature") || contains(msg, "pubkey") || contains(msg, "base64") {
		return "invalid_signature"
	}
	return "invalid_request"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
