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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/softwaresalt/backlogit/internal/canonical"
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
var familyManifest = map[int][]string{
	EvidenceSchemaVersion: {NodeFamilyParity},
}

// knownVersions is the set of accepted schema versions. A version absent here
// yields ErrUnknownVersion.
var knownVersions = map[int]struct{}{
	EvidenceSchemaVersion: {},
}

// registry holds the frozen family-factory snapshot keyed by composite
// (schemaVersion, familyName). Once frozen, all readers observe the same
// immutable snapshot so there is no register-vs-read race.
type registry struct {
	mu        sync.Mutex
	frozen    bool
	factories map[int]map[string]func() FamilyPayload
}

// globalRegistry is the package-level family registry.
var globalRegistry = &registry{
	factories: make(map[int]map[string]func() FamilyPayload),
}

// Canonical returns deterministic canonical bytes for the envelope + derived
// payload. It freezes the registry, derives the family payload from
// VerifiedEvidence, normalizes set-like fields (sorted + duplicate-free) so the
// digest is stable regardless of source ordering, and routes the merged map
// through internal/canonical.
func (a EvidenceArtifact) Canonical() ([]byte, error) {
	freeze()
	fp, err := decodePayload(a)
	if err != nil {
		return nil, err
	}
	m := map[string]any{
		"schema_version":     a.SchemaVersion,
		"producing_task":     a.ProducingTask,
		"producing_commit":   a.ProducingCommit,
		"node_family":        a.NodeFamily,
		"applicability":      sortedAnySet(a.Applicability),
		"validator_identity": a.ValidatorIdentity,
		"verified_evidence":  fp.CanonicalMap(),
		"status":             a.Status,
	}
	return canonical.Canonicalize(m)
}

// DecodeAndValidate bounded-decodes raw bytes into the envelope and its
// discriminated payload, then fully validates. It returns typed error
// categories (ErrMalformed, ErrUnknownVersion, ErrUnknownFamily, ErrValidation)
// so a classifier can distinguish outcomes via errors.Is.
func DecodeAndValidate(data []byte) (EvidenceArtifact, FamilyPayload, error) {
	freeze()
	var a EvidenceArtifact

	// (0) Bounded decode (SEC-01): reject oversized input before any parse.
	if len(data) > MaxArtifactBytes {
		return a, nil, fmt.Errorf("%w: artifact exceeds %d bytes (resource-limit)", ErrMalformed, MaxArtifactBytes)
	}

	// (1) Token-stream pre-scan: enforce structural budgets, reject duplicate
	// member names (case-sensitive AND case-fold-equal) anywhere in the tree,
	// and collect the root object keys for envelope exact-casing.
	rootKeys, err := preScan(data)
	if err != nil {
		return a, nil, err
	}
	envTags := structJSONTags(EvidenceArtifact{})
	for _, k := range rootKeys {
		if _, ok := envTags[k]; !ok {
			return a, nil, fmt.Errorf("%w: unknown or miscased envelope field %q", ErrMalformed, k)
		}
	}

	// (2) Two-phase peek: reject an unknown schema version with a typed error
	// before the strict decode so version skew is distinguishable.
	var peek struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return a, nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if _, ok := knownVersions[peek.SchemaVersion]; !ok {
		return a, nil, fmt.Errorf("%w: schema_version %d", ErrUnknownVersion, peek.SchemaVersion)
	}

	// (3) Strict envelope decode with unknown-field and trailing-data rejection.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return a, nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return a, nil, fmt.Errorf("%w: trailing data after artifact", ErrMalformed)
	}

	// (4) Family lookup by (SchemaVersion, NodeFamily) from the frozen registry.
	factory, ok := rawLookup(a.SchemaVersion, a.NodeFamily)
	if !ok {
		return a, nil, fmt.Errorf("%w: %q at v%d", ErrUnknownFamily, a.NodeFamily, a.SchemaVersion)
	}
	if len(a.VerifiedEvidence) == 0 {
		return a, nil, fmt.Errorf("%w: verified_evidence missing", ErrMalformed)
	}
	fp := factory()
	// Validate nested verified_evidence exact casing against the family's tags
	// (encoding/json alone matches case-insensitively, so this catches miscased
	// family fields that DisallowUnknownFields would silently accept).
	veKeys, err := preScan(a.VerifiedEvidence)
	if err != nil {
		return a, nil, err
	}
	famTags := structJSONTags(fp)
	for _, k := range veKeys {
		if _, ok := famTags[k]; !ok {
			return a, nil, fmt.Errorf("%w: unknown or miscased verified_evidence field %q", ErrMalformed, k)
		}
	}
	veDec := json.NewDecoder(bytes.NewReader(a.VerifiedEvidence))
	veDec.DisallowUnknownFields()
	if err := veDec.Decode(fp); err != nil {
		return a, nil, fmt.Errorf("%w: verified_evidence: %v", ErrMalformed, err)
	}

	// (5) Full validation (derives payload from a.VerifiedEvidence).
	if err := Validate(a); err != nil {
		return a, nil, err
	}
	return a, fp, nil
}

