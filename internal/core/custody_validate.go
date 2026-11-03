package core

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// ErrCustodyPathRejected reports a custody entry path that is not a safe,
// repository-contained path.
var ErrCustodyPathRejected = errors.New("custody: entry path rejected")

// ErrCustodyOutputTooLarge reports a worktree file or git output above custodyMaxReadBytes.
var ErrCustodyOutputTooLarge = errors.New("custody: read exceeds the size cap")

// custodyMaxReadBytes caps each worktree file read and each git output stream, so a
// hostile or corrupt input cannot exhaust memory.
const custodyMaxReadBytes = 64 << 20

const (
	custodyApprovalTokenVersion = "v1"
	custodyApprovalTokenDomain  = "backlogit/workflow-custody/approval-token/v1"
	custodyStagedDivergesOK     = "staged_diverges_accepted"
	custodyContributorMixed     = "mixed_reconciled"
	custodyProfileOperator      = "operator_verified"
)

// DefaultCustodyReceiptExpiresAt returns the expiry instant of a receipt accepted at acceptedAt.
func DefaultCustodyReceiptExpiresAt(acceptedAt time.Time) time.Time {
	return acceptedAt.Add(DefaultCustodyReceiptExpiry)
}

// DecodeCustodyEnvelopeJSON decodes a workflow-custody/v1 JSON payload. It rejects
// duplicate members, compared case-insensitively, and unknown fields before
// the envelope is used. The result still requires ValidateCustodyEnvelope.
func DecodeCustodyEnvelopeJSON(raw []byte) (WorkflowCustodyEnvelope, error) {
	if err := rejectDuplicateJSONMembers(raw); err != nil {
		return WorkflowCustodyEnvelope{}, fmt.Errorf("decode custody envelope JSON: %w", err)
	}
	var env WorkflowCustodyEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return WorkflowCustodyEnvelope{}, fmt.Errorf("decode custody envelope JSON: %w", err)
	}
	return env, nil
}

// DecodeCustodyEnvelopeYAML decodes a workflow-custody/v1 YAML payload. It rejects
// duplicate mapping keys at every depth and unknown fields before the envelope
// is used. The result still requires ValidateCustodyEnvelope.
func DecodeCustodyEnvelopeYAML(raw []byte) (WorkflowCustodyEnvelope, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return WorkflowCustodyEnvelope{}, fmt.Errorf("decode custody envelope YAML: %w", err)
	}
	if err := rejectDuplicateYAMLKeys(&doc); err != nil {
		return WorkflowCustodyEnvelope{}, fmt.Errorf("decode custody envelope YAML: %w: %w", blerrors.ErrValidation, err)
	}
	var env WorkflowCustodyEnvelope
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&env); err != nil {
		return WorkflowCustodyEnvelope{}, fmt.Errorf("decode custody envelope YAML: %w", err)
	}
	return env, nil
}

// ValidateCustodyEnvelope validates a workflow-custody/v1 envelope without touching
// the filesystem. It returns ErrUnsupportedCustodyChangeKind for unmodeled change
// kinds, ErrPartialRenameAdoptionRejected for incomplete rename bindings,
// ErrMixedIndexWorktreeDivergence for unreconciled staged and unstaged mixes,
// ErrCustodyDefaultBranchForbidden for a default-branch scope, and
// ErrOperatorApprovalRequired for operator-verified receipts that lack a proof.
func ValidateCustodyEnvelope(env WorkflowCustodyEnvelope) error {
	if env.SchemaVersion != WorkflowCustodySchemaVersion {
		return fmt.Errorf("%w: schema_version must be %q", blerrors.ErrValidation, WorkflowCustodySchemaVersion)
	}
	required := []struct{ name, value string }{
		{"workflow_id", env.WorkflowID},
		{"shipment_id", env.ShipmentID},
		{"scope_branch", env.ScopeBranch},
		{"base_commit_sha", env.BaseCommitSHA},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%w: %s is required", blerrors.ErrValidation, field.name)
		}
	}
	if strings.EqualFold(env.ScopeBranch, "main") || strings.EqualFold(env.ScopeBranch, "master") {
		return fmt.Errorf("%w: %q", ErrCustodyDefaultBranchForbidden, env.ScopeBranch)
	}
	if env.CustodyRevision < 0 {
		return fmt.Errorf("%w: custody_revision must not be negative", blerrors.ErrValidation)
	}
	if env.ActiveReceiptID != "" && !custodyHasReceipt(env.Receipts, env.ActiveReceiptID) {
		return fmt.Errorf("%w: active_receipt_id %q names no receipt", blerrors.ErrValidation, env.ActiveReceiptID)
	}
	for i := range env.Receipts {
		if err := validateCustodyReceipt(&env.Receipts[i]); err != nil {
			return fmt.Errorf("receipt %q: %w", env.Receipts[i].ReceiptID, err)
		}
	}
	return validateCustodyAcceptedRevisions(env)
}

