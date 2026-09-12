package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

const (
	shipmentReconcileRequestIdentityDigestDomain = "backlogit/shipment-reconcile/request-identity/v1"
	shipmentReconcileEvidenceDigestDomain        = "backlogit/shipment-reconcile/evidence/v1"
)

var (
	shipmentReconcileMarkdownHeadingRe = regexp.MustCompile(`^(#{1,6})\s*(.*?)\s*$`)
	shipmentReconcilePRCommitRe        = regexp.MustCompile(`(?i)PR\s*#\d+\s*\(([^)]+)\)\s*,\s*commit\s*([0-9a-f]{7,64})`)
)

type shipmentReconcileEvidenceInput struct {
	ManifestDigest       string
	MemberTerminalIDs    []string
	BeforeArchivedStatus string
	Timestamp            time.Time
}

type shipmentReconcileEvidenceResult struct {
	RequestIdentityDigest string
	TrustedRefName        string
	TrustedRefTip         string
	ClosureContentHash    string
	EvidenceDigest        string
	Event                 events.Event
	EventBytes            []byte
	EventDigest           string
}

type shipmentReconcileEvidenceDigestInput struct {
	ShipmentID         string
	MergeSHA           string
	TrustedRefName     string
	TrustedRefTip      string
	ManifestDigest     string
	ClosureContentHash string
	EvidenceRefs       []string
}

type shipmentReconcileDigestField struct {
	tag    string
	values []string
	isList bool
}

type shipmentReconcileNormalizedRequest struct {
	ShipmentID      string
	Reason          string
	Actor           string
	SecondApprover  string
	IdempotencyKey  string
	MergeSHA        string
	ClosureEvidence string
	EvidenceRefs    []string
}

func shipmentReconcileScalarField(tag, value string) shipmentReconcileDigestField {
	return shipmentReconcileDigestField{tag: tag, values: []string{value}}
}

func shipmentReconcileListField(tag string, values []string) shipmentReconcileDigestField {
	return shipmentReconcileDigestField{tag: tag, values: slices.Clone(values), isList: true}
}

// shipmentReconcileDigestHex implements 167.001-T's domain-separated canonical
// digest encoding: a versioned domain tag followed by field-tagged records.
// Each record encodes (field kind, field tag, value count, each value) with a
// uvarint byte-length prefix on every string, so neither adjacent-field boundary
// shifts nor list-item concatenation can collide under a different logical tuple.
func shipmentReconcileDigestHex(domain string, fields ...shipmentReconcileDigestField) string {
	var payload bytes.Buffer
	shipmentReconcileWriteDigestString(&payload, domain)
	for _, field := range fields {
		if field.isList {
			payload.WriteByte(0x02)
		} else {
			payload.WriteByte(0x01)
		}
		shipmentReconcileWriteDigestString(&payload, field.tag)
		shipmentReconcileWriteDigestUvarint(&payload, uint64(len(field.values)))
		for _, value := range field.values {
			shipmentReconcileWriteDigestString(&payload, value)
		}
	}
	sum := sha256.Sum256(payload.Bytes())
	return hex.EncodeToString(sum[:])
}

func shipmentReconcileWriteDigestString(buf *bytes.Buffer, value string) {
	shipmentReconcileWriteDigestUvarint(buf, uint64(len(value)))
	_, _ = buf.WriteString(value)
}

