package api

import (
	"net/http"
	"time"

	"github.com/AgentMesh-Net/indexer-go/internal/util"
)

func (h *handlers) GetInfo(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"name":         "AgentMesh-Net Indexer",
		"version":      "0.1",
		"service_time": time.Now().UTC().Format(time.RFC3339),
		"capabilities": map[string]any{
			"object_types":   []string{"task", "bid", "accept", "artifact"},
			"signature_algo": "ed25519",
			"canonical_json": "RFC8785-JCS",
		},
		"fee_recipient": nil,
		"notes":         "v0.1 no settlement",
	}
	util.WriteJSON(w, http.StatusOK, resp)
}
