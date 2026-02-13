// Package ethutil provides Ethereum signature utilities.
package ethutil

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

// ErrInvalidSignature is returned when signature format is wrong.
var ErrInvalidSignature = errors.New("invalid signature")

// ErrSignerMismatch is returned when recovered address != expected.
var ErrSignerMismatch = errors.New("signer mismatch")

// Keccak256 computes the Ethereum/legacy keccak256 hash of data.
func Keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

// Keccak256Hex returns a 0x-prefixed hex keccak256 hash of data.
func Keccak256Hex(data []byte) string {
	return "0x" + hex.EncodeToString(Keccak256(data))
}

// eip191PersonalSignHash wraps msgHash with the Ethereum personal_sign prefix:
//   "\x19Ethereum Signed Message:\n32" + msgHash
// This matches MetaMask/ethers personal_sign behaviour.
func eip191PersonalSignHash(msgHash []byte) []byte {
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	full := make([]byte, 0, len(prefix)+len(msgHash))
	full = append(full, prefix...)
	full = append(full, msgHash...)
	return Keccak256(full)
}

// RecoverPersonalSign recovers the signer address from an EIP-191
// personal_sign signature over msgHash (the pre-computed message hash,
// i.e. keccak256(message)).
//
// sig must be 0x-prefixed hex of the 65-byte [R||S||V] signature as
// produced by MetaMask/ethers signMessage.
func RecoverPersonalSign(msgHash []byte, sig string) (string, error) {
	sigBytes, err := decodeHex(sig)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}
	if len(sigBytes) != 65 {
		return "", fmt.Errorf("%w: expected 65 bytes, got %d", ErrInvalidSignature, len(sigBytes))
	}

	// Normalise V: Ethereum personal_sign uses V=27/28; crypto.SigToPub expects V=0/1.
	sigBytes = append([]byte(nil), sigBytes...) // copy
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	prefixedHash := eip191PersonalSignHash(msgHash)

	pubKey, err := crypto.SigToPub(prefixedHash, sigBytes)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}

	addr := crypto.PubkeyToAddress(*pubKey)
	return strings.ToLower(addr.Hex()), nil
}

// VerifyPersonalSign verifies that signature was produced by the owner of
// expectedAddress over keccak256(message).
//
// message is the raw message bytes (NOT pre-hashed).
// expectedAddress is lowercase 0x-prefixed 20-byte hex address.
// sig is 0x-prefixed 65-byte hex signature.
func VerifyPersonalSign(message []byte, sig, expectedAddress string) error {
	msgHash := Keccak256(message)
	recovered, err := RecoverPersonalSign(msgHash, sig)
	if err != nil {
		return err
	}
	if !strings.EqualFold(recovered, expectedAddress) {
		return fmt.Errorf("%w: recovered=%s expected=%s", ErrSignerMismatch, recovered, expectedAddress)
	}
	return nil
}

// decodeHex decodes a 0x-or-plain hex string into bytes.
func decodeHex(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	return hex.DecodeString(s)
}