func shipmentReconcileWriteDigestUvarint(buf *bytes.Buffer, value uint64) {
	var scratch [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(scratch[:], value)
	_, _ = buf.Write(scratch[:n])
}

// shipmentReconcileRequestIdentityDigest computes 167.001-T's cheap Phase-A
// replay identity over request scalars only. It validates trimmed non-empty
// core fields but deliberately does NOT read closure/evidence-ref files, so the
// classifier can match a same-key replay even after the original closure path is
// moved or deleted.
func shipmentReconcileRequestIdentityDigest(req ShipmentShippedReconcileRequest) (string, error) {
	normalized, err := normalizeShipmentReconcileRequest(req, false)
	if err != nil {
		return "", err
	}
	return shipmentReconcileRequestIdentityDigestForNormalized(normalized), nil
}

func shipmentReconcileRequestIdentityDigestForNormalized(req shipmentReconcileNormalizedRequest) string {
	return shipmentReconcileDigestHex(
		shipmentReconcileRequestIdentityDigestDomain,
		shipmentReconcileScalarField("idempotency_key", req.IdempotencyKey),
		shipmentReconcileScalarField("merge_sha", req.MergeSHA),
		shipmentReconcileScalarField("reason", req.Reason),
		shipmentReconcileScalarField("actor", req.Actor),
		shipmentReconcileScalarField("second_approver", req.SecondApprover),
		shipmentReconcileListField("evidence_refs", req.EvidenceRefs),
		shipmentReconcileScalarField("closure_path", req.ClosureEvidence),
	)
}

func shipmentReconcileEvidenceDigest(input shipmentReconcileEvidenceDigestInput) string {
	return shipmentReconcileDigestHex(
		shipmentReconcileEvidenceDigestDomain,
		shipmentReconcileScalarField("shipment_id", input.ShipmentID),
		shipmentReconcileScalarField("merge_sha", input.MergeSHA),
		shipmentReconcileScalarField("trusted_ref_name", input.TrustedRefName),
		shipmentReconcileScalarField("trusted_ref_tip", input.TrustedRefTip),
		shipmentReconcileScalarField("manifest_digest", input.ManifestDigest),
		shipmentReconcileScalarField("closure_content_hash", input.ClosureContentHash),
		shipmentReconcileListField("evidence_refs", input.EvidenceRefs),
	)
}

// prepareShipmentReconcileEvidence performs 167.001-T's Phase-C evidence work
// only: git/ref provenance, merge-commit verification, closure-evidence safe
// read + content hash, evidence-ref validation, evidence digest computation, and
// canonical prepared-event materialization. The caller (167.008-T) must supply
// manifestDigest/memberTerminalIDs from the already-locked shipment manifest and
// must never re-run this on a replay/no-op path that should reuse persisted
// digests instead of recomputing the closure content hash.
func prepareShipmentReconcileEvidence(ctx context.Context, ws *Workspace, req ShipmentShippedReconcileRequest, input shipmentReconcileEvidenceInput) (shipmentReconcileEvidenceResult, error) {
	var zero shipmentReconcileEvidenceResult
	if ws == nil {
		return zero, shipmentReconcileEvidenceErrorf("prepare shipment reconcile evidence: workspace is required")
	}
	if strings.TrimSpace(ws.RootPath) == "" {
		return zero, shipmentReconcileEvidenceErrorf("prepare shipment reconcile evidence: workspace root is required")
	}

	normalizedReq, err := normalizeShipmentReconcileRequest(req, true)
	if err != nil {
		return zero, err
	}
	normalizedInput, err := normalizeShipmentReconcileEvidenceInput(input)
	if err != nil {
		return zero, err
	}
	requestIdentityDigest := shipmentReconcileRequestIdentityDigestForNormalized(normalizedReq)

	trustedRefName, trustedRefTip, err := ws.resolveShipmentReconcileTrustedRefTip(ctx, normalizedReq.MergeSHA)
	if err != nil {
		return zero, err
	}
	if err := ws.verifyShipmentReconcileMergeCommit(ctx, normalizedReq.MergeSHA); err != nil {
		return zero, err
	}

	closureBytes, closureRealPath, err := readShipmentReconcileClosureEvidenceFile(ws.RootPath, normalizedReq.ClosureEvidence)
	if err != nil {
		return zero, shipmentReconcileEvidenceWrap(err, "prepare shipment reconcile evidence: read closure evidence %q", normalizedReq.ClosureEvidence)
	}
	if err := verifyShipmentReconcileClosureDeliveryMerge(closureBytes, normalizedReq.ShipmentID, normalizedReq.MergeSHA); err != nil {
		return zero, err
	}

	closureContentHash := shipmentReconcileSHA256Hex(closureBytes)
	closureEvidencePath, err := shipmentReconcileWorkspaceRelativeRealPath(ws.RootPath, closureRealPath)
	if err != nil {
		return zero, shipmentReconcileEvidenceWrap(err, "prepare shipment reconcile evidence: canonicalize closure evidence path")
	}
	persistedEvidenceRefs, err := normalizeShipmentReconcileEvidenceRefs(ws.RootPath, normalizedReq.EvidenceRefs)
	if err != nil {
		return zero, err
	}
	evidenceDigest := shipmentReconcileEvidenceDigest(shipmentReconcileEvidenceDigestInput{
		ShipmentID:         normalizedReq.ShipmentID,
		MergeSHA:           normalizedReq.MergeSHA,
		TrustedRefName:     trustedRefName,
		TrustedRefTip:      trustedRefTip,
		ManifestDigest:     normalizedInput.ManifestDigest,
		ClosureContentHash: closureContentHash,
		EvidenceRefs:       persistedEvidenceRefs,
	})

	delta := ShipmentReconciledShippedDelta{
		Before: ShipmentReconciledShippedBefore{
			Status:         string(models.StatusArchived),
			ArchivedStatus: normalizedInput.BeforeArchivedStatus,
		},
		After:                 ShipmentReconciledShippedAfter{ArchivedStatus: string(ShipmentShipped)},
		Reason:                normalizedReq.Reason,
		Actor:                 normalizedReq.Actor,
		SecondApprover:        normalizedReq.SecondApprover,
		IdempotencyKey:        normalizedReq.IdempotencyKey,
		RequestIdentityDigest: requestIdentityDigest,
		TrustedRefName:        trustedRefName,
		TrustedRefTip:         trustedRefTip,
		Evidence: ShipmentReconciledShippedEvidence{
			MergeSHA:           normalizedReq.MergeSHA,
			ClosureEvidence:    closureEvidencePath,
			ManifestDigest:     normalizedInput.ManifestDigest,
			MemberTerminalIDs:  slices.Clone(normalizedInput.MemberTerminalIDs),
			ClosureContentHash: closureContentHash,
			EvidenceRefs:       slices.Clone(persistedEvidenceRefs),
			EvidenceDigest:     evidenceDigest,
		},
	}
	deltaMap, err := marshalShipmentReconciledShippedDelta(delta)
	if err != nil {
		return zero, shipmentReconcileEvidenceWrap(err, "prepare shipment reconcile evidence: marshal typed delta")
	}
	timestamp := normalizedInput.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	event := events.Event{
		Timestamp: timestamp,
		Actor:     "backlogit",
		ItemID:    normalizedReq.ShipmentID,
		EventType: EventShipmentReconciledShipped,
		Delta:     deltaMap,
		CommitSHA: normalizedReq.MergeSHA,
	}
	eventBytes, err := marshalShipmentReconciledShippedEvent(event)
	if err != nil {
		return zero, shipmentReconcileEvidenceWrap(err, "prepare shipment reconcile evidence: marshal prepared event")
	}
	eventDigest := ShipmentReconciledShippedEventDigest(eventBytes)
	if err := ValidateShipmentReconciledShippedEvent(eventBytes, eventBytes, eventDigest); err != nil {
		return zero, shipmentReconcileEvidenceWrap(err, "prepare shipment reconcile evidence: validate prepared event")
	}

	return shipmentReconcileEvidenceResult{
		RequestIdentityDigest: requestIdentityDigest,
		TrustedRefName:        trustedRefName,
		TrustedRefTip:         trustedRefTip,
		ClosureContentHash:    closureContentHash,
		EvidenceDigest:        evidenceDigest,
		Event:                 event,
		EventBytes:            eventBytes,
		EventDigest:           eventDigest,
	}, nil
}

func normalizeShipmentReconcileRequest(req ShipmentShippedReconcileRequest, requireShipmentID bool) (shipmentReconcileNormalizedRequest, error) {
	var normalized shipmentReconcileNormalizedRequest
	if requireShipmentID {
		shipmentID, err := shipmentReconcileTrimRequiredField("shipment_id", req.ShipmentID)
		if err != nil {
			return normalized, err
		}
		normalized.ShipmentID = shipmentID
	}
	reason, err := shipmentReconcileTrimRequiredField("reason", req.Reason)
	if err != nil {
		return normalized, err
	}
	actor, err := shipmentReconcileTrimRequiredField("actor", req.Actor)
	if err != nil {
		return normalized, err
	}
	idempotencyKey, err := shipmentReconcileTrimRequiredField("idempotency_key", req.IdempotencyKey)
	if err != nil {
		return normalized, err
	}
	mergeSHA, err := shipmentReconcileTrimRequiredField("merge_sha", req.MergeSHA)
	if err != nil {
		return normalized, err
	}
	if !isGitObjectName(mergeSHA) {
		return normalized, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: merge_sha must be a full git object name")
	}
	closureEvidence, err := shipmentReconcileTrimRequiredField("closure_evidence", req.ClosureEvidence)
	if err != nil {
		return normalized, err
	}
	secondApprover := strings.TrimSpace(req.SecondApprover)
	if req.SecondApprover != "" && secondApprover == "" {
		return normalized, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: second_approver must be non-empty when supplied")
	}
	if secondApprover != "" && secondApprover == actor {
		return normalized, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: second_approver must differ from actor")
	}
	evidenceRefs, err := normalizeShipmentReconcileRequestEvidenceRefs(req.EvidenceRefs)
	if err != nil {
		return normalized, err
	}

	normalized.Reason = reason
	normalized.Actor = actor
	normalized.SecondApprover = secondApprover
	normalized.IdempotencyKey = idempotencyKey
	normalized.MergeSHA = strings.ToLower(mergeSHA)
	normalized.ClosureEvidence = closureEvidence
	normalized.EvidenceRefs = evidenceRefs
	return normalized, nil
}

func normalizeShipmentReconcileRequestEvidenceRefs(refs []string) ([]string, error) {
	if len(refs) == 0 {
		return []string{}, nil
	}
	normalized := make([]string, 0, len(refs))
	for i, ref := range refs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			return nil, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: evidence_ref[%d] must be non-empty", i)
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

func normalizeShipmentReconcileEvidenceInput(input shipmentReconcileEvidenceInput) (shipmentReconcileEvidenceInput, error) {
	manifestDigest, err := shipmentReconcileTrimRequiredField("manifest_digest", input.ManifestDigest)
	if err != nil {
		return shipmentReconcileEvidenceInput{}, err
	}
	beforeArchivedStatus, err := shipmentReconcileTrimRequiredField("before_archived_status", input.BeforeArchivedStatus)
	if err != nil {
		return shipmentReconcileEvidenceInput{}, err
	}
	if len(input.MemberTerminalIDs) == 0 {
		return shipmentReconcileEvidenceInput{}, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: member_terminal_ids must be non-empty")
	}
	memberTerminalIDs := make([]string, 0, len(input.MemberTerminalIDs))
	for i, memberID := range input.MemberTerminalIDs {
		trimmed := strings.TrimSpace(memberID)
		if trimmed == "" {
			return shipmentReconcileEvidenceInput{}, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: member_terminal_ids[%d] must be non-empty", i)
		}
		memberTerminalIDs = append(memberTerminalIDs, trimmed)
	}
	input.ManifestDigest = manifestDigest
	input.BeforeArchivedStatus = beforeArchivedStatus
	input.MemberTerminalIDs = memberTerminalIDs
	if !input.Timestamp.IsZero() {
		input.Timestamp = input.Timestamp.UTC()
	}
	return input, nil
}

func shipmentReconcileTrimRequiredField(fieldName, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", shipmentReconcileEvidenceErrorf("shipment reconcile evidence: %s must be non-empty", fieldName)
	}
	return trimmed, nil
}

func shipmentReconcileEvidenceErrorf(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, blerrors.ErrShipmentReconcileEvidence)...)
}

