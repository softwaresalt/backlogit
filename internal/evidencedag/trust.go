package evidencedag

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
		if !principalPattern.MatchString(k.KeyID) || !principalPattern.MatchString(k.Principal) {
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
// be a real regular file and, after resolving every symlink, must lie outside
// every forbidden root (the agent-writable evidence and workspace-output
// roots), so the keys and authorization policy cannot be supplied through the
// same channel as the evidence they are meant to authenticate. An empty
// forbidden root is a configuration error, never a wildcard.
func LoadTrust(path string, forbiddenRoots ...string) (*Trust, error) {
	if !filepath.IsAbs(path) {
		return nil, trustErr("trust path must be absolute")
	}
	li, err := os.Lstat(path)
	if err != nil {
		return nil, trustErr("stat trust file: %v", err)
	}
	if !li.Mode().IsRegular() {
		return nil, trustErr("trust path is not a regular file (symlinks are refused)")
	}
	canon, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, trustErr("resolve trust path: %v", err)
	}
	for _, r := range forbiddenRoots {
		if r == "" {
			return nil, trustErr("forbidden root must not be empty")
		}
		cr, err := filepath.EvalSymlinks(r)
		if err != nil {
			return nil, trustErr("resolve forbidden root: %v", err)
		}
		if within(cr, canon) {
			return nil, trustErr("trust configuration must live outside agent-writable evidence")
		}
	}
	f, err := os.Open(canon)
	if err != nil {
		return nil, trustErr("open trust file: %v", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || !os.SameFile(fi, li) {
		return nil, trustErr("trust file changed or is not a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxTrustBytes+1))
	if err != nil {
		return nil, trustErr("read trust file: %v", err)
	}
	return ParseTrust(raw)
}
