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

// PostAccept handles POST /v1/accepts with additional accept-specific validation:
// - payload.task_id must be present and non-empty
// - referenced task must exist
// - accept signer must equal task signer
func (h *handlers) PostAccept(w http.ResponseWriter, r *http.Request) {
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

	if env.ObjectType != "accept" {
		util.WriteError(w, http.StatusBadRequest, "invalid_request",
			"object_type must be accept for this endpoint")
		return
	}

	if err := env.Verify(); err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid_signature", err.Error())
		return
	}

	// Accept-specific: payload.task_id must be present and non-empty
	taskID, ok := env.PayloadTaskID()
	if !ok {
		util.WriteError(w, http.StatusBadRequest, "invalid_request",
			"accept payload must contain a non-empty task_id")
		return
	}

	// Lookup referenced task
	task, err := h.repo.GetObjectByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			util.WriteError(w, http.StatusNotFound, "not_found",
				"referenced task not found: "+taskID)
			return
		}
		util.WriteError(w, http.StatusInternalServerError, "internal", "failed to lookup task")
		return
	}

	// Verify referenced object is actually a task
	if task.ObjectType != "task" {
		util.WriteError(w, http.StatusBadRequest, "invalid_request",
			"referenced object is not a task")
		return
	}

	// Accept signer must equal task signer
	if env.Signer.PubKey != task.Signer.PubKey {
		util.WriteError(w, http.StatusBadRequest, "invalid_request",
			"accept signer must match task signer")
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