func shipmentReconcileEvidenceWrap(err error, format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, stderrors.Join(blerrors.ErrShipmentReconcileEvidence, err))...)
}

func shipmentReconcileSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func marshalShipmentReconciledShippedDelta(delta ShipmentReconciledShippedDelta) (map[string]any, error) {
	raw, err := json.Marshal(delta)
	if err != nil {
		return nil, fmt.Errorf("marshal delta: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal delta: %w", err)
	}
	return out, nil
}

func marshalShipmentReconciledShippedEvent(event events.Event) ([]byte, error) {
	raw, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func normalizeShipmentReconcileEvidenceRefs(rootPath string, refs []string) ([]string, error) {
	if len(refs) == 0 {
		return []string{}, nil
	}
	normalized := make([]string, 0, len(refs))
	for i, ref := range refs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			return nil, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: evidence_ref[%d] must be non-empty", i)
		}
		absolute, err := shipmentReconcileContainedWorkspacePath(rootPath, trimmed)
		if err != nil {
			return nil, shipmentReconcileEvidenceWrap(err, "shipment reconcile evidence: validate evidence_ref[%d] %q", i, trimmed)
		}
		rel, err := shipmentReconcileWorkspaceRelativeRealPath(rootPath, absolute)
		if err != nil {
			return nil, shipmentReconcileEvidenceWrap(err, "shipment reconcile evidence: canonicalize evidence_ref[%d] %q", i, trimmed)
		}
		normalized = append(normalized, rel)
	}
	return normalized, nil
}

