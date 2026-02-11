// Package canonicaljson implements RFC 8785 (JCS) JSON canonicalization.
package canonicaljson

import (
	"encoding/json"
	"fmt"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

// Canonicalize takes a Go value, marshals it to JSON, then applies RFC 8785 JCS
// canonicalization and returns the canonical UTF-8 bytes.
func Canonicalize(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonicaljson: marshal: %w", err)
	}
	return CanonicalizeRaw(raw)
}

// CanonicalizeRaw takes raw JSON bytes and returns RFC 8785 canonical form.
func CanonicalizeRaw(raw json.RawMessage) ([]byte, error) {
	out, err := jsoncanonicalizer.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("canonicaljson: transform: %w", err)
	}
	return out, nil
}
