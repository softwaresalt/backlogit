package core

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
)

// Sentinel errors for the verification-key loader (168.002-T).
var (
	// ErrVerificationKeyPrivate reports private key material where only a
	// public key is permitted.
	ErrVerificationKeyPrivate = errors.New("verification key material is a private key")
	// ErrVerificationKeyMalformed reports unparseable or empty key material.
	ErrVerificationKeyMalformed = errors.New("verification key material is malformed")
	// ErrVerificationKeyAlgoMismatch reports key material whose type does not
	// match the anchor's declared algo.
	ErrVerificationKeyAlgoMismatch = errors.New("verification key type does not match anchor algo")
	// ErrVerificationKeyRefUnsafe reports a public_key_ref file path that is
	// not a contained, non-symlink regular file under the workspace root.
	ErrVerificationKeyRefUnsafe = errors.New("verification key reference is not a contained regular file")
	// ErrVerificationKeyUnreadable reports a contained file that cannot be read.
	ErrVerificationKeyUnreadable = errors.New("verification key file unreadable")
)

// VerificationKeyError is the typed error returned by the key loader.
type VerificationKeyError struct {
	// AnchorID is the id of the anchor whose key failed to load (may be empty).
	AnchorID string
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *VerificationKeyError) Error() string {
	return fmt.Sprintf("trust anchor %q: %v", e.AnchorID, e.Err)
}

// Unwrap returns the sentinel category.
func (e *VerificationKeyError) Unwrap() error { return e.Err }

// VerificationKey is a parsed, public-only verification key.
type VerificationKey struct {
	// Algo is the anchor algo the key was parsed for.
	Algo string
	// Public is the parsed public key (ed25519.PublicKey or *ecdsa.PublicKey).
	Public crypto.PublicKey
	// Fingerprint is "sha256:" + lowercase hex of the SHA-256 of the PKIX DER.
	Fingerprint string
}

// maxVerificationKeyBytes bounds how much key material is read from a file.
const maxVerificationKeyBytes = 64 * 1024

// LoadVerificationKey loads and parses the public key referenced by
// anchor.PublicKeyRef. The reference is either an inline PEM block or a
// workspace-relative file path. File references are opened through an
// os.Root handle on workspaceRoot (no escape via "..", absolute paths or
// symlinks) and any symlink path component is refused. Private keys and
// malformed material are rejected with *VerificationKeyError. Fingerprint
// pin-binding is a separate step (VerifyVerificationKeyPin).
func LoadVerificationKey(workspaceRoot string, anchor config.TrustAnchor) (*VerificationKey, error) {
	fail := func(err error) (*VerificationKey, error) {
		return nil, &VerificationKeyError{AnchorID: anchor.ID, Err: err}
	}
	ref := strings.TrimSpace(anchor.PublicKeyRef)
	if ref == "" {
		return fail(ErrVerificationKeyMalformed)
	}
	var material []byte
	if strings.HasPrefix(ref, "-----BEGIN") {
		material = []byte(ref)
	} else {
		data, err := readContainedKeyFile(workspaceRoot, ref)
		if err != nil {
			return fail(err)
		}
		material = data
	}
	key, err := parseVerificationKey(anchor.Algo, material)
	if err != nil {
		return fail(err)
	}
	return key, nil
}

// readContainedKeyFile reads ref beneath workspaceRoot without following
// symlinks, returning a sentinel-wrapping error.
func readContainedKeyFile(workspaceRoot, ref string) ([]byte, error) {
	rel := filepath.FromSlash(strings.ReplaceAll(ref, `\`, "/"))
	if !filepath.IsLocal(rel) {
		return nil, ErrVerificationKeyRefUnsafe
	}
	root, err := os.OpenRoot(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: open workspace root: %w", ErrVerificationKeyUnreadable, err)
	}
	defer func() { _ = root.Close() }()

	parts := strings.Split(rel, string(filepath.Separator))
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("%w: %w", ErrVerificationKeyUnreadable, err)
			}
			return nil, fmt.Errorf("%w: %w", ErrVerificationKeyRefUnsafe, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil, ErrVerificationKeyRefUnsafe
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, ErrVerificationKeyRefUnsafe
		}
	}
	f, err := root.Open(rel)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVerificationKeyUnreadable, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxVerificationKeyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrVerificationKeyUnreadable, err)
	}
	if len(data) > maxVerificationKeyBytes {
		return nil, ErrVerificationKeyMalformed
	}
	return data, nil
}

// parseVerificationKey parses exactly one PEM "PUBLIC KEY" block per algo.
func parseVerificationKey(algo string, material []byte) (*VerificationKey, error) {
	if algo != config.TrustAnchorAlgoEd25519 && algo != config.TrustAnchorAlgoECDSAP256 {
		return nil, ErrVerificationKeyMalformed
	}
	block, rest := pem.Decode(material)
	if block == nil {
		return nil, ErrVerificationKeyMalformed
	}
	if strings.Contains(strings.ToUpper(block.Type), "PRIVATE") {
		return nil, ErrVerificationKeyPrivate
	}
	for {
		var next *pem.Block
		next, rest = pem.Decode(rest)
		if next == nil {
			break
		}
		if strings.Contains(strings.ToUpper(next.Type), "PRIVATE") {
			return nil, ErrVerificationKeyPrivate
		}
		return nil, ErrVerificationKeyMalformed
	}
	if strings.TrimSpace(string(rest)) != "" || block.Type != "PUBLIC KEY" {
		return nil, ErrVerificationKeyMalformed
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if _, perr := x509.ParsePKCS8PrivateKey(block.Bytes); perr == nil {
			return nil, ErrVerificationKeyPrivate
		}
		if _, perr := x509.ParseECPrivateKey(block.Bytes); perr == nil {
			return nil, ErrVerificationKeyPrivate
		}
		return nil, ErrVerificationKeyMalformed
	}
	switch k := pub.(type) {
	case ed25519.PublicKey:
		if algo != config.TrustAnchorAlgoEd25519 {
			return nil, ErrVerificationKeyAlgoMismatch
		}
	case *ecdsa.PublicKey:
		if algo != config.TrustAnchorAlgoECDSAP256 || k.Curve != elliptic.P256() {
			return nil, ErrVerificationKeyAlgoMismatch
		}
	default:
		return nil, ErrVerificationKeyAlgoMismatch
	}
	return &VerificationKey{Algo: algo, Public: pub, Fingerprint: "sha256:" + canonical.HashBytes(block.Bytes)}, nil
}
