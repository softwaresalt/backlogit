package gateproof

import (
	stderrors "errors"
	"strings"
	"testing"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func validTaskEnvelope() Envelope {
	return Envelope{
		Magic:        Magic,
		Purpose:      PurposeTask,
		Schema:       Schema,
		Alg:          AlgHMACSHA256,
		KeyID:        "key-2026-08",
		WorkspaceID:  "ws-abc123",
		ItemID:       "106.003-T",
		EventType:    "pre_task_completion_gate_passed",
		Ran:          true,
		Actor:        "backlogit",
		TimestampUTC: "2026-08-08T00:00:00Z",
		HeadSHA:      "deadbeef",
		ReportDigest: "a" + strings.Repeat("0", 63),
		Counter:      1,
	}
}

func validShipmentEnvelope() Envelope {
	e := validTaskEnvelope()
	e.Purpose = PurposeShipment
	e.ItemID = "117-S"
	e.ManifestDigest = "b" + strings.Repeat("0", 63)
	return e
}

func testKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef") // 33 bytes, >= 32
}

func TestSignVerify_RoundTrip(t *testing.T) {
	for _, env := range []Envelope{validTaskEnvelope(), validShipmentEnvelope()} {
		mac, err := Sign(env, testKey())
		if err != nil {
			t.Fatalf("Sign() unexpected error: %v", err)
		}
		if mac == "" {
			t.Fatal("Sign() returned empty MAC")
		}
		if err := Verify(env, mac, testKey()); err != nil {
			t.Fatalf("Verify() unexpected error on valid round-trip: %v", err)
		}
	}
}

func TestVerify_TamperedFieldRejected(t *testing.T) {
	env := validTaskEnvelope()
	mac, err := Sign(env, testKey())
	if err != nil {
		t.Fatalf("Sign() unexpected error: %v", err)
	}

	tampered := env
	tampered.Counter = env.Counter + 1
	if err := Verify(tampered, mac, testKey()); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Verify(tampered) error = %v, want ErrProofInvalid", err)
	}
}

func TestVerify_WrongKeyRejected(t *testing.T) {
	env := validTaskEnvelope()
	mac, err := Sign(env, testKey())
	if err != nil {
		t.Fatalf("Sign() unexpected error: %v", err)
	}
	wrongKey := []byte("ffffffffffffffffffffffffffffffff")
	if err := Verify(env, mac, wrongKey); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Verify(wrong key) error = %v, want ErrProofInvalid", err)
	}
}

func TestVerify_UnknownSchemaRejected(t *testing.T) {
	env := validTaskEnvelope()
	env.Schema = 999
	if _, err := Sign(env, testKey()); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Sign(unknown schema) error = %v, want ErrProofInvalid", err)
	}
	// Even a MAC computed some other way must still be rejected at verify time.
	if err := Verify(env, "00", testKey()); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Verify(unknown schema) error = %v, want ErrProofInvalid", err)
	}
}

func TestSign_TaskEnvelopeCarryingManifestDigestRejected(t *testing.T) {
	env := validTaskEnvelope()
	env.ManifestDigest = "should-not-be-set-for-task"
	if _, err := Sign(env, testKey()); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Sign(task with manifest_digest) error = %v, want ErrProofInvalid", err)
	}
}

func TestSign_ShipmentEnvelopeMissingManifestDigestRejected(t *testing.T) {
	env := validShipmentEnvelope()
	env.ManifestDigest = ""
	if _, err := Sign(env, testKey()); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Sign(shipment missing manifest_digest) error = %v, want ErrProofInvalid", err)
	}
}

func TestVerify_MalformedMACRejected(t *testing.T) {
	env := validTaskEnvelope()
	if err := Verify(env, "not-hex-!!!", testKey()); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Verify(malformed mac) error = %v, want ErrProofInvalid", err)
	}
	if err := Verify(env, "aa", testKey()); !stderrors.Is(err, bkerrors.ErrProofInvalid) {
		t.Fatalf("Verify(short mac) error = %v, want ErrProofInvalid", err)
	}
}

func TestSign_KeyTooShortRejected(t *testing.T) {
	env := validTaskEnvelope()
	_, err := Sign(env, []byte("short"))
	if err == nil {
		t.Fatal("Sign() with too-short key should fail")
	}
}