// validateCustodyAcceptedRevisions refuses two accepted receipts at one revision, and an
// accepted receipt at or below CustodyRevision that is not the active receipt. Either
// shape means an older offer has been accepted over a newer one.
func validateCustodyAcceptedRevisions(env WorkflowCustodyEnvelope) error {
	seen := make(map[int]string, len(env.Receipts))
	for i := range env.Receipts {
		r := env.Receipts[i]
		if r.Status != custodyReceiptAccepted {
			continue
		}
		if other, dup := seen[r.Revision]; dup {
			return fmt.Errorf("%w: %w: accepted receipts %q and %q share revision %d", blerrors.ErrValidation, ErrCustodyStaleRevision, other, r.ReceiptID, r.Revision)
		}
		seen[r.Revision] = r.ReceiptID
		if r.ReceiptID != env.ActiveReceiptID && r.Revision <= env.CustodyRevision {
			return fmt.Errorf("%w: %w: accepted receipt %q at revision %d is not active and does not exceed custody revision %d", blerrors.ErrValidation, ErrCustodyStaleRevision, r.ReceiptID, r.Revision, env.CustodyRevision)
		}
	}
	return nil
}

func custodyHasReceipt(receipts []CustodyReceipt, id string) bool {
	for i := range receipts {
		if receipts[i].ReceiptID == id {
			return true
		}
	}
	return false
}

func validateCustodyReceipt(r *CustodyReceipt) error {
	if strings.TrimSpace(r.ReceiptID) == "" {
		return fmt.Errorf("%w: receipt_id is required", blerrors.ErrValidation)
	}
	if r.Revision < 1 {
		return fmt.Errorf("%w: revision must be at least 1", blerrors.ErrValidation)
	}
	if !custodyKindKnown(r.Kind) || !custodyProfileKnown(r.AuthorityProfile) || !custodyStatusKnown(r.Status) {
		return fmt.Errorf("%w: unknown kind, authority_profile, or status", blerrors.ErrValidation)
	}
	if !custodyProducerRoleKnown(r.ProducerRole) || !custodyAcceptingRoleKnown(r.AcceptingRole) || !custodyActionKnown(r.AllowedAction) {
		return fmt.Errorf("%w: unknown producer_role, accepting_role, or allowed_action", blerrors.ErrValidation)
	}
	if strings.TrimSpace(r.CandidateSetDigest) == "" {
		return fmt.Errorf("%w: candidate_set_digest is required", blerrors.ErrValidation)
	}
	if r.Kind == "preexisting_adoption" && r.AuthorityProfile != custodyProfileOperator {
		return fmt.Errorf("%w: preexisting_adoption requires %s", ErrOperatorApprovalRequired, custodyProfileOperator)
	}
	if r.AuthorityProfile == custodyProfileOperator && r.Status == "accepted" && r.ApprovalProof == nil {
		return fmt.Errorf("%w: accepted %s receipt has no approval_proof", ErrOperatorApprovalRequired, custodyProfileOperator)
	}
	if len(r.Entries) == 0 {
		return fmt.Errorf("%w: receipt has no entries", blerrors.ErrValidation)
	}
	return validateCustodyEntries(r.Entries, r.AuthorityProfile)
}

func validateCustodyEntries(entries []CustodyEntry, profile string) error {
	seen := make(map[string]struct{}, len(entries))
	groups := make(map[string][]string)
	for i := range entries {
		e := &entries[i]
		fold := custodyFoldKey(e.Path)
		if _, dup := seen[fold]; dup {
			return fmt.Errorf("%w: duplicate entry path %q (case-folded)", blerrors.ErrValidation, e.Path)
		}
		seen[fold] = struct{}{}
		if err := validateCustodyEntry(e, profile); err != nil {
			return err
		}
		if e.RenameGroupID != "" {
			groups[e.RenameGroupID] = append(groups[e.RenameGroupID], e.ChangeKind)
		}
	}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !isPairedRename(groups[id]) {
			return fmt.Errorf("%w: rename_group_id %q must link exactly one deleted and one added entry", ErrPartialRenameAdoptionRejected, id)
		}
	}
	return nil
}

func isPairedRename(kinds []string) bool {
	if len(kinds) != 2 {
		return false
	}
	return (kinds[0] == "deleted" && kinds[1] == "added") || (kinds[0] == "added" && kinds[1] == "deleted")
}

