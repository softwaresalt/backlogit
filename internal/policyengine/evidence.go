package policyengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"time"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
	"github.com/softwaresalt/backlogit/internal/faultline"
)

// EvidenceSchemaVersion is the evidence record and bundle schema version.
const EvidenceSchemaVersion = 1

// Outcomes a record may carry.
const (
	// OutcomePass states that the recorded step succeeded.
	OutcomePass = "pass"
	// OutcomeFail states that the recorded step ran and failed.
	OutcomeFail = "fail"
)

// Evidence bounds. The bundle cap equals the shared decoder's document cap.
const (
	// MaxRecords caps the records in one bundle or request.
	MaxRecords = 16
	// MaxBundleBytes caps an evidence bundle file.
	MaxBundleBytes = evidencedag.MaxDeclarationBytes
	// MaxArtifactBytes caps a record's embedded artifact.
	MaxArtifactBytes = 192 << 10

	minRecordYear = 2000
	maxRecordYear = 2200
)

// ErrInvalidRecord classifies malformed evidence records and bundles.
var ErrInvalidRecord = errors.New("policyengine: invalid evidence record")

// Record is commit-SHA-bound evidence for one gate of one task. The digest
// covers every field, so an altered record no longer matches itself. A record
// is integrity-checked and verified against git history; it is not signed (see
// the design note's known limitations).
type Record struct {
	SchemaVersion int             `json:"schema_version"`
	TaskID        string          `json:"task_id"`
	Node          NodeType        `json:"node"`
	CommitSHA     string          `json:"commit_sha"`
	RecordedAt    time.Time       `json:"recorded_at"`
	Outcome       string          `json:"outcome"`
	Artifact      json.RawMessage `json:"artifact,omitempty"`
	Digest        string          `json:"digest"`
}

func knownNode(n NodeType) bool {
	for _, k := range NodeTypes() {
		if k == n {
			return true
		}
	}
	return false
}

func writeField(h hash.Hash, s string) {
	var l [8]byte
	binary.BigEndian.PutUint64(l[:], uint64(len(s)))
	h.Write(l[:])
	h.Write([]byte(s))
}

// checkShape validates every field of r except its digest.
func (r Record) checkShape() error {
	switch {
	case r.SchemaVersion != EvidenceSchemaVersion:
		return fmt.Errorf("unsupported record schema version %d", r.SchemaVersion)
	case !validTaskID(r.TaskID):
		return errors.New("task id is not a canonical backlog task ID")
	case !knownNode(r.Node):
		return fmt.Errorf("unknown node %q", clip(string(r.Node), 64))
	case !faultline.ValidCommitSHA(r.CommitSHA):
		return errors.New("commit_sha must be a full lowercase commit SHA")
	case r.RecordedAt.IsZero() || r.RecordedAt.Year() < minRecordYear || r.RecordedAt.Year() > maxRecordYear:
		return errors.New("recorded_at is absent or out of range")
	case r.Outcome != OutcomePass && r.Outcome != OutcomeFail:
		return fmt.Errorf("outcome must be %q or %q", OutcomePass, OutcomeFail)
	case len(r.Artifact) > MaxArtifactBytes:
		return fmt.Errorf("artifact exceeds %d bytes", MaxArtifactBytes)
	}
	if len(r.Artifact) > 0 && !json.Valid(r.Artifact) {
		return errors.New("artifact is not valid JSON")
	}
	return nil
}

// computeDigest digests every field of r other than Digest. The artifact is
// digested in compact form, so insignificant whitespace cannot change it.
func (r Record) computeDigest() (string, error) {
	var art string
	if len(r.Artifact) > 0 {
		var c bytes.Buffer
		if err := json.Compact(&c, r.Artifact); err != nil {
			return "", fmt.Errorf("compact artifact: %w", err)
		}
		sum := sha256.Sum256(c.Bytes())
		art = hex.EncodeToString(sum[:])
	}
	h := sha256.New()
	for _, f := range []string{
		"policyengine/record/v1", r.TaskID, string(r.Node), r.CommitSHA,
		fmt.Sprintf("%d", r.RecordedAt.UTC().UnixNano()), r.Outcome, art,
	} {
		writeField(h, f)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// NewRecord builds a validated, digested record.
func NewRecord(taskID string, node NodeType, commit string, at time.Time, outcome string, artifact json.RawMessage) (Record, error) {
	r := Record{
		SchemaVersion: EvidenceSchemaVersion, TaskID: taskID, Node: node, CommitSHA: commit,
		RecordedAt: at, Outcome: outcome, Artifact: artifact,
	}
	if err := r.checkShape(); err != nil {
		return Record{}, fmt.Errorf("%w: %s", ErrInvalidRecord, err.Error())
	}
	d, err := r.computeDigest()
	if err != nil {
		return Record{}, fmt.Errorf("%w: %s", ErrInvalidRecord, err.Error())
	}
	r.Digest = d
	return r, nil
}

// Bundle is the evidence file consumed by the CLI: the task, the head commit
// under evaluation, an optional pull request number and the records. It
// carries no operator identity, interactivity or override fields: unknown
// fields are refused, so none can be smuggled in through evidence.
type Bundle struct {
	SchemaVersion int      `json:"schema_version"`
	TaskID        string   `json:"task_id"`
	HeadCommit    string   `json:"head_commit"`
	PR            int      `json:"pr,omitempty"`
	Records       []Record `json:"records"`
}

// DecodeBundle strictly decodes an evidence bundle (JSON only). Unknown
// fields, duplicate keys (under Unicode folding), trailing data, oversized
// input and more than MaxRecords records are refused.
func DecodeBundle(data []byte) (Bundle, error) {
	var b Bundle
	if len(data) > MaxBundleBytes {
		return b, fmt.Errorf("%w: bundle exceeds %d bytes", ErrInvalidRecord, MaxBundleBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return b, fmt.Errorf("%w: bundle must be a JSON object", ErrInvalidRecord)
	}
	if err := evidencedag.DecodeStrict(trimmed, &b); err != nil {
		return Bundle{}, fmt.Errorf("%w: %s", ErrInvalidRecord, clip(err.Error(), maxErrorText))
	}
	if b.SchemaVersion != EvidenceSchemaVersion {
		return Bundle{}, fmt.Errorf("%w: unsupported bundle schema version %d", ErrInvalidRecord, b.SchemaVersion)
	}
	if len(b.Records) > MaxRecords {
		return Bundle{}, fmt.Errorf("%w: more than %d records", ErrInvalidRecord, MaxRecords)
	}
	return b, nil
}

// Request converts the bundle into an evaluation request for a surface. The
// request is never interactive and carries no override tokens.
func (b Bundle) Request(s Surface) Request {
	return Request{
		Surface: s, TaskID: b.TaskID, HeadCommit: b.HeadCommit, PR: b.PR,
		Records: append([]Record(nil), b.Records...),
	}
}
