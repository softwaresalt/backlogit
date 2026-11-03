package core

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
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

	errCustodyExternalNotImplemented = errors.New("custody external configuration not implemented")
)

// CustodyPathFromFlagOrEnv returns flagValue when it is set, and otherwise the value of
// the environment variable envName. The flag overrides the environment.
func CustodyPathFromFlagOrEnv(flagValue, envName string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(envName)
}

// custodyFileNonceStore is the file-backed nonce store. It is a stub until GREEN.
type custodyFileNonceStore struct {
	path        string
	lockTimeout time.Duration
	rename      func(oldpath, newpath string) error
	now         func() time.Time
}

// newCustodyFileNonceStore returns a store over the file at path.
func newCustodyFileNonceStore(path string) *custodyFileNonceStore {
	return &custodyFileNonceStore{path: path}
}

// CompareAndConsumeNonce implements NonceConsumeProvider without a digest.
func (s *custodyFileNonceStore) CompareAndConsumeNonce(ctx context.Context, nonce string, notAfter time.Time) error {
	return errCustodyExternalNotImplemented
}

// CompareAndConsumeNonceBound atomically records nonce with its digest.
func (s *custodyFileNonceStore) CompareAndConsumeNonceBound(ctx context.Context, nonce string, notAfter time.Time, requestIdentityDigest string) error {
	return errCustodyExternalNotImplemented
}

// RecordNonceConsume implements NonceConsumeAuditor.
func (s *custodyFileNonceStore) RecordNonceConsume(ev NonceConsumeEvent) error {
	return errCustodyExternalNotImplemented
}

// resolveCustodyPinsPath canonicalizes an existing pins file and refuses one inside wsRoot.
func resolveCustodyPinsPath(wsRoot, path string) (string, error) {
	return "", errCustodyExternalNotImplemented
}

// resolveCustodyNonceStorePath canonicalizes a nonce store path and refuses one inside wsRoot.
func resolveCustodyNonceStorePath(wsRoot, path string) (string, error) {
	return "", errCustodyExternalNotImplemented
}

// custodyWithinRoot reports whether path is root or lies beneath it.
func custodyWithinRoot(root, path string, foldCase bool) bool {
	return false
}

// loadCustodyOperatorPins reads and strictly decodes an operator pins file outside wsRoot.
func loadCustodyOperatorPins(wsRoot, path string) ([]config.TrustAnchor, *ExternalPinSet, error) {
	return nil, nil, errCustodyExternalNotImplemented
}

// custodyRedeemedWithStore reports redemption from the workspace NonceLedger, the item log,
// and the external nonce store at nonceStorePath.
func custodyRedeemedWithStore(ws *Workspace, shipmentID, receiptID string, proof CustodyApprovalProof, nonceStorePath string) (bool, error) {
	return false, errCustodyExternalNotImplemented
}
