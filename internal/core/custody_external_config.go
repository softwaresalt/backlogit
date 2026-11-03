package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// Environment variables that name the external custody files. The CLI flags override
// them. MCP reads them only, so a remote MCP caller cannot choose the files.
const (
	// CustodyOperatorPinsEnv names the environment variable holding the operator pins file path.
	CustodyOperatorPinsEnv = "BACKLOGIT_CUSTODY_OPERATOR_PINS_FILE"
	// CustodyNonceStoreEnv names the environment variable holding the external nonce store path.
	CustodyNonceStoreEnv = "BACKLOGIT_CUSTODY_NONCE_STORE_PATH"
)

// Sentinel errors for the external custody configuration (211-S decisions 1a and 2).
var (
	// ErrCustodyExternalPathInWorkspace reports an external file that resolves inside the
	// workspace root. The file is refused without being read.
	ErrCustodyExternalPathInWorkspace = errors.New("custody external file resolves inside the workspace root (fail closed)")
	// ErrCustodyOperatorPinsInvalid reports an operator pins file that is unreadable, fails
	// strict decoding, or yields neither trust anchors nor pins.
	ErrCustodyOperatorPinsInvalid = errors.New("custody operator pins file is invalid (fail closed)")
)

// custodyPinsMaxBytes bounds the operator pins file read.
const custodyPinsMaxBytes = 1 << 20

// custodyNonceStoreLockTimeout bounds how long a consume or redemption waits for the store lock.
const custodyNonceStoreLockTimeout = 5 * time.Second

// custodyNonceStoreLockPoll is the delay between lock acquisition attempts.
const custodyNonceStoreLockPoll = 10 * time.Millisecond

// CustodyPathFromFlagOrEnv returns flagValue when it is set, and otherwise the value of
// the environment variable envName. The flag overrides the environment.
func CustodyPathFromFlagOrEnv(flagValue, envName string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(envName)
}

// custodyFileNonceStore is the external file-backed nonce store. When NonceStorePath is set it
// is the atomic consume authority for live operator accepts and an additional redemption
// source for verify.
//
// Trust model: the file lives outside the workspace root, so workspace-writable access cannot
// rewrite it. It is NOT rollback-resistant against a hostile process running as the same OS
// user, which can edit or delete it. That is the cooperative trust model (211-S waiver 1b,
// recorded in Plan Section 5.2 #5). The external path removes only the workspace-writable
// exposure.
type custodyFileNonceStore struct {
	path        string
	lockTimeout time.Duration
	rename      func(oldpath, newpath string) error
	now         func() time.Time
}

// newCustodyFileNonceStore returns a store over the file at path.
func newCustodyFileNonceStore(path string) *custodyFileNonceStore {
	return &custodyFileNonceStore{path: path, lockTimeout: custodyNonceStoreLockTimeout, rename: os.Rename, now: time.Now}
}

// CompareAndConsumeNonce implements NonceConsumeProvider without a request-identity digest.
func (s *custodyFileNonceStore) CompareAndConsumeNonce(ctx context.Context, nonce string, notAfter time.Time) error {
	return s.CompareAndConsumeNonceBound(ctx, nonce, notAfter, "")
}

// CompareAndConsumeNonceBound atomically checks and records nonce with its expiry, consume
// time, and digest. A replayed or expired nonce is refused and nothing is recorded. A lock,
// read, or commit failure returns an error and records nothing, so success is never reported
// for a write that did not persist.
func (s *custodyFileNonceStore) CompareAndConsumeNonceBound(ctx context.Context, nonce string, notAfter time.Time, requestIdentityDigest string) (err error) {
	if nonce == "" {
		return ErrNonceInvalid
	}
	now := s.now().UTC()
	if now.After(notAfter) {
		return ErrNonceExpired
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, unlock()) }()
	records, err := s.ledger().load()
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.Nonce == nonce {
			return ErrNonceReplayed
		}
	}
	records = append(records, nonceLedgerRecord{
		Nonce: nonce, NotAfter: notAfter.UTC(), RequestIdentityDigest: requestIdentityDigest, ConsumedAt: &now,
	})
	return s.commit(records)
}

// RecordNonceConsume implements NonceConsumeAuditor. The record written by
// CompareAndConsumeNonceBound already holds the consume audit fields, so there is nothing
// further to persist here.
func (s *custodyFileNonceStore) RecordNonceConsume(NonceConsumeEvent) error {
	return nil
}

// ledger returns a NonceLedger over the store file for its record format and corruption checks.
func (s *custodyFileNonceStore) ledger() *NonceLedger {
	return &NonceLedger{path: s.path}
}

