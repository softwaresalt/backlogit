package evidencedag

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

const maxTrustKeys = 64

var principalPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{0,95}$`)

type trustKeyDoc struct {
	KeyID     string `json:"key_id"`
	Principal string `json:"principal"`
	Kind      string `json:"kind"`
	PublicKey string `json:"public_key"`
}

type trustPolicyDoc struct {
	Principal string   `json:"principal"`
	Families  []string `json:"families"`
}

type trustDocument struct {
	SchemaVersion int               `json:"schema_version"`
	Keys          *[]trustKeyDoc    `json:"keys"`
	Policy        *[]trustPolicyDoc `json:"policy"`
}

func trustErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrTrustInvalid, fmt.Sprintf(format, args...))
}

// ParseTrust decodes a trust configuration document: the set of trusted
// ed25519 verification keys (each bound to exactly one principal and a key
// kind) and the explicit authorization policy mapping a principal to the node
// families it may waive. Decoding is strict and bounded; nothing is trusted
// by default.
func ParseTrust(data []byte) (*Trust, error) {
	if len(data) > MaxTrustBytes {
		return nil, trustErr("document exceeds %d bytes", MaxTrustBytes)
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, trustErr("document is empty")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return nil, trustErr("%s", truncate(err.Error()))
	}
	var doc trustDocument
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, trustErr("%s", truncate(err.Error()))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, trustErr("trailing data after document")
	}
	if doc.SchemaVersion != TrustSchemaVersion {
		return nil, fmt.Errorf("%w: %w: got %d, want %d", ErrTrustInvalid, ErrUnsupportedVersion, doc.SchemaVersion, TrustSchemaVersion)
	}
	if doc.Keys == nil || doc.Policy == nil {
		return nil, trustErr("keys and policy must both be declared (possibly empty)")
	}
	if len(*doc.Keys) > maxTrustKeys {
		return nil, trustErr("more than %d keys", maxTrustKeys)
	}
	t := &Trust{keys: map[string]TrustedKey{}, policy: map[string]map[Family]struct{}{}}
	principals := map[string]struct{}{}
	for _, k := range *doc.Keys {
		if !validPrincipalName(k.KeyID) || !validPrincipalName(k.Principal) {
			return nil, trustErr("key_id and principal must match %s", principalPattern)
		}
		kind := KeyKind(k.Kind)
		if kind != KeyKindHuman && kind != KeyKindAutomation {
			return nil, trustErr("key %q has unknown kind", truncate(k.KeyID))
		}
		pub, err := base64.StdEncoding.Strict().DecodeString(k.PublicKey)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return nil, trustErr("key %q has an invalid ed25519 public key", truncate(k.KeyID))
		}
		if _, dup := t.keys[k.KeyID]; dup {
			return nil, trustErr("duplicate key_id %q", truncate(k.KeyID))
		}
		t.keys[k.KeyID] = TrustedKey{KeyID: k.KeyID, Principal: k.Principal, Kind: kind, PublicKey: ed25519.PublicKey(pub)}
		principals[k.Principal] = struct{}{}
	}
	for _, p := range *doc.Policy {
		if _, ok := principals[p.Principal]; !ok {
			return nil, trustErr("policy principal %q has no trusted key", truncate(p.Principal))
		}
		if _, dup := t.policy[p.Principal]; dup {
			return nil, trustErr("duplicate policy for principal %q", truncate(p.Principal))
		}
		if len(p.Families) == 0 {
			return nil, trustErr("policy for %q grants no family", truncate(p.Principal))
		}
		set := map[Family]struct{}{}
		for _, f := range p.Families {
			if !knownFamily(Family(f)) {
				return nil, trustErr("policy for %q names unknown family %q", truncate(p.Principal), truncate(f))
			}
			set[Family(f)] = struct{}{}
		}
		t.policy[p.Principal] = set
	}
	return t, nil
}

// authorized reports whether principal may waive nodes of family f. Anything
// not explicitly granted is denied.
func (t *Trust) authorized(principal string, f Family) bool {
	if t == nil {
		return false
	}
	_, ok := t.policy[principal][f]
	return ok
}

// within reports whether p is root or lies beneath it.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// LoadTrust reads the trust configuration at the absolute path. The file must
// be a real regular file (a symlink or reparse point leaf is refused) and,
// after resolving its directory, must lie outside every forbidden root (the
// agent-writable evidence and workspace-output roots), so the keys and
// authorization policy cannot be supplied through the same channel as the
// evidence they are meant to authorize. At least one forbidden root is
// required: an empty list or an empty root is a configuration error, never a
// wildcard.
func LoadTrust(path string, forbiddenRoots ...string) (*Trust, error) {
	if !filepath.IsAbs(path) {
		return nil, trustErr("trust path must be absolute")
	}
	dir, name, err := splitCanonical(path)
	if err != nil {
		return nil, trustErr("resolve trust path: %v", err)
	}
	if err := checkOutsideRoots(filepath.Join(dir, name), forbiddenRoots); err != nil {
		return nil, trustErr("%v", err)
	}
	raw, exists, err := readRegularBounded(dir, name, MaxTrustBytes)
	switch {
	case err != nil && errors.Is(err, errTooLarge):
		return nil, trustErr("document exceeds %d bytes", MaxTrustBytes)
	case err != nil:
		return nil, trustErr("read trust file (symlinks and non-regular files are refused): %v", err)
	case !exists:
		return nil, trustErr("trust file does not exist")
	}
	return ParseTrust(raw)
}
