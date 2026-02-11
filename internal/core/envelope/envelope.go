// Package envelope defines the AgentMesh-Net signed envelope structure
// and provides validation and signature verification.
package envelope

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/AgentMesh-Net/indexer-go/internal/core/canonicaljson"
	"github.com/AgentMesh-Net/indexer-go/internal/core/crypto"
)

// ValidObjectTypes enumerates the object types supported in v0.1.
var ValidObjectTypes = map[string]bool{
	"task":     true,
	"bid":      true,
	"accept":   true,
	"artifact": true,
}

// Signer represents the signer block in an envelope.
type Signer struct {
	Algo   string `json:"algo"`
	PubKey string `json:"pubkey"`
}

// Envelope represents a signed protocol object envelope.
type Envelope struct {
	ObjectType    string          `json:"object_type"`
	ObjectVersion string          `json:"object_version"`
	ObjectID      string          `json:"object_id"`
	CreatedAt     string          `json:"created_at"`
	Payload       json.RawMessage `json:"payload"`
	Signer        Signer          `json:"signer"`
	Signature     string          `json:"signature"`
}

// ValidateBasic checks that all required fields are present, correct types,
// and version/algo match v0.1 expectations.
func (e *Envelope) ValidateBasic() error {
	if !ValidObjectTypes[e.ObjectType] {
		return fmt.Errorf("invalid object_type: %q", e.ObjectType)
	}
	if e.ObjectVersion != "0.1" {
		return fmt.Errorf("unsupported object_version: %q", e.ObjectVersion)
	}
	if e.ObjectID == "" {
		return fmt.Errorf("object_id is required")
	}
	if e.CreatedAt == "" {
		return fmt.Errorf("created_at is required")
	}
	if _, err := time.Parse(time.RFC3339, e.CreatedAt); err != nil {
		if _, err2 := time.Parse(time.RFC3339Nano, e.CreatedAt); err2 != nil {
			return fmt.Errorf("created_at is not valid RFC3339: %w", err)
		}
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("payload is required")
	}
	// Ensure payload is a JSON object
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(e.Payload, &obj); err != nil {
		return fmt.Errorf("payload must be a JSON object: %w", err)
	}
	if e.Signer.Algo != "ed25519" {
		return fmt.Errorf("unsupported signer.algo: %q", e.Signer.Algo)
	}
	if e.Signer.PubKey == "" {
		return fmt.Errorf("signer.pubkey is required")
	}
	if e.Signature == "" {
		return fmt.Errorf("signature is required")
	}

	// Validate base64 decode lengths
	if _, err := crypto.DecodePubKey(e.Signer.PubKey); err != nil {
		return fmt.Errorf("signer.pubkey: %w", err)
	}
	if _, err := crypto.DecodeSignature(e.Signature); err != nil {
		return fmt.Errorf("signature: %w", err)
	}

	return nil
}

// SignedPreimageBytes returns the canonical JSON bytes of the envelope
// with the signature field removed, suitable for signature verification.
func (e *Envelope) SignedPreimageBytes() ([]byte, error) {
	// Build a map without the signature field
	m := map[string]any{
		"object_type":    e.ObjectType,
		"object_version": e.ObjectVersion,
		"object_id":      e.ObjectID,
		"created_at":     e.CreatedAt,
		"payload":        json.RawMessage(e.Payload),
		"signer": map[string]any{
			"algo":   e.Signer.Algo,
			"pubkey": e.Signer.PubKey,
		},
	}
	return canonicaljson.Canonicalize(m)
}

// Verify performs full signature verification: decodes the public key and
// signature, computes the signing preimage, and verifies the ed25519 signature.
func (e *Envelope) Verify() error {
	pubkey, err := crypto.DecodePubKey(e.Signer.PubKey)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	sig, err := crypto.DecodeSignature(e.Signature)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	preimage, err := e.SignedPreimageBytes()
	if err != nil {
		return fmt.Errorf("verify: preimage: %w", err)
	}
	if !crypto.VerifyEd25519(pubkey, preimage, sig) {
		return fmt.Errorf("verify: ed25519 signature verification failed")
	}
	return nil
}

// PayloadTaskID extracts the task_id field from the payload, if present.
func (e *Envelope) PayloadTaskID() (string, bool) {
	var p struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return "", false
	}
	if p.TaskID == "" {
		return "", false
	}
	return p.TaskID, true
}
