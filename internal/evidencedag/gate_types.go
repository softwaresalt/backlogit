package evidencedag

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"
)

// Gate, waiver, trust and ledger bounds.
const (
	// MaxWaiverTokenBytes caps an encoded waiver token.
	MaxWaiverTokenBytes = 16 << 10
	// MaxWaiverLifetime caps ExpiresAt-NotBefore of a waiver token.
	MaxWaiverLifetime = 24 * time.Hour
	// MaxTrustBytes caps the trusted-key/policy configuration file.
	MaxTrustBytes = 64 << 10
	// MaxLedgerBytes caps the waiver ledger file.
	MaxLedgerBytes = 4 << 20
	// MaxLedgerRecords caps the number of ledger records.
	MaxLedgerRecords = 4096
	// TrustSchemaVersion is the trust configuration schema version.
	TrustSchemaVersion = 1
	// TokenSchemaVersion is the waiver token schema version.
	TokenSchemaVersion = 1
	// LedgerSchemaVersion is the ledger record schema version.
	LedgerSchemaVersion = 1
)

// Typed error categories.
var (
	// ErrWaiverDenied is the root of every waiver denial.
	ErrWaiverDenied = errors.New("evidencedag: waiver denied")
	// ErrNonceReplayed indicates a waiver nonce that was already consumed.
	ErrNonceReplayed = fmt.Errorf("%w: nonce replayed", ErrWaiverDenied)
	// ErrLedgerTampered indicates the ledger failed chain verification.
	ErrLedgerTampered = errors.New("evidencedag: ledger tampered")
	// ErrLedgerBusy indicates the ledger writer lock could not be acquired.
	ErrLedgerBusy = errors.New("evidencedag: ledger busy")
	// ErrSidecarStale indicates a ledger record was committed but its head
	// sidecar could not be refreshed; the next append heals it.
	ErrSidecarStale = errors.New("evidencedag: ledger record committed but head sidecar is stale")
	// ErrAnchorStale indicates a ledger record was committed but the external
	// anchor could not be advanced; further anchored appends fail closed until
	// an operator re-anchors with Ledger.AdoptHead.
	ErrAnchorStale = errors.New("evidencedag: ledger record committed but external anchor is stale")
	// ErrAnchorRequired indicates an anchored operation without an anchor store.
	ErrAnchorRequired = errors.New("evidencedag: an external ledger anchor is required")
	// ErrTrustInvalid indicates an invalid trust configuration.
	ErrTrustInvalid = errors.New("evidencedag: invalid trust configuration")
)

// KeyKind classifies who may hold a trusted signing key.
type KeyKind string

// Key kinds. Only human-held keys can authorize waivers; automation keys are
// recognised so they can be explicitly denied.
const (
	// KeyKindHuman marks a key held by an out-of-band human principal.
	KeyKindHuman KeyKind = "human"
	// KeyKindAutomation marks a key held by an agent or non-interactive process.
	KeyKindAutomation KeyKind = "automation"
)

// TrustedKey binds a public key to exactly one principal.
type TrustedKey struct {
	KeyID     string
	Principal string
	Kind      KeyKind
	PublicKey ed25519.PublicKey
}

// Trust is the out-of-band trust anchor: verification keys and the explicit
// authorization policy saying which principal may waive which node family.
type Trust struct {
	keys   map[string]TrustedKey
	policy map[string]map[Family]struct{}
}

// WaiverClaims is the signed payload of a waiver token. It is bound to one
// shipment, node and head commit and is valid only inside its time window.
type WaiverClaims struct {
	Nonce      string `json:"nonce"`
	ShipmentID string `json:"shipment_id"`
	NodeID     string `json:"node_id"`
	HeadCommit string `json:"head_commit"`
	Principal  string `json:"principal"`
	Kind       string `json:"kind"`
	Reason     string `json:"reason"`
	NotBefore  int64  `json:"not_before"`
	ExpiresAt  int64  `json:"expires_at"`
}

// waiverDomain separates waiver signatures from any other use of the keys.
const waiverDomain = "backlogit/evidencedag/waiver/v1\n"

// waiverToken is the wire form of a signed waiver.
type waiverToken struct {
	SchemaVersion int    `json:"schema_version"`
	KeyID         string `json:"key_id"`
	Payload       string `json:"payload"`
	Signature     string `json:"signature"`
}

// LedgerEntry is the caller-supplied content of one ledger record.
type LedgerEntry struct {
	AcceptedAt  int64
	ShipmentID  string
	NodeID      string
	HeadCommit  string
	Principal   string
	KeyID       string
	Family      string
	Kind        string
	Nonce       string
	Reason      string
	TokenDigest string
}

// LedgerRecord is one hash-chained, append-only waiver record.
type LedgerRecord struct {
	Version     int    `json:"v"`
	Seq         int    `json:"seq"`
	PrevHash    string `json:"prev_hash"`
	AcceptedAt  int64  `json:"accepted_at"`
	ShipmentID  string `json:"shipment_id"`
	NodeID      string `json:"node_id"`
	HeadCommit  string `json:"head_commit"`
	Principal   string `json:"principal"`
	KeyID       string `json:"key_id"`
	Family      string `json:"family"`
	Kind        string `json:"kind"`
	Nonce       string `json:"nonce"`
	Reason      string `json:"reason"`
	TokenDigest string `json:"token_digest"`
	Hash        string `json:"hash"`
}

// Anchor is an externally retained ledger head used to detect truncation,
// deletion and rollback. Seq 0 with an empty hash is the genesis (empty) head.
type Anchor struct {
	Seq  int    `json:"seq"`
	Hash string `json:"hash"`
}

// Ledger is an append-only, tamper-evident waiver ledger stored as a JSONL
// hash chain.
type Ledger struct {
	path string
	dir  string
	name string
}

// WaiverService verifies waiver tokens against the trust anchor and records
// accepted waivers in the ledger. Acceptance is anchored: it requires an
// external AnchorStore and fails closed without one.
type WaiverService struct {
	trust   *Trust
	ledger  *Ledger
	anchors AnchorStore
	now     func() time.Time
}

// Mode is the gate enforcement mode. There is no default: it must be explicit.
type Mode string

// Gate modes.
const (
	// ModeReportOnly reports the verdict but never blocks (rollout mode).
	ModeReportOnly Mode = "report-only"
	// ModeEnforce blocks review-ready unless every applicable terminal node is satisfied.
	ModeEnforce Mode = "enforce"
)

// Gate is the single authoritative review-ready gate over a (possibly
// composed) DAG.
type Gate struct {
	decl    *Declaration
	eval    *Evaluator
	mode    Mode
	waivers *WaiverService
}

// GateInput is one gate check request.
type GateInput struct {
	Shipment     Shipment
	Evidence     map[string]EvidenceRecord
	Failures     map[string]string
	WaiverTokens [][]byte
}

// WaiverDenial records a rejected waiver token.
type WaiverDenial struct {
	Index  int
	Reason string
}

// Decision is the gate outcome.
type Decision struct {
	// ReviewReady is the truthful verdict: every applicable terminal node is
	// verified or validly waived.
	ReviewReady bool
	// Allowed is whether the shipment may proceed: ReviewReady, or any verdict
	// when the gate runs report-only.
	Allowed bool
	Mode    Mode
	Rollup  Rollup
	Report  Report
	Denials []WaiverDenial
	Reason  string
}

// Contribution is a set of policy nodes (for example from S11) composed into
// the authoritative DAG.
type Contribution struct {
	Source string
	Nodes  []Node
	Edges  []Edge
}