// Validate enforces envelope invariants, then derives the payload from
// a.VerifiedEvidence and delegates to FamilyPayload.Validate(view).
func Validate(a EvidenceArtifact) error {
	freeze()
	if _, ok := knownVersions[a.SchemaVersion]; !ok {
		return fmt.Errorf("%w: schema_version %d", ErrUnknownVersion, a.SchemaVersion)
	}
	if a.ProducingTask == "" || a.ProducingCommit == "" || a.ValidatorIdentity == "" {
		return fmt.Errorf("%w: producing_task, producing_commit and validator_identity are required", ErrValidation)
	}
	if !containsStr(knownFamilies(a.SchemaVersion), a.NodeFamily) {
		return fmt.Errorf("%w: %q at v%d", ErrUnknownFamily, a.NodeFamily, a.SchemaVersion)
	}
	if len(a.Applicability) == 0 {
		return fmt.Errorf("%w: applicability is required", ErrValidation)
	}
	for _, s := range a.Applicability {
		if !validSurface(s) {
			return fmt.Errorf("%w: applicability surface %q not in {cli, mcp, internal}", ErrValidation, s)
		}
	}
	if hasDuplicate(a.Applicability) {
		return fmt.Errorf("%w: applicability contains duplicate surfaces", ErrValidation)
	}
	if !validStatus(a.Status) {
		return fmt.Errorf("%w: status %q not in {pass, report_only, fail}", ErrValidation, a.Status)
	}
	fp, err := decodePayload(a)
	if err != nil {
		return err
	}
	view := EvidenceView{
		Status:        a.Status,
		Applicability: a.Applicability,
		SchemaVersion: a.SchemaVersion,
		NodeFamily:    a.NodeFamily,
	}
	return fp.Validate(view)
}

// RegisterFamily registers a family factory keyed by composite (version, name).
// It errors on a nil factory, a duplicate (version, name), or post-freeze
// registration (no last-writer-wins).
func RegisterFamily(version int, name string, factory func() FamilyPayload) error {
	return globalRegistry.register(version, name, factory)
}

