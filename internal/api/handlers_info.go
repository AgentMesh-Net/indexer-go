package api

import (
	"crypto/ed25519"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"github.com/AgentMesh-Net/indexer-go/internal/core/canonicaljson"
	"github.com/AgentMesh-Net/indexer-go/internal/util"
)

// chainInfo is the JSON shape for /v1/meta chains array.
type chainInfo struct {
	ChainID            int    `json:"chain_id"`
	SettlementContract string `json:"settlement_contract"`
	MinConfirmations   int    `json:"min_confirmations,omitempty"`
}

// metaSignPayload is the canonical payload that gets signed (sorted field names).
type metaSignPayload struct {
	Chains []chainInfo `json:"chains"`
	FeeBPS int         `json:"fee_bps"`
	Name   string      `json:"name"`
	URL    string      `json:"url"`
}

// GetHealth handles GET /v1/health
func (h *handlers) GetHealth(w http.ResponseWriter, r *http.Request) {
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"time":    time.Now().UTC().Format(time.RFC3339),
		"version": h.cfg.Version,
		"commit":  h.cfg.Commit,
	})
}

// GetMeta handles GET /v1/meta
func (h *handlers) GetMeta(w http.ResponseWriter, r *http.Request) {
	chains := make([]chainInfo, len(h.cfg.SupportedChains))
	for i, c := range h.cfg.SupportedChains {
		chains[i] = chainInfo{
			ChainID:            c.ChainID,
			SettlementContract: c.SettlementContract,
			MinConfirmations:   c.MinConfirmations,
		}
	}

	pubKeyHex, sigHex := h.signMeta(chains)

	resp := map[string]any{
		"name":       h.cfg.IndexerName,
		"url":        h.cfg.IndexerBaseURL,
		"owner":      h.cfg.IndexerOwner,
		"contact":    h.cfg.IndexerContact,
		"fee_bps":    h.cfg.FeeBPS,
		"chains":     chains,
		"public_key": pubKeyHex,
		"signature":  sigHex,
		"version":    h.cfg.Version,
	}
	util.WriteJSON(w, http.StatusOK, resp)
}

// GetInfo handles GET /v1/indexer/info (legacy, kept for backwards compat)
func (h *handlers) GetInfo(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"name":         h.cfg.IndexerName,
		"version":      h.cfg.Version,
		"service_time": time.Now().UTC().Format(time.RFC3339),
		"capabilities": map[string]any{
			"object_types":   []string{"task", "bid", "accept", "artifact"},
			"signature_algo": "ed25519",
			"canonical_json": "RFC8785-JCS",
		},
		"fee_bps": h.cfg.FeeBPS,
	}
	util.WriteJSON(w, http.StatusOK, resp)
}

// signMeta signs the canonical meta payload and returns (pubKeyHex, sigHex).
// Returns ("", "") if no signing key is configured.
func (h *handlers) signMeta(chains []chainInfo) (string, string) {
	if h.cfg.SigningKeyHex == "" {
		return "", ""
	}
	raw, err := hex.DecodeString(h.cfg.SigningKeyHex)
	if err != nil || len(raw) != 32 {
		log.Printf("invalid INDEXER_SIGNING_KEY: %v", err)
		return "", ""
	}
	privKey := ed25519.NewKeyFromSeed(raw)
	pubKey := privKey.Public().(ed25519.PublicKey)

	payload := metaSignPayload{
		Name:   h.cfg.IndexerName,
		URL:    h.cfg.IndexerBaseURL,
		FeeBPS: h.cfg.FeeBPS,
		Chains: chains,
	}
	canonical, err := canonicaljson.Canonicalize(payload)
	if err != nil {
		log.Printf("canonicalize meta payload: %v", err)
		return "", ""
	}
	sig := ed25519.Sign(privKey, canonical)
	return hex.EncodeToString(pubKey), hex.EncodeToString(sig)
}