func validateCustodyEntry(e *CustodyEntry, profile string) error {
	if custodyUnmodeledChangeKind(e.ChangeKind) {
		return fmt.Errorf("%w: git status %q is not modeled and fails closed", ErrUnsupportedCustodyChangeKind, e.ChangeKind)
	}
	if !custodyChangeKindKnown(e.ChangeKind) {
		return fmt.Errorf("%w: change_kind %q", ErrUnsupportedCustodyChangeKind, e.ChangeKind)
	}
	if _, err := validateCustodyRelPath(e.Path); err != nil {
		return err
	}
	if !custodyStagedStateKnown(e.StagedState) || !custodyContributorKnown(e.Contributor) || !custodySourceKindKnown(e.SourceKind) {
		return fmt.Errorf("%w: unknown staged_state, contributor, or source_kind for %q", blerrors.ErrValidation, e.Path)
	}
	if e.ChangeKind == "renamed" {
		if err := validateCustodyRename(e); err != nil {
			return err
		}
	} else if e.OldPath != "" || e.CaseOnlyRename {
		return fmt.Errorf("%w: old_path and case_only_rename apply only to renamed entries", blerrors.ErrValidation)
	}
	if e.RenameGroupID != "" && e.ChangeKind != "added" && e.ChangeKind != "deleted" {
		return fmt.Errorf("%w: rename_group_id applies only to added and deleted entries", blerrors.ErrValidation)
	}
	if e.IndexSHA256 != e.BaseBlobSHA256 && e.WorktreeSHA256 != e.IndexSHA256 {
		if e.StagedState != custodyStagedDivergesOK || e.Contributor != custodyContributorMixed || profile != custodyProfileOperator {
			return fmt.Errorf("%w: %q has index and worktree content that diverge from base", ErrMixedIndexWorktreeDivergence, e.Path)
		}
	}
	return nil
}

func validateCustodyRename(e *CustodyEntry) error {
	if e.OldPath == "" {
		return fmt.Errorf("%w: renamed entry %q requires old_path", ErrPartialRenameAdoptionRejected, e.Path)
	}
	if e.OldBaseBlobSHA256 == "" {
		return fmt.Errorf("%w: renamed entry %q requires old_base_blob_sha256", ErrPartialRenameAdoptionRejected, e.Path)
	}
	if _, err := validateCustodyRelPath(e.OldPath); err != nil {
		return err
	}
	if e.OldPath == e.Path {
		return fmt.Errorf("%w: renamed entry %q has identical old_path and path", blerrors.ErrValidation, e.Path)
	}
	caseOnly := strings.EqualFold(e.OldPath, e.Path) && e.OldPath != e.Path
	if e.CaseOnlyRename != caseOnly {
		return fmt.Errorf("%w: case_only_rename does not match old_path %q and path %q", blerrors.ErrValidation, e.OldPath, e.Path)
	}
	return nil
}

// custodyFoldKey folds each rune to the minimum of its unicode.SimpleFold orbit,
// the same equivalence the JSON member scanner applies to object keys.
func custodyFoldKey(s string) string {
	var b strings.Builder
	for _, r := range s {
		lowest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < lowest {
				lowest = f
			}
		}
		b.WriteRune(lowest)
	}
	return b.String()
}

// validateCustodyRelPath performs the purely lexical path checks and returns the
// normalized POSIX path.
func validateCustodyRelPath(p string) (string, error) {
	switch {
	case p == "":
		return "", fmt.Errorf("%w: path is empty", ErrCustodyPathRejected)
	case strings.ContainsAny(p, "\\\x00"):
		return "", fmt.Errorf("%w: %q contains a backslash or NUL", ErrCustodyPathRejected, p)
	case strings.HasPrefix(p, "/"):
		return "", fmt.Errorf("%w: %q is absolute", ErrCustodyPathRejected, p)
	case len(p) >= 2 && p[1] == ':':
		return "", fmt.Errorf("%w: %q has a drive letter", ErrCustodyPathRejected, p)
	}
	for _, segment := range strings.Split(p, "/") {
		switch {
		case segment == "":
			return "", fmt.Errorf("%w: %q has an empty segment", ErrCustodyPathRejected, p)
		case segment == "." || segment == "..":
			return "", fmt.Errorf("%w: %q has a %q segment", ErrCustodyPathRejected, p, segment)
		case strings.EqualFold(segment, ".git"):
			return "", fmt.Errorf("%w: %q is inside .git", ErrCustodyPathRejected, p)
		case strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " "):
			return "", fmt.Errorf("%w: %q has a segment with a trailing dot or space", ErrCustodyPathRejected, p)
		case strings.ContainsRune(segment, ':'):
			return "", fmt.Errorf("%w: %q has an alternate data stream or pathspec marker", ErrCustodyPathRejected, p)
		case custodyShortNameSegment(segment):
			// An 8.3 alias such as GIT~1 can name .git, so it is refused; an ordinary tilde is not.
			return "", fmt.Errorf("%w: %q has an 8.3 short-name segment", ErrCustodyPathRejected, p)
		case custodyReservedDeviceName(segment):
			return "", fmt.Errorf("%w: %q has a reserved device name", ErrCustodyPathRejected, p)
		}
	}
	return p, nil
}

// custodyShortNameSegment reports whether segment ends its stem in an 8.3 alias marker: a
// tilde followed by digits, then the end of the segment or an extension dot (GIT~1, PROGRA~1.TXT).
func custodyShortNameSegment(segment string) bool {
	for i := 0; i < len(segment); i++ {
		if segment[i] != '~' {
			continue
		}
		j := i + 1
		for j < len(segment) && segment[j] >= '0' && segment[j] <= '9' {
			j++
		}
		if j > i+1 && (j == len(segment) || segment[j] == '.') {
			return true
		}
	}
	return false
}