// knownFamilies returns the manifest family set for a schema version, reading
// the frozen snapshot.
func knownFamilies(version int) []string {
	freeze()
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	regv := globalRegistry.factories[version]
	out := make([]string, 0, len(regv))
	for name := range regv {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// freeze freezes the global registry on first use and validates that the
// registered factories match familyManifest EXACTLY per version. Any missing or
// extra family is fatal (fail closed).
func freeze() {
	if err := globalRegistry.freezeAgainst(familyManifest); err != nil {
		panic(fmt.Sprintf("faultline: freeze: %v", err))
	}
}

// rawLookup returns the registered factory function for (version, name) under
// the registry lock, or (nil, false) when absent.
func rawLookup(version int, name string) (func() FamilyPayload, bool) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	regv := globalRegistry.factories[version]
	if regv == nil {
		return nil, false
	}
	f, ok := regv[name]
	return f, ok
}

// lookupFactory resolves (version, name) from the frozen registry and returns a
// fresh FamilyPayload instance. It returns ErrUnknownVersion when the schema
// version is absent from knownVersions and ErrUnknownFamily when the family is
// absent from the frozen registry for that version.
func lookupFactory(version int, name string) (FamilyPayload, error) {
	freeze()
	if _, ok := knownVersions[version]; !ok {
		return nil, fmt.Errorf("%w: schema_version %d", ErrUnknownVersion, version)
	}
	factory, ok := rawLookup(version, name)
	if !ok {
		return nil, fmt.Errorf("%w: %q at v%d", ErrUnknownFamily, name, version)
	}
	return factory(), nil
}

// decodePayload derives the concrete family payload from a.VerifiedEvidence via
// the frozen registry, enforcing SEC-01 bounds and exact-casing on the family
// field set (mirrors the verification path in DecodeAndValidate so that
// Validate and Canonical apply identical structural controls).
func decodePayload(a EvidenceArtifact) (FamilyPayload, error) {
	if len(a.VerifiedEvidence) > MaxArtifactBytes {
		return nil, fmt.Errorf("faultline: %w: verified_evidence exceeds MaxArtifactBytes", ErrMalformed)
	}
	fp, err := lookupFactory(a.SchemaVersion, a.NodeFamily)
	if err != nil {
		return nil, err
	}
	if len(a.VerifiedEvidence) == 0 {
		return nil, fmt.Errorf("%w: verified_evidence missing", ErrMalformed)
	}
	// Exact-casing enforcement: scan the family payload keys and reject any that
	// are absent from the family's declared JSON tag set. This catches miscased
	// keys (e.g. "Scenario_ID" instead of "scenario_id") that encoding/json
	// would otherwise accept via its case-insensitive fallback matching, closing
	// the SEC-01 gap between decodePayload and DecodeAndValidate.
	veKeys, scanErr := preScan(a.VerifiedEvidence)
	if scanErr != nil {
		return nil, scanErr
	}
	famTags := structJSONTags(fp)
	for _, k := range veKeys {
		if _, ok := famTags[k]; !ok {
			return nil, fmt.Errorf("%w: unknown or miscased verified_evidence field %q", ErrMalformed, k)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(a.VerifiedEvidence))
	dec.DisallowUnknownFields()
	if err := dec.Decode(fp); err != nil {
		return nil, fmt.Errorf("faultline: %w: %s", ErrMalformed, err)
	}
	return fp, nil
}

// register adds a family factory keyed by (version, name) under the registry
// lock. It rejects a nil factory, a duplicate key, and post-freeze registration.
func (r *registry) register(version int, name string, factory func() FamilyPayload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if factory == nil {
		return fmt.Errorf("faultline: nil factory for family (%d, %q)", version, name)
	}
	if r.frozen {
		return fmt.Errorf("faultline: registry frozen; cannot register family (%d, %q)", version, name)
	}
	if r.factories[version] == nil {
		r.factories[version] = make(map[string]func() FamilyPayload)
	}
	if _, dup := r.factories[version][name]; dup {
		return fmt.Errorf("faultline: duplicate family (%d, %q)", version, name)
	}
	r.factories[version][name] = factory
	return nil
}

// freezeAgainst freezes r (idempotently) and validates that its registered
// factories match manifest EXACTLY per version. Freezing under the registry
// lock means every subsequent reader observes an immutable snapshot, so
// register-vs-read is race clean.
func (r *registry) freezeAgainst(manifest map[int][]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return nil
	}
	// Every manifest family MUST have a registered factory.
	for version, names := range manifest {
		regv := r.factories[version]
		for _, name := range names {
			if regv == nil || regv[name] == nil {
				return fmt.Errorf("version %d missing registered family %q", version, name)
			}
		}
	}
	// No registered factory may be absent from the manifest.
	for version, regv := range r.factories {
		for name := range regv {
			if !containsStr(manifest[version], name) {
				return fmt.Errorf("version %d has extra family %q not in manifest", version, name)
			}
		}
	}
	r.frozen = true
	return nil
}

// CanonicalMap returns the parity payload as a canonicalization-ready map. The
// SET-like fields (surfaces, divergent_fields) are sorted + duplicate-free so
// the canonical digest is independent of source ordering; empty collections are
// present as an empty array (never null).
func (p *ParityEvidence) CanonicalMap() map[string]any {
	return map[string]any{
		"scenario_id":         p.ScenarioID,
		"dimension":           p.Dimension,
		"surfaces":            sortedAnySet(p.Surfaces),
		"divergent_fields":    sortedAnySet(p.DivergentFields),
		"expected_divergence": p.ExpectedDivergence,
		"tracked_defect":      p.TrackedDefect,
		"detail":              p.Detail,
	}
}

// trackedDefectPattern is the canonical backlog-ID format for TrackedDefect.
var trackedDefectPattern = regexp.MustCompile(`^\d{3,}(\.\d{3,})*-[A-Z]{1,2}$`)

// Validate enforces the parity family invariants against the envelope view: the
// surface set must equal the envelope applicability set and the cross-field
// state matrix (pass/report_only/fail) must hold.
func (p *ParityEvidence) Validate(view EvidenceView) error {
	if p.ScenarioID == "" {
		return fmt.Errorf("%w: scenario_id is required", ErrValidation)
	}
	if p.Dimension == "" {
		return fmt.Errorf("%w: dimension is required", ErrValidation)
	}
	if len(p.Surfaces) == 0 {
		return fmt.Errorf("%w: surfaces is required", ErrValidation)
	}
	for _, s := range p.Surfaces {
		if !validSurface(s) {
			return fmt.Errorf("%w: surface %q not in {cli, mcp, internal}", ErrValidation, s)
		}
	}
	if hasDuplicate(p.Surfaces) {
		return fmt.Errorf("%w: surfaces contains duplicates", ErrValidation)
	}
	if hasDuplicate(p.DivergentFields) {
		return fmt.Errorf("%w: divergent_fields contains duplicates", ErrValidation)
	}
	if !sameStringSet(p.Surfaces, view.Applicability) {
		return fmt.Errorf("%w: surfaces set must equal applicability set", ErrValidation)
	}
	if err := p.validateStateMatrix(view.Status); err != nil {
		return err
	}
	if p.TrackedDefect != "" && !trackedDefectPattern.MatchString(p.TrackedDefect) {
		return fmt.Errorf("%w: tracked_defect %q must match canonical backlog-ID format", ErrValidation, p.TrackedDefect)
	}
	return nil
}

// validateStateMatrix enforces the cross-field Status/divergence invariants.
func (p *ParityEvidence) validateStateMatrix(status string) error {
	if p.ExpectedDivergence && status != StatusReportOnly {
		return fmt.Errorf("%w: expected_divergence requires status=report_only", ErrValidation)
	}
	switch status {
	case StatusPass:
		if len(p.DivergentFields) != 0 || p.ExpectedDivergence {
			return fmt.Errorf("%w: status=pass requires no divergent_fields and expected_divergence=false", ErrValidation)
		}
	case StatusReportOnly:
		if !p.ExpectedDivergence || p.TrackedDefect == "" || len(p.DivergentFields) == 0 {
			return fmt.Errorf("%w: status=report_only requires expected_divergence, tracked_defect and divergent_fields", ErrValidation)
		}
	case StatusFail:
		if len(p.DivergentFields) == 0 || p.ExpectedDivergence {
			return fmt.Errorf("%w: status=fail requires divergent_fields and expected_divergence=false", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: status %q not in {pass, report_only, fail}", ErrValidation, status)
	}
	return nil
}

// validSurface reports whether s is a governed surface identifier.
func validSurface(s string) bool {
	switch s {
	case "cli", "mcp", "internal":
		return true
	default:
		return false
	}
}

// validStatus reports whether s is a governed envelope status.
func validStatus(s string) bool {
	switch s {
	case StatusPass, StatusReportOnly, StatusFail:
		return true
	default:
		return false
	}
}

// containsStr reports whether s is present in xs.
func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// hasDuplicate reports whether xs contains a repeated element.
func hasDuplicate(xs []string) bool {
	seen := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			return true
		}
		seen[x] = struct{}{}
	}
	return false
}

// sameStringSet reports whether a and b contain the same distinct elements.
func sameStringSet(a, b []string) bool {
	sa := make(map[string]struct{}, len(a))
	for _, x := range a {
		sa[x] = struct{}{}
	}
	sb := make(map[string]struct{}, len(b))
	for _, x := range b {
		sb[x] = struct{}{}
	}
	if len(sa) != len(sb) {
		return false
	}
	for x := range sa {
		if _, ok := sb[x]; !ok {
			return false
		}
	}
	return true
}

// sortedAnySet returns a sorted, duplicate-free []any copy of xs. An empty or
// nil input yields a non-nil empty slice so the canonical form is an empty
// array (never null), and the sorted order makes the digest order-independent.
func sortedAnySet(xs []string) []any {
	seen := make(map[string]struct{}, len(xs))
	uniq := make([]string, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		uniq = append(uniq, x)
	}
	sort.Strings(uniq)
	out := make([]any, 0, len(uniq))
	for _, x := range uniq {
		out = append(out, x)
	}
	return out
}

// structJSONTags returns the set of top-level json tag names declared on the
// struct type of v (dereferencing a pointer). Fields tagged "-" are skipped.
func structJSONTags(v any) map[string]struct{} {
	t := reflect.TypeOf(v)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	tags := make(map[string]struct{})
	if t == nil || t.Kind() != reflect.Struct {
		return tags
	}
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if comma := strings.IndexByte(tag, ','); comma >= 0 {
			tag = tag[:comma]
		}
		if tag != "" {
			tags[tag] = struct{}{}
		}
	}
	return tags
}