// lock acquires the cross-process lock file beside the store. Creating it exclusively means only
// one process holds it. A lock that is not acquired before lockTimeout is an error. A lock left
// by a crashed process keeps failing closed until an operator removes it.
func (s *custodyFileNonceStore) lock(ctx context.Context) (func() error, error) {
	lockPath := s.path + ".lock"
	deadline := s.now().Add(s.lockTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if cerr := f.Close(); cerr != nil {
				_ = os.Remove(lockPath) // close error takes precedence
				return nil, fmt.Errorf("close nonce store lock: %w", cerr)
			}
			return func() error {
				if rerr := os.Remove(lockPath); rerr != nil {
					return fmt.Errorf("release nonce store lock: %w", rerr)
				}
				return nil
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire nonce store lock: %w", err)
		}
		if !s.now().Before(deadline) {
			return nil, fmt.Errorf("acquire nonce store lock %s: held by another process", lockPath)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acquire nonce store lock: %w", ctx.Err())
		case <-time.After(custodyNonceStoreLockPoll):
		}
	}
}

// commit replaces the store with records. It writes a temp file in the same directory, fsyncs it,
// and renames it over the store. A nil return means the records persisted.
func (s *custodyFileNonceStore) commit(records []nonceLedgerRecord) error {
	if info, statErr := os.Lstat(s.path); statErr == nil {
		linked, linkErr := IsSymlinkOrReparsePoint(info, s.path)
		if linkErr != nil {
			return fmt.Errorf("check nonce store: %w", linkErr)
		}
		if linked {
			return fmt.Errorf("nonce store %s is a symlink or reparse point: %w", s.path, ErrNonceLedgerCorrupt)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat nonce store: %w", statErr)
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".custody-nonce-store-*")
	if err != nil {
		return fmt.Errorf("create nonce store temp: %w", err)
	}
	tmpName := tmp.Name()
	enc := json.NewEncoder(tmp)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			_ = tmp.Close()        // encode error takes precedence
			_ = os.Remove(tmpName) // best-effort cleanup
			return fmt.Errorf("write nonce store: %w", err)
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()        // sync error takes precedence
		_ = os.Remove(tmpName) // best-effort cleanup
		return fmt.Errorf("sync nonce store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName) // best-effort cleanup
		return fmt.Errorf("close nonce store temp: %w", err)
	}
	if err := s.rename(tmpName, s.path); err != nil {
		_ = os.Remove(tmpName) // best-effort cleanup
		return fmt.Errorf("replace nonce store: %w", err)
	}
	return syncCustodyDir(dir)
}

// lookup returns the request-identity digest and consume time recorded for nonce, and whether
// nonce is recorded. A lock or read failure is an error, so the caller fails closed.
func (s *custodyFileNonceStore) lookup(nonce string) (string, time.Time, bool, error) {
	unlock, err := s.lock(context.Background())
	if err != nil {
		return "", time.Time{}, false, err
	}
	records, loadErr := s.ledger().load()
	if err := errors.Join(loadErr, unlock()); err != nil {
		return "", time.Time{}, false, err
	}
	for _, r := range records {
		if r.Nonce != nonce {
			continue
		}
		var at time.Time
		if r.ConsumedAt != nil {
			at = *r.ConsumedAt
		}
		return r.RequestIdentityDigest, at, true, nil
	}
	return "", time.Time{}, false, nil
}

// syncCustodyDir fsyncs dir so a completed rename survives a crash. Windows cannot fsync a
// directory handle, so the call is skipped there.
func syncCustodyDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open nonce store directory: %w", err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close() // sync error takes precedence
		return fmt.Errorf("sync nonce store directory: %w", err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("close nonce store directory: %w", err)
	}
	return nil
}

// resolveCustodyPinsPath canonicalizes an existing pins file and refuses one inside wsRoot.
func resolveCustodyPinsPath(wsRoot, path string) (string, error) {
	return custodyExternalPath(wsRoot, path, false)
}

// resolveCustodyNonceStorePath canonicalizes a nonce store path and refuses one inside wsRoot.
// The file itself may not exist yet, but its directory must.
func resolveCustodyNonceStorePath(wsRoot, path string) (string, error) {
	return custodyExternalPath(wsRoot, path, true)
}

// custodyExternalPath returns the canonical form of path and refuses one inside wsRoot. Both
// paths are canonicalized the same way. A missing final element is allowed only when allowMissing
// is set, and then its parent must exist.
func custodyExternalPath(wsRoot, path string, allowMissing bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("custody external path is empty: %w", blerrors.ErrValidation)
	}
	root, err := custodyCanonicalPath(wsRoot, false)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w: %w", blerrors.ErrValidation, err)
	}
	canon, err := custodyCanonicalPath(path, allowMissing)
	if err != nil {
		return "", fmt.Errorf("%w: %w", blerrors.ErrValidation, err)
	}
	if custodyWithinRoot(root, canon, runtime.GOOS == "windows") {
		return "", fmt.Errorf("external custody file %s is inside workspace root %s: %w: %w",
			canon, root, ErrCustodyExternalPathInWorkspace, blerrors.ErrValidation)
	}
	return canon, nil
}

// custodyCanonicalPath returns the absolute, symlink-resolved form of path. When allowMissing is
// set and path does not exist, its parent is resolved instead and the base name is kept.
func custodyCanonicalPath(path string, allowMissing bool) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	canon, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return canon, nil
	}
	if !allowMissing || !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", fmt.Errorf("resolve parent of %s: %w", path, err)
	}
	return filepath.Join(dir, filepath.Base(abs)), nil
}