// custodyReservedDeviceName reports whether segment, ignoring any extension, is a DOS device
// name that Windows reserves in every directory.
func custodyReservedDeviceName(segment string) bool {
	stem := segment
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	stem = strings.ToUpper(stem)
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	return len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9'
}

// ValidateCustodyEntryPath validates that entryPath is a safe, repository-relative
// POSIX path under ws. It normalizes the workspace root, rejects lexically unsafe
// paths, and walks each existing component with Lstat, rejecting any symlink or
// reparse point. Each component must also resolve to its own lexical path. A component
// that does not exist ends the walk, because nothing below it can be a link.
//
// This is a point-in-time check. A local writer that replaces a component with a link
// after its check and before the open is not detected here. The final-component
// no-follow read in readCustodyWorktreeFileNoFollow narrows that window, but does not
// close it, for the final component only.
func ValidateCustodyEntryPath(ws *Workspace, entryPath string) error {
	rel, err := validateCustodyRelPath(entryPath)
	if err != nil {
		return err
	}
	if ws == nil || ws.RootPath == "" {
		return fmt.Errorf("%w: workspace root is not set", ErrCustodyPathRejected)
	}
	absRoot, err := filepath.Abs(ws.RootPath)
	if err != nil {
		return fmt.Errorf("%w: resolve workspace root: %w", ErrCustodyPathRejected, err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return fmt.Errorf("%w: resolve workspace root: %w", ErrCustodyPathRejected, err)
	}
	current := realRoot
	for _, segment := range strings.Split(rel, "/") {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: lstat %s: %w", ErrCustodyPathRejected, rel, err)
		}
		linked, err := IsSymlinkOrReparsePoint(info, current)
		if err != nil {
			return fmt.Errorf("%w: inspect %s: %w", ErrCustodyPathRejected, rel, err)
		}
		if linked {
			return fmt.Errorf("%w: %s traverses a symlink or reparse point", ErrCustodyPathRejected, rel)
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return fmt.Errorf("%w: resolve %s: %w", ErrCustodyPathRejected, rel, err)
		}
		if !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(current)) {
			return fmt.Errorf("%w: %s resolves outside its lexical path", ErrCustodyPathRejected, rel)
		}
	}
	return nil
}

// ValidateCustodyRenameCasing verifies every case_only_rename entry in env against
// the worktree under ws. A case-only rename is a renamed entry whose old_path and
// path are equal under strings.EqualFold but not byte-equal. For each one, the
// path must pass ValidateCustodyEntryPath, and its parent directory must hold a
// directory entry whose name exactly matches the path's casing. A missing or
// differently cased entry fails closed with ErrPartialRenameAdoptionRejected.
func ValidateCustodyRenameCasing(ws *Workspace, env WorkflowCustodyEnvelope) error {
	for i := range env.Receipts {
		for j := range env.Receipts[i].Entries {
			e := &env.Receipts[i].Entries[j]
			if e.ChangeKind != "renamed" || e.OldPath == e.Path || !strings.EqualFold(e.OldPath, e.Path) {
				continue
			}
			if err := validateCaseOnlyRenameOnDisk(ws, e.Path); err != nil {
				return fmt.Errorf("receipt %q: %w", env.Receipts[i].ReceiptID, err)
			}
		}
	}
	return nil
}

// validateCaseOnlyRenameOnDisk checks one case-only rename target against the
// directory entries of its parent under ws.
func validateCaseOnlyRenameOnDisk(ws *Workspace, entryPath string) error {
	if err := ValidateCustodyEntryPath(ws, entryPath); err != nil {
		return err
	}
	realRoot, err := custodyRealRoot(ws)
	if err != nil {
		return err
	}
	dir, base := filepath.Split(filepath.FromSlash(entryPath))
	entries, err := os.ReadDir(filepath.Join(realRoot, dir))
	if err != nil {
		return fmt.Errorf("%w: read directory of %s: %w", ErrPartialRenameAdoptionRejected, entryPath, err)
	}
	for _, entry := range entries {
		if entry.Name() == base {
			return nil
		}
	}
	return fmt.Errorf("%w: no on-disk entry with exact casing %q for case_only_rename path %s", ErrPartialRenameAdoptionRejected, base, entryPath)
}