// preScan walks the entire JSON tree of data enforcing the bounded-decode
// budgets (depth, object members, array elements, string bytes) and rejecting
// duplicate member names (case-sensitive AND case-fold-equal) in every object.
// It returns the root object's keys in source order for envelope exact-casing.
func preScan(data []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	rootKeys, err := scanValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing data", ErrMalformed)
	}
	return rootKeys, nil
}

// scanValue scans one JSON value, returning the object keys when the value is an
// object (nil otherwise).
func scanValue(dec *json.Decoder, depth int) ([]string, error) {
	if depth > MaxJSONDepth {
		return nil, fmt.Errorf("%w: max nesting depth %d exceeded (resource-limit)", ErrMalformed, MaxJSONDepth)
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return scanObject(dec, depth)
		case '[':
			return nil, scanArray(dec, depth)
		default:
			return nil, fmt.Errorf("%w: unexpected token %q", ErrMalformed, t)
		}
	case string:
		if len(t) > MaxStringBytes {
			return nil, fmt.Errorf("%w: string exceeds %d bytes (resource-limit)", ErrMalformed, MaxStringBytes)
		}
	}
	return nil, nil
}

// scanObject scans a JSON object body (after the opening '{'), enforcing the
// member budget and duplicate-key rejection, and returns its keys.
func scanObject(dec *json.Decoder, depth int) ([]string, error) {
	seen := make(map[string]struct{})
	fold := make(map[string]struct{})
	var keys []string
	members := 0
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("%w: non-string object key", ErrMalformed)
		}
		members++
		if members > MaxObjectMembers {
			return nil, fmt.Errorf("%w: object exceeds %d members (resource-limit)", ErrMalformed, MaxObjectMembers)
		}
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("%w: duplicate key %q", ErrMalformed, key)
		}
		lc := strings.ToLower(key)
		if _, dup := fold[lc]; dup {
			return nil, fmt.Errorf("%w: case-variant duplicate key %q", ErrMalformed, key)
		}
		seen[key] = struct{}{}
		fold[lc] = struct{}{}
		keys = append(keys, key)
		if _, err := scanValue(dec, depth+1); err != nil {
			return nil, err
		}
	}
	// Consume the closing '}'.
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	return keys, nil
}

// scanArray scans a JSON array body (after the opening '['), enforcing the
// element budget.
func scanArray(dec *json.Decoder, depth int) error {
	count := 0
	for dec.More() {
		count++
		if count > MaxArrayElements {
			return fmt.Errorf("%w: array exceeds %d elements (resource-limit)", ErrMalformed, MaxArrayElements)
		}
		if _, err := scanValue(dec, depth+1); err != nil {
			return err
		}
	}
	// Consume the closing ']'.
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	return nil
}

// init registers the v1 parity family so the frozen registry matches the
// foundation-owned manifest.
func init() {
	if err := RegisterFamily(EvidenceSchemaVersion, NodeFamilyParity, func() FamilyPayload {
		return &ParityEvidence{}
	}); err != nil {
		panic(fmt.Sprintf("faultline: register parity family: %v", err))
	}
}