// custodyWithinRoot reports whether path is root or lies beneath it. Both must be canonical
// absolute paths. foldCase compares them case-insensitively, as Windows file systems do. A
// sibling that shares only a name prefix with root is outside it.
func custodyWithinRoot(root, path string, foldCase bool) bool {
	if foldCase {
		root = strings.ToLower(root)
		path = strings.ToLower(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// custodyOperatorPinsFile is the operator pins file. Unknown keys are refused.
type custodyOperatorPinsFile struct {
	TrustAnchors []config.TrustAnchor `yaml:"trust_anchors"`
	Pins         []custodyOperatorPin `yaml:"pins"`
}

// custodyOperatorPin pins one fingerprint to the externally authoritative role.
type custodyOperatorPin struct {
	Fingerprint string `yaml:"fingerprint"`
	Role        string `yaml:"role"`
}

// loadCustodyOperatorPins reads and strictly decodes an operator pins file outside wsRoot. The
// file must yield at least one trust anchor or pin. Any failure fails closed.
func loadCustodyOperatorPins(wsRoot, path string) ([]config.TrustAnchor, *ExternalPinSet, error) {
	canon, err := resolveCustodyPinsPath(wsRoot, path)
	if err != nil {
		return nil, nil, err
	}
	anchors, pins, err := readCustodyOperatorPins(canon)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", blerrors.ErrValidation, err)
	}
	return anchors, pins, nil
}

// readCustodyOperatorPins decodes the canonical pins file at path.
func readCustodyOperatorPins(path string) ([]config.TrustAnchor, *ExternalPinSet, error) {
	f, err := openCustodyNoFollow(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: open %s: %w", ErrCustodyOperatorPinsInvalid, path, err)
	}
	defer func() { _ = f.Close() }() // read-only handle
	raw, err := custodyReadBounded(f, custodyPinsMaxBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: read %s: %w", ErrCustodyOperatorPinsInvalid, path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var doc custodyOperatorPinsFile
	if err := dec.Decode(&doc); err != nil {
		return nil, nil, fmt.Errorf("%w: decode %s: %w", ErrCustodyOperatorPinsInvalid, path, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, nil, fmt.Errorf("%w: %s must hold exactly one YAML document", ErrCustodyOperatorPinsInvalid, path)
	}
	if len(doc.TrustAnchors) == 0 && len(doc.Pins) == 0 {
		return nil, nil, fmt.Errorf("%w: %s yields neither trust_anchors nor pins", ErrCustodyOperatorPinsInvalid, path)
	}
	bindings := make([]PinBinding, 0, len(doc.Pins))
	for i, p := range doc.Pins {
		if p.Fingerprint == "" || p.Role == "" {
			return nil, nil, fmt.Errorf("%w: pins[%d] requires a fingerprint and a role", ErrCustodyOperatorPinsInvalid, i)
		}
		bindings = append(bindings, PinBinding{Fingerprint: p.Fingerprint, Role: p.Role})
	}
	// Always non-nil, so workspace config anchors are never consulted. An anchor the file does not pin cannot verify.
	return doc.TrustAnchors, NewBoundExternalPinSet(bindings...), nil
}

// custodyTrustInputs returns the operator trust inputs for one call. A set pins file replaces the
// caller-supplied anchors and pins, so workspace config anchors are then never consulted.
func custodyTrustInputs(wsRoot, pinsPath string, anchors []config.TrustAnchor, pins *ExternalPinSet) ([]config.TrustAnchor, *ExternalPinSet, error) {
	if pinsPath == "" {
		return anchors, pins, nil
	}
	return loadCustodyOperatorPins(wsRoot, pinsPath)
}

// newCustodyNonceStoreConsumer builds a live consumer over the external nonce store at storePath.
// The store is the atomic authority. The workspace NonceLedger stays the local projection.
func newCustodyNonceStoreConsumer(ws *Workspace, storePath string) (*NonceConsumer, error) {
	projection, err := OpenNonceLedger(custodyNonceLedgerPath(ws.RootPath))
	if err != nil {
		return nil, err
	}
	store := newCustodyFileNonceStore(storePath)
	return NewNonceConsumer(store, projection, store)
}

// custodyRedeemedWithStore reports redemption from the external nonce store at nonceStorePath,
// the workspace NonceLedger, and the item log. Any one source that records the nonce against
// its request identity inside the token window is enough. An empty nonceStorePath skips the
// store. A configured store that cannot be read is an error, which fails closed.
func custodyRedeemedWithStore(ws *Workspace, shipmentID, receiptID string, proof CustodyApprovalProof, nonceStorePath string) (bool, error) {
	if nonceStorePath != "" {
		digest, consumedAt, found, err := newCustodyFileNonceStore(nonceStorePath).lookup(proof.Nonce)
		if err != nil {
			return false, fmt.Errorf("read external nonce store: %w", err)
		}
		if found && digest == proof.RequestIdentityDigest && !consumedAt.IsZero() && custodyWithinTokenWindow(consumedAt, proof) {
			return true, nil
		}
	}
	return custodyRedeemed(ws, shipmentID, receiptID, proof), nil
}