func custodyRealRoot(ws *Workspace) (string, error) {
	if ws == nil || ws.RootPath == "" {
		return "", fmt.Errorf("%w: workspace root is not set", ErrCustodyPathRejected)
	}
	absRoot, err := filepath.Abs(ws.RootPath)
	if err != nil {
		return "", fmt.Errorf("%w: resolve workspace root: %w", ErrCustodyPathRejected, err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("%w: resolve workspace root: %w", ErrCustodyPathRejected, err)
	}
	return realRoot, nil
}

// readCustodyWorktreeFileNoFollow reads the regular file at entryPath under ws. It
// validates the path, Lstat-checks the target, opens it through the no-follow
// helper, and confirms the opened file is the same file as the Lstat pre-check.
// A symlink or junction swapped in after validation therefore fails closed.
func readCustodyWorktreeFileNoFollow(ws *Workspace, entryPath string) ([]byte, error) {
	if err := ValidateCustodyEntryPath(ws, entryPath); err != nil {
		return nil, err
	}
	realRoot, err := custodyRealRoot(ws)
	if err != nil {
		return nil, err
	}
	target := filepath.Join(realRoot, filepath.FromSlash(entryPath))
	pre, err := os.Lstat(target)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", entryPath, err)
	}
	if !pre.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrCustodyPathRejected, entryPath)
	}
	file, err := openCustodyNoFollow(target)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCustodyPathRejected, err)
	}
	defer func() { _ = file.Close() }()
	post, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened %s: %w", entryPath, err)
	}
	if !os.SameFile(pre, post) || !post.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s changed between validation and open", ErrCustodyPathRejected, entryPath)
	}
	data, err := custodyReadBounded(file, custodyMaxReadBytes)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", entryPath, err)
	}
	return data, nil
}

// custodyReadBounded reads r to EOF and fails closed with ErrCustodyOutputTooLarge
// when more than limit bytes are available.
func custodyReadBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrCustodyOutputTooLarge, limit)
	}
	return data, nil
}

// custodyRequestIdentity is the canonical request-identity projection of one receipt. It omits
// lifecycle fields such as status, timestamps, and approval proofs so a token minted before
// acceptance still binds the request after acceptance. It deliberately carries only the receipt
// fields the approval covers, so a later receipt in the same envelope does not change it.
type custodyRequestIdentity struct {
	WorkflowID    string                        `json:"workflow_id"`
	ShipmentID    string                        `json:"shipment_id"`
	ScopeBranch   string                        `json:"scope_branch"`
	BaseCommitSHA string                        `json:"base_commit_sha"`
	Receipt       custodyRequestIdentityReceipt `json:"receipt"`
}

type custodyRequestIdentityReceipt struct {
	ReceiptID          string         `json:"receipt_id"`
	CandidateSetDigest string         `json:"candidate_set_digest"`
	AllowedAction      string         `json:"allowed_action"`
	Revision           int            `json:"revision"`
	AcceptingRole      string         `json:"accepting_role"`
	Entries            []CustodyEntry `json:"entries"`
}

// CustodyRequestIdentityDigest returns the domain-separated request identity digest of the
// latest receipt of env: the receipt with the highest revision, ties broken by the lowest
// receipt_id. For a single-receipt envelope that is the digest of its only receipt, so proofs
// minted over a single-receipt envelope still verify. Profile A proofs for a specific receipt
// are checked against custodyReceiptIdentityDigest instead.
func CustodyRequestIdentityDigest(env WorkflowCustodyEnvelope) (string, error) {
	return custodyReceiptIdentityDigest(CustodyRequestIdentityDomain, env, latestCustodyReceipt(env))
}

// custodyReceiptIdentityDigest returns the digest of the request identity of receipt within
// env, domain-separated by domain. Changes to other receipts of env do not change it.
func custodyReceiptIdentityDigest(domain string, env WorkflowCustodyEnvelope, receipt CustodyReceipt) (string, error) {
	identity := custodyRequestIdentity{
		WorkflowID:    env.WorkflowID,
		ShipmentID:    env.ShipmentID,
		ScopeBranch:   env.ScopeBranch,
		BaseCommitSHA: env.BaseCommitSHA,
		Receipt: custodyRequestIdentityReceipt{
			ReceiptID:          receipt.ReceiptID,
			CandidateSetDigest: receipt.CandidateSetDigest,
			AllowedAction:      receipt.AllowedAction,
			Revision:           receipt.Revision,
			AcceptingRole:      receipt.AcceptingRole,
			Entries:            receipt.Entries,
		},
	}
	payload, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("encode custody request identity: %w", err)
	}
	return canonical.HashBytes(append([]byte(domain+"\n"), payload...)), nil
}

// latestCustodyReceipt returns the receipt with the highest revision, ties broken by the lowest
// receipt_id. An envelope with no receipts yields the zero receipt.
func latestCustodyReceipt(env WorkflowCustodyEnvelope) CustodyReceipt {
	var latest CustodyReceipt
	found := false
	for _, r := range env.Receipts {
		if !found || r.Revision > latest.Revision || (r.Revision == latest.Revision && r.ReceiptID < latest.ReceiptID) {
			latest, found = r, true
		}
	}
	return latest
}

// custodyApprovalReceipt returns the receipt an approval proof names. A proof that names no
// receipt is bound to the latest receipt, and a named receipt must exist in env.
func custodyApprovalReceipt(env WorkflowCustodyEnvelope, receiptID string) (CustodyReceipt, bool) {
	if receiptID == "" {
		return latestCustodyReceipt(env), len(env.Receipts) > 0
	}
	if idx := custodyReceiptIndex(env.Receipts, receiptID); idx >= 0 {
		return env.Receipts[idx], true
	}
	return CustodyReceipt{}, false
}

