package envelope

import (
	"encoding/json"
	"strings"
	"testing"
)

// Test vectors generated with real ed25519 keys.
const testTaskJSON = `{
  "created_at": "2025-01-01T00:00:00Z",
  "object_id": "01J0000000000000000000TEST",
  "object_type": "task",
  "object_version": "0.1",
  "payload": {"description": "a test", "title": "test task"},
  "signature": "5vNLiFEPahJCdqvg8w7cRZhdMmEBh4OHfF00LV0xGCmU7x5Y4E8YklW+SjYXeCVRC0SxcegUllxfL6GLQA57Bg==",
  "signer": {"algo": "ed25519", "pubkey": "5pCB+DwMAPVHm8aabzPlBWx3kBVX94EOijtjcU4/Gzc="}
}`

const testAcceptJSON = `{
  "created_at": "2025-01-01T00:01:00Z",
  "object_id": "01J0000000000000000000ACPT",
  "object_type": "accept",
  "object_version": "0.1",
  "payload": {"task_id": "01J0000000000000000000TEST"},
  "signature": "NquujNYmexNWvu8m0X0UN5PngabR3ZMQ1PeVe0wIPa+ePFsAsQoRyYWfJ7dolKvnmBiV0d5EN6aYPOCEeSHNDA==",
  "signer": {"algo": "ed25519", "pubkey": "5pCB+DwMAPVHm8aabzPlBWx3kBVX94EOijtjcU4/Gzc="}
}`

const testDiffSignerAcceptJSON = `{
  "created_at": "2025-01-01T00:02:00Z",
  "object_id": "01J0000000000000000BADACPT",
  "object_type": "accept",
  "object_version": "0.1",
  "payload": {"task_id": "01J0000000000000000000TEST"},
  "signature": "KWs2KrlHMO6oZT37RbSIbmhgP/0VDeTGR9MtIcvH5B+scEysItygRD2IwC1X8cJStAh5KFm54bOSyX8L3XzHCw==",
  "signer": {"algo": "ed25519", "pubkey": "0pbUTLRh5NRYoWD7G7sGPGZY/0J6IWlvubhl6mnuUIs="}
}`

func TestVerify_ValidTask(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := env.ValidateBasic(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := env.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerify_ValidAccept(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testAcceptJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := env.ValidateBasic(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := env.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Tamper with the payload
	env.Payload = json.RawMessage(`{"title":"tampered","description":"evil"}`)

	if err := env.Verify(); err == nil {
		t.Fatal("expected verification to fail for tampered payload")
	}
}

func TestValidateBasic_MissingObjectID(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env.ObjectID = ""
	err := env.ValidateBasic()
	if err == nil {
		t.Fatal("expected error for missing object_id")
	}
}

func TestValidateBasic_WrongVersion(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env.ObjectVersion = "0.2"
	err := env.ValidateBasic()
	if err == nil {
		t.Fatal("expected error for wrong version")
	}
	if !strings.Contains(err.Error(), "unsupported object_version") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateBasic_InvalidObjectType(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env.ObjectType = "unknown"
	err := env.ValidateBasic()
	if err == nil {
		t.Fatal("expected error for invalid object_type")
	}
}

func TestValidateBasic_PayloadNotObject(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env.Payload = json.RawMessage(`"not an object"`)
	err := env.ValidateBasic()
	if err == nil {
		t.Fatal("expected error for non-object payload")
	}
}

func TestPayloadTaskID_Present(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testAcceptJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	taskID, ok := env.PayloadTaskID()
	if !ok {
		t.Fatal("expected task_id to be present")
	}
	if taskID != "01J0000000000000000000TEST" {
		t.Errorf("got %q, want %q", taskID, "01J0000000000000000000TEST")
	}
}

func TestPayloadTaskID_Missing(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	_, ok := env.PayloadTaskID()
	if ok {
		t.Fatal("expected task_id to not be present in task payload")
	}
}

func TestAcceptSignerMismatch(t *testing.T) {
	var task Envelope
	if err := json.Unmarshal([]byte(testTaskJSON), &task); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}

	var accept Envelope
	if err := json.Unmarshal([]byte(testDiffSignerAcceptJSON), &accept); err != nil {
		t.Fatalf("unmarshal accept: %v", err)
	}

	// Both should pass individual signature verification
	if err := task.Verify(); err != nil {
		t.Fatalf("task verify: %v", err)
	}
	if err := accept.Verify(); err != nil {
		t.Fatalf("accept verify: %v", err)
	}

	// But signer pubkeys should differ
	if accept.Signer.PubKey == task.Signer.PubKey {
		t.Fatal("expected different signer pubkeys for mismatch test")
	}
}

func TestAcceptMissingTaskID(t *testing.T) {
	envJSON := `{
		"object_type": "accept",
		"object_version": "0.1",
		"object_id": "test",
		"created_at": "2025-01-01T00:00:00Z",
		"payload": {},
		"signer": {"algo": "ed25519", "pubkey": "5pCB+DwMAPVHm8aabzPlBWx3kBVX94EOijtjcU4/Gzc="},
		"signature": "5vNLiFEPahJCdqvg8w7cRZhdMmEBh4OHfF00LV0xGCmU7x5Y4E8YklW+SjYXeCVRC0SxcegUllxfL6GLQA57Bg=="
	}`
	var env Envelope
	if err := json.Unmarshal([]byte(envJSON), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, ok := env.PayloadTaskID()
	if ok {
		t.Fatal("expected PayloadTaskID to return false for empty payload")
	}
}
