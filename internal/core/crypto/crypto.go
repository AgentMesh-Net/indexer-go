// Package crypto provides ed25519 signature verification and base64 decoding
// helpers for AgentMesh-Net protocol objects.
package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

// DecodePubKey decodes a standard base64 (RFC 4648 §4) public key string
// and validates it is exactly 32 bytes.
func DecodePubKey(s string) (ed25519.PublicKey, error) {
	b, err := decodeStdBase64(s)
	if err != nil {
		return nil, fmt.Errorf("pubkey: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("pubkey: expected %d bytes, got %d", ed25519.PublicKeySize, len(b))
	}
	return ed25519.PublicKey(b), nil
}

// DecodeSignature decodes a standard base64 (RFC 4648 §4) signature string
// and validates it is exactly 64 bytes.
func DecodeSignature(s string) ([]byte, error) {
	b, err := decodeStdBase64(s)
	if err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}
	if len(b) != ed25519.SignatureSize {
		return nil, fmt.Errorf("signature: expected %d bytes, got %d", ed25519.SignatureSize, len(b))
	}
	return b, nil
}

// VerifyEd25519 verifies an ed25519 signature over message bytes.
func VerifyEd25519(pubkey ed25519.PublicKey, message, sig []byte) bool {
	return ed25519.Verify(pubkey, message, sig)
}

// decodeStdBase64 decodes standard base64 (RFC 4648 §4 with '=' padding).
// URL-safe base64 is NOT accepted.
func decodeStdBase64(s string) ([]byte, error) {
	// Reject URL-safe base64 characters
	for _, c := range s {
		if c == '-' || c == '_' {
			return nil, fmt.Errorf("invalid base64: url-safe characters not allowed")
		}
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return b, nil
}
