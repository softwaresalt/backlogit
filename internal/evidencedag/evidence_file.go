package evidencedag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

const (
	maxRecordBytes   = faultline.MaxArtifactBytes + 8192
	maxRelPathBytes  = 512
	maxPathComponent = 128
)

// validateRelPath rejects anything that is not a plain relative path made of
// visible components: no "..", no absolute or rooted or drive-qualified forms,
// no alternate data streams, and no hidden (dot-prefixed) or underscore-
// prefixed components.
var shortNamePattern = regexp.MustCompile(`~[0-9]`)

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// reservedDeviceName reports Windows device names (with or without an
// extension), which open devices instead of files.
func reservedDeviceName(p string) bool {
	stem := strings.ToUpper(strings.TrimRight(strings.SplitN(p, ".", 2)[0], " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$", "CLOCK$":
		return true
	}
	for _, pre := range []string{"COM", "LPT"} {
		if rest, ok := strings.CutPrefix(stem, pre); ok {
			switch rest {
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
				return true
			}
		}
	}
	return false
}

func validateRelPath(rel string) ([]string, error) {
	if rel == "" || len(rel) > maxRelPathBytes || strings.ContainsRune(rel, 0) {
		return nil, errors.New("evidence path is empty, too long or contains NUL")
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return nil, errors.New("evidence path must be relative")
	}
	// Both separators are normalised first, so every mixed form ("a/\\b",
	// "a\\/b", a trailing separator) yields an empty component and is refused.
	parts := strings.Split(strings.ReplaceAll(rel, `\`, "/"), "/")
	for _, p := range parts {
		if p == "" {
			return nil, errors.New("evidence path has empty components")
		}
	}
	for _, p := range parts {
		switch {
		case p == "." || p == "..":
			return nil, errors.New("evidence path must not contain . or .. components")
		case strings.HasPrefix(p, ".") || strings.HasPrefix(p, "_"):
			return nil, errors.New("evidence path must not contain hidden or underscore-prefixed components")
		case strings.ContainsAny(p, `:*?"<>|`) || len(p) > maxPathComponent || hasControl(p):
			return nil, errors.New("evidence path component is not allowed")
		case strings.HasSuffix(p, ".") || strings.HasSuffix(p, " "):
			return nil, errors.New("evidence path component must not end with a dot or space")
		case reservedDeviceName(p):
			return nil, errors.New("evidence path component is a reserved device name")
		case shortNamePattern.MatchString(p):
			return nil, errors.New("evidence path component looks like an 8.3 short name")
		}
	}
	if !filepath.IsLocal(filepath.Join(parts...)) {
		return nil, errors.New("evidence path is not local")
	}
	return parts, nil
}

// LoadEvidenceFile reads one evidence record at rel under root. The path is
// validated and contained by canonicalising root and opening through os.Root;
// every component must be a real directory and the target a regular file
// (symlinks, junctions and other reparse points are refused) and the opened
// descriptor must be the very file that was inspected. The read is bounded and
// the record decoded strictly. A residual time-of-check/time-of-use window
// remains between the inspection and the open for a writer that can swap
// directories inside the evidence root, which is why that root must be
// writable only by trusted producers.
func LoadEvidenceFile(root, rel string) (EvidenceRecord, error) {
	var zero EvidenceRecord
	if root == "" {
		return zero, errors.New("evidencedag: evidence root is required")
	}
	parts, err := validateRelPath(rel)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: %w", err)
	}
	canon, err := filepath.EvalSymlinks(root)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: resolve evidence root: %w", err)
	}
	ri, err := os.Lstat(canon)
	if err != nil || !ri.IsDir() {
		return zero, errors.New("evidencedag: evidence root is not a directory")
	}
	r, err := os.OpenRoot(canon)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: open evidence root: %w", err)
	}
	defer r.Close()

	var leaf fs.FileInfo
	for i := range parts {
		fi, err := r.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return zero, fmt.Errorf("evidencedag: stat evidence path: %w", err)
		}
		if fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return zero, errors.New("evidencedag: evidence path crosses a symlink or reparse point")
		}
		if i < len(parts)-1 && !fi.IsDir() {
			return zero, errors.New("evidencedag: evidence path component is not a directory")
		}
		if i == len(parts)-1 {
			if !fi.Mode().IsRegular() {
				return zero, errors.New("evidencedag: evidence file is not a regular file")
			}
			leaf = fi
		}
	}
	f, err := r.OpenFile(strings.Join(parts, "/"), os.O_RDONLY|readOnlyOpenFlags, 0)
	if err != nil {
		return zero, fmt.Errorf("evidencedag: open evidence file: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || !os.SameFile(leaf, fi) {
		return zero, errors.New("evidencedag: evidence file is not a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxRecordBytes+1))
	if err != nil {
		return zero, fmt.Errorf("evidencedag: read evidence file: %w", err)
	}
	if len(raw) > maxRecordBytes {
		return zero, fmt.Errorf("evidencedag: evidence file exceeds %d bytes", maxRecordBytes)
	}
	return decodeRecord(raw)
}

func decodeRecord(raw []byte) (EvidenceRecord, error) {
	var rec EvidenceRecord
	if err := rejectDuplicateKeys(raw); err != nil {
		return rec, fmt.Errorf("evidencedag: evidence record: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rec); err != nil {
		return EvidenceRecord{}, fmt.Errorf("evidencedag: evidence record: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return EvidenceRecord{}, errors.New("evidencedag: evidence record has trailing data")
	}
	return rec, nil
}