func shipmentReconcileContainedWorkspacePath(rootPath, candidate string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return "", fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root real path: %w", err)
	}
	absolute := candidate
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(rootAbs, candidate)
	}
	absolute = filepath.Clean(absolute)
	realCandidate, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve real path for %s: %w", candidate, err)
	}
	if realCandidate != realRoot && !pathContained(realRoot, realCandidate) {
		return "", fmt.Errorf("path %s resolves outside workspace root %s", candidate, realRoot)
	}
	return realCandidate, nil
}

func shipmentReconcileWorkspaceRelativeRealPath(rootPath, realPath string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return "", fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root real path: %w", err)
	}
	realTarget, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		return "", fmt.Errorf("resolve real path %s: %w", realPath, err)
	}
	if realTarget != realRoot && !pathContained(realRoot, realTarget) {
		return "", fmt.Errorf("path %s resolves outside workspace root %s", realTarget, realRoot)
	}
	return filepath.ToSlash(filepath.Clean(workspaceRelativePath(realRoot, realTarget))), nil
}

func verifyShipmentReconcileClosureDeliveryMerge(closureBytes []byte, shipmentID, mergeSHA string) error {
	mergeSHA = strings.ToLower(strings.TrimSpace(mergeSHA))
	shipmentID = strings.TrimSpace(shipmentID)
	featureSHA, err := shipmentReconcileFeatureMergeFromClosure(closureBytes, shipmentID)
	if err != nil {
		return shipmentReconcileEvidenceWrap(err, "shipment reconcile evidence: parse delivery merge from closure for %s", shipmentID)
	}
	if !shipmentReconcileNarrativeSHAMatches(featureSHA, mergeSHA) {
		return shipmentReconcileEvidenceErrorf("shipment reconcile evidence: closure delivery merge for %s does not match merge_sha", shipmentID)
	}
	return nil
}

