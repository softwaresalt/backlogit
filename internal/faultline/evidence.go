// Package faultline defines the shared, version-aware fault-line evidence
// contract: a discriminated envelope (EvidenceArtifact) whose verified payload
// is a node-family-specific FamilyPayload validated against a foundation-owned
// per-version family manifest.
//
// This package is a standalone leaf. Its only intentional internal dependencies
// are internal/canonical (for deterministic serialization) and optionally
// internal/errors; it must never import detector, harness, core, or events
// packages so consumers can link the complete manifest set without an
// import-every-producer gap and without risking a dependency cycle.
//
// # Contract surface implemented by 156.006-T
//
// This file (156.004-T) lands only body-free-compilable declarations: the
// struct types, the FamilyPayload interface, the const and error-sentinel sets,
// the foundation-owned family manifest, and the registry scaffold. Go has no
// body-free function declaration, so the behavioral API below is documented
// here as the CONTRACT the behavior task (156.006-T) implements — it is NOT
// declared as a free function in this file:
//
//	func (a EvidenceArtifact) Canonical() ([]byte, error)
//	    Deterministic canonical bytes for the envelope + derived payload.
//
//	func DecodeAndValidate(data []byte) (EvidenceArtifact, FamilyPayload, error)
//	    Bounded decode of raw bytes into the envelope and its discriminated
//	    payload, then full validation. Returns typed error categories
//	    (ErrMalformed, ErrUnknownVersion, ErrUnknownFamily, ErrValidation) so a
//	    classifier can distinguish OutcomeUnknownOrMalformed from
//	    OutcomeValidationFailed via errors.Is.
//
//	func Validate(a EvidenceArtifact) error
//	    Envelope invariant checks, then derives the payload from
//	    a.VerifiedEvidence and delegates to FamilyPayload.Validate(view).
//
//	func RegisterFamily(version int, name string, factory func() FamilyPayload) error
//	    Init-only registration keyed by composite (version, name); errors on
//	    duplicate, nil factory, or post-freeze registration (no last-writer-wins).
//
//	func knownFamilies(version int) []string
//	    The manifest family set for a schema version (reads the frozen snapshot).
//
//	func freeze()
//	    Freezes the registry on the first of DecodeAndValidate/Canonical/
//	    Validate/knownFamilies; validates that registered factories match the
//	    manifest EXACTLY per version (fail closed on missing or extra).
package faultline

import (
	"encoding/json"
	"errors"
	"sync"
)

// EvidenceSchemaVersion is the current fault-line evidence schema version. Each
// version uniquely identifies a complete, closed family set via familyManifest.
const EvidenceSchemaVersion = 1

// NodeFamilyParity is the v1 node family for cross-surface parity evidence (S4).
const NodeFamilyParity = "parity"

// Envelope status values. Status describes the parity outcome an artifact
// records and is cross-checked against the family payload state matrix.
const (
	// StatusPass indicates conformant parity with no divergence.
	StatusPass = "pass"
	// StatusReportOnly indicates an expected, tracked divergence (report only).
	StatusReportOnly = "report_only"
	// StatusFail indicates an unexpected divergence (conformance failure).
	StatusFail = "fail"
)

// Outcome classifies a decode-and-validate result so callers can distinguish a
// malformed/unknown artifact from one that decoded cleanly but failed an
// invariant.
type Outcome int

const (
	// OutcomeAbsent indicates no evidence artifact was present.
	OutcomeAbsent Outcome = iota
	// OutcomeValid indicates a well-formed artifact that passed validation.
	OutcomeValid
	// OutcomeUnknownOrMalformed indicates a malformed artifact or an unknown
	// schema version or node family (ErrMalformed/ErrUnknownVersion/
	// ErrUnknownFamily).
	OutcomeUnknownOrMalformed
	// OutcomeValidationFailed indicates a known version and family whose
	// invariant checks failed (ErrValidation).
	OutcomeValidationFailed
)

// Bounded-decode limits (SEC-01). A raw artifact that exceeds any budget fails
// closed with ErrMalformed (resource-limit), preventing decode-bomb inputs.
const (
	// MaxArtifactBytes caps the raw artifact size accepted by DecodeAndValidate.
	MaxArtifactBytes = 1 << 20
	// MaxJSONDepth caps nested container depth during decode.
	MaxJSONDepth = 32
	// MaxObjectMembers caps the member count of any single JSON object.
	MaxObjectMembers = 256
	// MaxArrayElements caps the element count of any single JSON array.
	MaxArrayElements = 1024
	// MaxStringBytes caps the byte length of any single JSON string value.
	MaxStringBytes = 65536
)