// MintCustodyApprovalToken always fails closed with ErrApprovalChannelUnavailable
// because no operator signing key is injected. Host supervisors that hold the
// operator key must call MintCustodyApprovalTokenWithSigner.
func MintCustodyApprovalToken(proof CustodyApprovalProof) (string, error) {
	if err := validateCustodyApprovalFields(proof); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%w: no operator signing key is injected", ErrApprovalChannelUnavailable)
}

// MintCustodyApprovalTokenWithSigner signs proof with the injected Ed25519 operator
// key and returns the encoded token "v1.<payload>.<signature>". The payload is the
// JSON encoding of proof with Signature cleared, and the signature covers the
// domain-separated payload bytes.
func MintCustodyApprovalTokenWithSigner(proof CustodyApprovalProof, signer ed25519.PrivateKey) (string, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("%w: operator signing key has invalid size", ErrApprovalChannelUnavailable)
	}
	if err := validateCustodyApprovalFields(proof); err != nil {
		return "", err
	}
	if proof.NotAfterUTC.Sub(proof.NotBeforeUTC) > MaxCustodyApprovalTokenWindow {
		return "", fmt.Errorf("%w: approval window exceeds %s", blerrors.ErrValidation, MaxCustodyApprovalTokenWindow)
	}
	proof.Signature = ""
	payload, err := json.Marshal(proof)
	if err != nil {
		return "", fmt.Errorf("encode approval proof: %w", err)
	}
	sig := ed25519.Sign(signer, custodyTokenSigningInput(payload))
	return custodyApprovalTokenVersion + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func custodyTokenSigningInput(payload []byte) []byte {
	return append([]byte(custodyApprovalTokenDomain+"\n"), payload...)
}

func validateCustodyApprovalFields(proof CustodyApprovalProof) error {
	required := []struct{ name, value string }{
		{"key_id", proof.KeyID},
		{"principal", proof.Principal},
		{"authorized_role", proof.AuthorizedRole},
		{"workflow_id", proof.WorkflowID},
		{"shipment_id", proof.ShipmentID},
		{"scope_branch", proof.ScopeBranch},
		{"base_commit_sha", proof.BaseCommitSHA},
		{"candidate_set_digest", proof.CandidateSetDigest},
		{"nonce", proof.Nonce},
		{"request_identity_digest", proof.RequestIdentityDigest},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%w: approval proof %s is required", blerrors.ErrValidation, field.name)
		}
	}
	if proof.Algorithm != config.TrustAnchorAlgoEd25519 {
		return fmt.Errorf("%w: approval proof algorithm must be %s", blerrors.ErrValidation, config.TrustAnchorAlgoEd25519)
	}
	if !proof.NotAfterUTC.After(proof.NotBeforeUTC) {
		return fmt.Errorf("%w: approval proof window is empty or inverted", blerrors.ErrValidation)
	}
	return nil
}

// VerifyCustodyApprovalToken authenticates token against the trust anchors and then
// checks its claims against env at time now. It returns the proof only when every
// check passes, and fails closed otherwise. Authentication runs before any claim is
// trusted. A signature that does not verify against the matching operator anchor is
// rejected, and so is a non-operator role, a window longer than
// MaxCustodyApprovalTokenWindow, a time outside the window, or any binding mismatch.
// Anchor file references are resolved without a workspace root, so only inline PEM
// anchors verify here, and file-backed anchors fail closed.
func VerifyCustodyApprovalToken(token string, env WorkflowCustodyEnvelope, now time.Time, anchors ...config.TrustAnchor) (CustodyApprovalProof, error) {
	payload, signature, err := splitCustodyApprovalToken(token)
	if err != nil {
		return CustodyApprovalProof{}, err
	}
	proof, err := decodeCustodyApprovalPayload(payload)
	if err != nil {
		return CustodyApprovalProof{}, err
	}
	pub, err := resolveCustodyOperatorAnchor(proof.KeyID, now, anchors)
	if err != nil {
		return CustodyApprovalProof{}, err
	}
	if !ed25519.Verify(pub, custodyTokenSigningInput(payload), signature) {
		return CustodyApprovalProof{}, fmt.Errorf("%w: approval signature does not verify", ErrAuthorizationRejected)
	}
	if proof.AuthorizedRole != "operator" {
		return CustodyApprovalProof{}, fmt.Errorf("%w: authorized_role %q is not operator", ErrOperatorApprovalRequired, proof.AuthorizedRole)
	}
	if err := validateCustodyApprovalFields(proof); err != nil {
		return CustodyApprovalProof{}, fmt.Errorf("%w: %w", ErrAuthorizationRequired, err)
	}
	if !proof.NotAfterUTC.After(proof.NotBeforeUTC) || proof.NotAfterUTC.Sub(proof.NotBeforeUTC) > MaxCustodyApprovalTokenWindow {
		return CustodyApprovalProof{}, fmt.Errorf("%w: approval window exceeds %s or is inverted", ErrAuthorizationRejected, MaxCustodyApprovalTokenWindow)
	}
	if now.Before(proof.NotBeforeUTC) || now.After(proof.NotAfterUTC) {
		return CustodyApprovalProof{}, fmt.Errorf("%w: approval is outside its validity window", ErrAuthorizationRejected)
	}
	if err := verifyCustodyApprovalBinding(proof, env); err != nil {
		return CustodyApprovalProof{}, err
	}
	proof.Signature = base64.RawURLEncoding.EncodeToString(signature)
	return proof, nil
}