func shipmentReconcileFeatureMergeFromClosure(closureBytes []byte, shipmentID string) (string, error) {
	content := string(closureBytes)
	if !shipmentReconcileMentionsShipment(content, shipmentID) {
		return "", fmt.Errorf("closure does not reference shipment %s", shipmentID)
	}
	scope := content
	if section, ok := shipmentReconcileShipmentSection(content, shipmentID); ok {
		scope = section
	}
	matches := shipmentReconcilePRCommitRe.FindAllStringSubmatch(scope, -1)
	featureSHAs := make([]string, 0, 1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(match[1]), "feature") {
			featureSHAs = append(featureSHAs, strings.ToLower(match[2]))
		}
	}
	if len(featureSHAs) == 0 {
		return "", fmt.Errorf("closure section for shipment %s lacks a feature-annotated merge commit", shipmentID)
	}
	if len(featureSHAs) > 1 {
		return "", fmt.Errorf("closure section for shipment %s has multiple feature-annotated merge commits", shipmentID)
	}
	return featureSHAs[0], nil
}

func shipmentReconcileMentionsShipment(content, shipmentID string) bool {
	re := regexp.MustCompile(`(?i)(^|[^A-Za-z0-9])` + regexp.QuoteMeta(shipmentID) + `([^A-Za-z0-9]|$)`)
	return re.FindStringIndex(content) != nil
}

