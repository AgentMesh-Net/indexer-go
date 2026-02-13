package ethutil_test

import (
	"crypto/ecdsa"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/AgentMesh-Net/indexer-go/internal/ethutil"
)

// genKey creates a fresh ECDSA key and returns the key + lowercase address.
func genKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	addr := crypto.PubkeyToAddress(key.PublicKey).Hex()
	return key, addr
}

// personalSign produces an EIP-191 personal_sign signature (V=27/28) for the
// given message using key. This mirrors what MetaMask/ethers do.
func personalSign(t *testing.T, key *ecdsa.PrivateKey, message []byte) string {
	t.Helper()
	msgHash := ethutil.Keccak256(message)
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	full := append(prefix, msgHash...)
	prefixedHash := ethutil.Keccak256(full)

	sig, err := crypto.Sign(prefixedHash, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	sig[64] += 27 // convert V from 0/1 to 27/28

	return "0x" + func() string {
		const hex = "0123456789abcdef"
		out := make([]byte, len(sig)*2)
		for i, b := range sig {
			out[i*2] = hex[b>>4]
			out[i*2+1] = hex[b&0xf]
		}
		return string(out)
	}()
}

// ── Tests ──────────────────────────────────────────────────────────────────────

func TestVerifyPersonalSign_EmployerFlow(t *testing.T) {
	key, addr := genKey(t)
	message := []byte("task-employer-001")

	sig := personalSign(t, key, message)

	if err := ethutil.VerifyPersonalSign(message, sig, addr); err != nil {
		t.Fatalf("expected valid sig, got: %v", err)
	}
}

func TestVerifyPersonalSign_WorkerFlow(t *testing.T) {
	key, addr := genKey(t)
	// Worker message: keccak256(task_id + accept_id)
	message := []byte("task-001" + "accept-001")

	sig := personalSign(t, key, message)

	if err := ethutil.VerifyPersonalSign(message, sig, addr); err != nil {
		t.Fatalf("expected valid sig, got: %v", err)
	}
}

func TestVerifyPersonalSign_WrongSigner(t *testing.T) {
	key, _ := genKey(t)
	_, otherAddr := genKey(t) // different address
	message := []byte("task-002")

	sig := personalSign(t, key, message)

	err := ethutil.VerifyPersonalSign(message, sig, otherAddr)
	if err == nil {
		t.Fatal("expected error for wrong signer, got nil")
	}
	if !errors.Is(err, ethutil.ErrSignerMismatch) {
		t.Fatalf("expected ErrSignerMismatch, got: %v", err)
	}
}

func TestVerifyPersonalSign_BadSigFormat(t *testing.T) {
	_, addr := genKey(t)

	err := ethutil.VerifyPersonalSign([]byte("task-003"), "0xdeadbeef", addr)
	if err == nil {
		t.Fatal("expected error for malformed sig, got nil")
	}
	if !errors.Is(err, ethutil.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got: %v", err)
	}
}

func TestVerifyPersonalSign_WrongMessage(t *testing.T) {
	key, addr := genKey(t)

	// Sign one message, verify with a different one
	sig := personalSign(t, key, []byte("original-message"))

	err := ethutil.VerifyPersonalSign([]byte("different-message"), sig, addr)
	if err == nil {
		t.Fatal("expected error for wrong message, got nil")
	}
	if !errors.Is(err, ethutil.ErrSignerMismatch) {
		t.Fatalf("expected ErrSignerMismatch, got: %v", err)
	}
}

func TestKeccak256Hex(t *testing.T) {
	// Known keccak256("") = c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
	got := ethutil.Keccak256Hex([]byte(""))
	want := "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	if got != want {
		t.Fatalf("Keccak256Hex(\"\") = %s, want %s", got, want)
	}
}