func splitCustodyApprovalToken(token string) ([]byte, []byte, error) {
	if token == "" {
		return nil, nil, fmt.Errorf("%w: approval token is empty", ErrAuthorizationRequired)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != custodyApprovalTokenVersion {
		return nil, nil, fmt.Errorf("%w: approval token is malformed", ErrAuthorizationRequired)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("%w: approval payload is not base64url: %w", ErrAuthorizationRequired, err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, nil, fmt.Errorf("%w: approval signature is malformed", ErrAuthorizationRequired)
	}
	return payload, signature, nil
}

func decodeCustodyApprovalPayload(payload []byte) (CustodyApprovalProof, error) {
	if err := rejectDuplicateJSONMembers(payload); err != nil {
		return CustodyApprovalProof{}, fmt.Errorf("%w: %w", ErrAuthorizationRequired, err)
	}
	var proof CustodyApprovalProof
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&proof); err != nil {
		return CustodyApprovalProof{}, fmt.Errorf("%w: decode approval proof: %w", ErrAuthorizationRequired, err)
	}
	return proof, nil
}

// resolveCustodyOperatorAnchor selects the single trust anchor named by keyID and
// returns its verified Ed25519 public key. It requires an active, in-window,
// operator-role anchor.
func resolveCustodyOperatorAnchor(keyID string, now time.Time, anchors []config.TrustAnchor) (ed25519.PublicKey, error) {
	var match *config.TrustAnchor
	for i := range anchors {
		if anchors[i].ID != keyID {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("%w: trust anchor id %q is not unique", ErrAuthorizationRejected, keyID)
		}
		match = &anchors[i]
	}
	if match == nil {
		return nil, fmt.Errorf("%w: no trust anchor for key_id %q", ErrApprovalChannelUnavailable, keyID)
	}
	if match.Algo != config.TrustAnchorAlgoEd25519 {
		return nil, fmt.Errorf("%w: trust anchor %q algo is not ed25519", ErrAuthorizationRejected, keyID)
	}
	if match.Role != "operator" {
		return nil, fmt.Errorf("%w: trust anchor %q role is not operator", ErrOperatorApprovalRequired, keyID)
	}
	switch match.Status {
	case config.TrustAnchorStatusActive:
	case config.TrustAnchorStatusRevoked:
		return nil, fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrTrustAnchorRevoked)
	default:
		return nil, fmt.Errorf("%w: trust anchor %q has unknown status", ErrAuthorizationRejected, keyID)
	}
	if match.NotBefore != nil && now.Before(*match.NotBefore) {
		return nil, fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrTrustAnchorNotYetValid)
	}
	if match.NotAfter != nil && now.After(*match.NotAfter) {
		return nil, fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrTrustAnchorExpired)
	}
	return custodyAnchorPublicKey(*match)
}

// custodyAnchorPublicKey loads the verified Ed25519 public key of an operator trust anchor.
func custodyAnchorPublicKey(anchor config.TrustAnchor) (ed25519.PublicKey, error) {
	key, err := LoadVerificationKey("", anchor)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrApprovalChannelUnavailable, err)
	}
	pub, ok := key.Public.(ed25519.PublicKey)
	if !ok || len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: trust anchor %q is not an Ed25519 key", ErrApprovalChannelUnavailable, anchor.ID)
	}
	return pub, nil
}

// custodyWorkspaceOperatorAnchors returns the workspace-config anchors considered when
// verifying operator approvals at now. Revoked and expired anchors stay in the set, so
// resolution reports their typed refusal rather than an unavailable channel. Each anchor must
// carry role operator and the ed25519 algorithm, and must not start before not_before. A
// key_id that appears more than once is dropped entirely.
func custodyWorkspaceOperatorAnchors(anchors []config.TrustAnchor, now time.Time) []config.TrustAnchor {
	counts := make(map[string]int, len(anchors))
	for i := range anchors {
		counts[anchors[i].ID]++
	}
	usable := make([]config.TrustAnchor, 0, len(anchors))
	for i := range anchors {
		a := anchors[i]
		if counts[a.ID] != 1 || a.Role != "operator" || a.Algo != config.TrustAnchorAlgoEd25519 {
			continue
		}
		if a.Status != config.TrustAnchorStatusActive && a.Status != config.TrustAnchorStatusRevoked {
			continue
		}
		if a.NotBefore != nil && now.Before(*a.NotBefore) {
			continue
		}
		usable = append(usable, a)
	}
	return usable
}