func shipmentReconcileShipmentSection(content, shipmentID string) (string, bool) {
	lines := strings.Split(content, "\n")
	start := -1
	level := 0
	for i, line := range lines {
		match := shipmentReconcileMarkdownHeadingRe.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		title := strings.TrimSpace(match[2])
		if title == shipmentID {
			start = i + 1
			level = len(match[1])
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		match := shipmentReconcileMarkdownHeadingRe.FindStringSubmatch(lines[i])
		if len(match) == 0 {
			continue
		}
		if len(match[1]) <= level {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

func shipmentReconcileNarrativeSHAMatches(narrativeSHA, mergeSHA string) bool {
	narrativeSHA = strings.ToLower(strings.TrimSpace(narrativeSHA))
	mergeSHA = strings.ToLower(strings.TrimSpace(mergeSHA))
	return narrativeSHA != "" && len(narrativeSHA) <= len(mergeSHA) && strings.HasPrefix(mergeSHA, narrativeSHA)
}

func (ws *Workspace) resolveShipmentReconcileTrustedRefTip(ctx context.Context, mergeSHA string) (string, string, error) {
	trustedRefs, err := ws.shipmentReconcileTrustedRefs()
	if err != nil {
		return "", "", err
	}
	var attemptErrs []error
	for _, ref := range trustedRefs {
		tip, tipErr := ws.shipmentReconcileResolveTrustedRefTip(ctx, ref)
		if tipErr != nil {
			attemptErrs = append(attemptErrs, tipErr)
			continue
		}
		ancestor, ancestorErr := ws.isAncestor(ctx, mergeSHA, tip)
		if ancestorErr != nil {
			attemptErrs = append(attemptErrs, fmt.Errorf("check reachability from trusted ref %q (%s): %w", ref, tip, ancestorErr))
			continue
		}
		if ancestor {
			return ref, tip, nil
		}
	}
	if len(attemptErrs) > 0 {
		return "", "", shipmentReconcileEvidenceWrap(stderrors.Join(attemptErrs...), "shipment reconcile evidence: merge_sha is not verifiable from trusted refs %v", trustedRefs)
	}
	return "", "", shipmentReconcileEvidenceErrorf("shipment reconcile evidence: merge_sha is not reachable from trusted refs %v", trustedRefs)
}

func (ws *Workspace) shipmentReconcileTrustedRefs() ([]string, error) {
	cfg := &config.ReconcileConfig{}
	if ws != nil && ws.Config != nil && ws.Config.Reconcile != nil {
		cfg.TrustedRefs = slices.Clone(ws.Config.Reconcile.TrustedRefs)
	}
	if err := cfg.Normalize(ws.RootPath, config.ResolveDefaultBranch); err != nil {
		return nil, shipmentReconcileEvidenceWrap(err, "shipment reconcile evidence: normalize reconcile.trusted_refs")
	}
	trustedRefs := make([]string, 0, len(cfg.TrustedRefs))
	for i, ref := range cfg.TrustedRefs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			return nil, shipmentReconcileEvidenceErrorf("shipment reconcile evidence: trusted_refs[%d] must be non-empty", i)
		}
		trustedRefs = append(trustedRefs, trimmed)
	}
	return trustedRefs, nil
}

func (ws *Workspace) shipmentReconcileResolveTrustedRefTip(ctx context.Context, ref string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, ws.boundedHelperTimeout())
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, "git", "rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	cmd.Dir = ws.RootPath
	cmd.Env = gate.MinimalEnv()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		if ctxErr := runCtx.Err(); ctxErr != nil {
			return "", fmt.Errorf("resolve trusted ref %q aborted: %w", ref, ctxErr)
		}
		var ee *exec.ExitError
		if stderrors.As(runErr, &ee) {
			return "", fmt.Errorf("git rev-parse %q exit %d: %s: %w", ref, ee.ExitCode(), bytes.TrimSpace(stderr.Bytes()), runErr)
		}
		return "", fmt.Errorf("run git rev-parse %q: %w", ref, runErr)
	}
	tip := strings.ToLower(strings.TrimSpace(stdout.String()))
	if !isGitObjectName(tip) {
		return "", fmt.Errorf("trusted ref %q resolved to non-object %q", ref, tip)
	}
	return tip, nil
}

func (ws *Workspace) verifyShipmentReconcileMergeCommit(ctx context.Context, mergeSHA string) error {
	runCtx, cancel := context.WithTimeout(ctx, ws.boundedHelperTimeout())
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, "git", "rev-list", "--parents", "-n", "1", mergeSHA)
	cmd.Dir = ws.RootPath
	cmd.Env = gate.MinimalEnv()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		if ctxErr := runCtx.Err(); ctxErr != nil {
			return shipmentReconcileEvidenceWrap(ctxErr, "shipment reconcile evidence: verify merge commit %s aborted", mergeSHA)
		}
		var ee *exec.ExitError
		if stderrors.As(runErr, &ee) {
			return shipmentReconcileEvidenceWrap(runErr, "shipment reconcile evidence: git rev-list --parents exit %d for %s: %s", ee.ExitCode(), mergeSHA, bytes.TrimSpace(stderr.Bytes()))
		}
		return shipmentReconcileEvidenceWrap(runErr, "shipment reconcile evidence: run git rev-list --parents for %s", mergeSHA)
	}
	fields := strings.Fields(stdout.String())
	if len(fields) < 3 {
		return shipmentReconcileEvidenceErrorf("shipment reconcile evidence: merge_sha %s is not a true merge commit", mergeSHA)
	}
	return nil
}