// Typed error categories returned (wrapped) from DecodeAndValidate and Validate.
// Callers use errors.Is to classify outcomes without string matching.
var (
	// ErrMalformed indicates a structurally invalid artifact: duplicate or
	// case-variant keys, unknown fields, trailing data, or a bounded-decode
	// budget overrun.
	ErrMalformed = errors.New("malformed")
	// ErrUnknownVersion indicates a schema version absent from knownVersions.
	ErrUnknownVersion = errors.New("unknown schema version")
	// ErrUnknownFamily indicates a node family absent from the version manifest.
	ErrUnknownFamily = errors.New("unknown node family")
	// ErrValidation indicates a known version and family whose invariant checks
	// failed.
	ErrValidation = errors.New("validation failed")
)

// EvidenceArtifact is the shared, discriminated fault-line evidence envelope.
// VerifiedEvidence carries a node-family-specific payload (discriminated by
// NodeFamily), not one parity struct for all producers.
type EvidenceArtifact struct {
	SchemaVersion     int             `json:"schema_version"`
	ProducingTask     string          `json:"producing_task"`
	ProducingCommit   string          `json:"producing_commit"`
	NodeFamily        string          `json:"node_family"`
	Applicability     []string        `json:"applicability"`
	ValidatorIdentity string          `json:"validator_identity"`
	VerifiedEvidence  json.RawMessage `json:"verified_evidence"`
	Status            string          `json:"status"`
}

// EvidenceView is a foundation-owned, read-only projection of the envelope that
// a FamilyPayload uses to enforce its own cross-envelope invariants without the
// foundation importing producer packages (which would reverse the dependency).
type EvidenceView struct {
	// Status is the envelope Status value.
	Status string
	// Applicability is the envelope Applicability set.
	Applicability []string
	// SchemaVersion is the envelope SchemaVersion.
	SchemaVersion int
	// NodeFamily is the envelope NodeFamily discriminator.
	NodeFamily string
}

// ParityEvidence is the v1 "parity" family payload: cross-surface parity
// evidence for a single governed scenario dimension.
type ParityEvidence struct {
	ScenarioID         string   `json:"scenario_id"`
	Dimension          string   `json:"dimension"`
	Surfaces           []string `json:"surfaces"`
	DivergentFields    []string `json:"divergent_fields"`
	ExpectedDivergence bool     `json:"expected_divergence"`
	TrackedDefect      string   `json:"tracked_defect"`
	Detail             string   `json:"detail"`
}

// FamilyPayload is the discriminated node-family payload contract. Each family
// exposes a canonical map view and validates its own invariants against the
// foundation-owned EvidenceView.
type FamilyPayload interface {
	// CanonicalMap returns the payload as a canonicalization-ready map.
	CanonicalMap() map[string]any
	// Validate enforces the family's invariants against the envelope view.
	Validate(view EvidenceView) error
}

// familyManifest is the foundation-owned, per-version family manifest. It is
// binary-independent (static, not derived from linked packages) so two binaries
// can never freeze different family sets for a version. Adding or removing a
// family REQUIRES a version bump; the v1 set is {parity} forever.
//
//nolint:unused // Consumed by knownFamilies/freeze in 156.006-T (red-deliverable scaffold).
var familyManifest = map[int][]string{
	EvidenceSchemaVersion: {NodeFamilyParity},
}

// knownVersions is the set of accepted schema versions. A version absent here
// yields ErrUnknownVersion.
//
//nolint:unused // Consumed by Validate/DecodeAndValidate in 156.006-T (red-deliverable scaffold).
var knownVersions = map[int]struct{}{
	EvidenceSchemaVersion: {},
}

// registry holds the frozen family-factory snapshot keyed by composite
// (schemaVersion, familyName). Once frozen, all readers observe the same
// immutable snapshot so there is no register-vs-read race.
//
//nolint:unused // Consumed by RegisterFamily/freeze/knownFamilies in 156.006-T (red-deliverable scaffold).
type registry struct {
	mu        sync.Mutex
	frozen    bool
	factories map[int]map[string]func() FamilyPayload
}

// globalRegistry is the package-level family registry.
//
//nolint:unused // Consumed by RegisterFamily/freeze/knownFamilies in 156.006-T (red-deliverable scaffold).
var globalRegistry = &registry{
	factories: make(map[int]map[string]func() FamilyPayload),
}
