package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestDecodePubKey_Valid(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	b64 := base64.StdEncoding.EncodeToString(pub)

	got, err := DecodePubKey(b64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(got))
	}
}

func TestDecodePubKey_WrongLength(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err := DecodePubKey(b64)
	if err == nil {
		t.Error("expected error for wrong length")
	}
}

func TestDecodePubKey_URLSafeBase64Rejected(t *testing.T) {
	// Use URL-safe base64 characters
	_, err := DecodePubKey("abc-def_ghi=")
	if err == nil {
		t.Error("expected error for URL-safe base64")
	}
}

func TestDecodeSignature_Valid(t *testing.T) {
	sig := make([]byte, 64)
	b64 := base64.StdEncoding.EncodeToString(sig)

	got, err := DecodeSignature(b64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 64 {
		t.Errorf("expected 64 bytes, got %d", len(got))
	}
}

func TestDecodeSignature_WrongLength(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	_, err := DecodeSignature(b64)
	if err == nil {
		t.Error("expected error for wrong length")
	}
}

func TestVerifyEd25519_Valid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	msg := []byte("test message")
	sig := ed25519.Sign(priv, msg)

	if !VerifyEd25519(pub, msg, sig) {
		t.Error("expected valid signature to verify")
	}
}

func TestVerifyEd25519_Invalid(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	msg := []byte("test message")
	sig := make([]byte, 64)

	if VerifyEd25519(pub, msg, sig) {
		t.Error("expected invalid signature to fail verification")
	}
}