func verifyCustodyApprovalBinding(proof CustodyApprovalProof, env WorkflowCustodyEnvelope) error {
	if env.WorkflowID != proof.WorkflowID {
		return fmt.Errorf("%w: approval workflow_id does not match the envelope", ErrAuthorizationRejected)
	}
	if env.ShipmentID != proof.ShipmentID {
		return fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrApprovalWrongShipment)
	}
	if env.ScopeBranch != proof.ScopeBranch {
		return fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrCustodyWrongBranch)
	}
	if env.BaseCommitSHA != proof.BaseCommitSHA {
		return fmt.Errorf("%w: approval base_commit_sha does not match the envelope", ErrAuthorizationRejected)
	}
	if !custodyHasCandidateSet(env.Receipts, proof.CandidateSetDigest) {
		return fmt.Errorf("%w: approval candidate_set_digest matches no receipt", ErrCustodyStaleDigest)
	}
	receipt, ok := custodyApprovalReceipt(env, proof.ReceiptID)
	if !ok {
		return fmt.Errorf("%w: approval receipt %q is not in the envelope", ErrAuthorizationRejected, proof.ReceiptID)
	}
	if err := verifyCustodyApprovalClaims(proof, receipt); err != nil {
		return err
	}
	digest, err := custodyReceiptIdentityDigest(CustodyRequestIdentityDomain, env, receipt)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAuthorizationRejected, err)
	}
	if digest != proof.RequestIdentityDigest {
		return fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrApprovalWrongRequestIdentity)
	}
	return nil
}

// verifyCustodyApprovalClaims checks the receipt-scoped claims of proof against the receipt it
// selects. A claim is compared only when both sides set it, so an unset optional claim never
// refuses a proof.
func verifyCustodyApprovalClaims(proof CustodyApprovalProof, receipt CustodyReceipt) error {
	if receipt.CandidateSetDigest != "" && receipt.CandidateSetDigest != proof.CandidateSetDigest {
		return fmt.Errorf("%w: approval candidate_set_digest does not match receipt %q", ErrAuthorizationRejected, receipt.ReceiptID)
	}
	if proof.AllowedAction != "" && receipt.AllowedAction != "" && proof.AllowedAction != receipt.AllowedAction {
		return fmt.Errorf("%w: approval allowed_action %q does not match receipt %q", ErrAuthorizationRejected, proof.AllowedAction, receipt.ReceiptID)
	}
	if proof.TargetRevision > 0 && receipt.Revision > 0 && proof.TargetRevision != receipt.Revision {
		return fmt.Errorf("%w: approval target_revision %d does not match receipt %q revision %d", ErrAuthorizationRejected, proof.TargetRevision, receipt.ReceiptID, receipt.Revision)
	}
	if proof.ReceiptID != "" && receipt.ReceiptID != "" && proof.ReceiptID != receipt.ReceiptID {
		return fmt.Errorf("%w: approval names receipt %q, not %q", ErrAuthorizationRejected, proof.ReceiptID, receipt.ReceiptID)
	}
	if proof.AcceptingRole != "" && receipt.AcceptingRole != "" && proof.AcceptingRole != receipt.AcceptingRole {
		return fmt.Errorf("%w: approval names accepting role %q, not %q", ErrAuthorizationRejected, proof.AcceptingRole, receipt.AcceptingRole)
	}
	return nil
}

func custodyHasCandidateSet(receipts []CustodyReceipt, digest string) bool {
	for i := range receipts {
		if receipts[i].CandidateSetDigest == digest {
			return true
		}
	}
	return false
}

func custodyKindKnown(kind string) bool {
	switch kind {
	case "preexisting_adoption", "stage_to_ship_handoff", "ship_task_custody":
		return true
	}
	return false
}

func custodyProfileKnown(profile string) bool {
	return profile == custodyProfileOperator || profile == "agent_workflow_handoff"
}

func custodyStatusKnown(status string) bool {
	switch status {
	case "offered", "accepted", "rejected", "superseded", "invalidated":
		return true
	}
	return false
}

func custodyProducerRoleKnown(role string) bool {
	switch role {
	case "operator", "stage", "ship", "orchestrator":
		return true
	}
	return false
}

func custodyAcceptingRoleKnown(role string) bool {
	switch role {
	case "operator", "stage", "ship":
		return true
	}
	return false
}

func custodyActionKnown(action string) bool {
	switch action {
	case "stage_artifact_commit", "preexisting_adoption_commit", "ship_task_commit":
		return true
	}
	return false
}

func custodyUnmodeledChangeKind(kind string) bool {
	switch kind {
	case "unmerged", "copied":
		return true
	}
	return false
}

func custodyChangeKindKnown(kind string) bool {
	switch kind {
	case "added", "modified", "deleted", "renamed", "staged_and_unstaged":
		return true
	}
	return false
}

func custodyStagedStateKnown(state string) bool {
	switch state {
	case "staged_only", "unstaged_only", "staged_matches_worktree", custodyStagedDivergesOK:
		return true
	}
	return false
}

func custodyContributorKnown(contributor string) bool {
	switch contributor {
	case "operator", "stage", "ship", custodyContributorMixed:
		return true
	}
	return false
}

func custodySourceKindKnown(source string) bool {
	switch source {
	case "stage_artifact", "operator_config", "operator_code_or_doc", "stash_companion":
		return true
	}
	return false
}
