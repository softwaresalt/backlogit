// Package canonical provides one canonical JSON serialization and SHA-256 hash
// seam for gate evidence and gate report payloads.
//
// It is a stdlib-only leaf package: it imports ZERO internal packages so it can
// be shared across the one-way core->db boundary (and any future consumer)
// without risking an import cycle. All governed gate-evidence hashing SHOULD
// route through Canonicalize/Hash so payload bytes are deterministic and any
// change to the hashing scheme is caught by the committed golden vectors.
//
// The canonical byte contract (v1) is deliberately narrow and fail-closed:
//   - integers only for numbers (non-integers must be carried as strings),
//   - strings and object keys MUST be valid UTF-8 (invalid UTF-8 fails closed
//     so distinct byte sequences can never collide on the U+FFFD replacement),
//   - CR/CRLF normalized to LF in string values AND object keys (never deleted),
//   - object keys sorted by Go string (UTF-8 byte) order of their NORMALIZED
//     (post CR->LF) form — a deliberate divergence from RFC 8785's UTF-16
//     ordering — with any post-normalization key collision rejected,
//   - exactly one trailing LF appended to the serialized root.
package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrNonIntegerNumber is returned (wrapped) when a numeric value cannot be
// represented as a canonical integer. Callers MUST carry non-integer numbers as
// strings so the canonical form stays fail-closed and lossless.
var ErrNonIntegerNumber = errors.New("canonical: non-integer number")

// ErrUnsupportedType is returned (wrapped) when a value's dynamic type is not
// part of the supported canonical value set (nil, bool, integer numerics,
// string, []any, map[string]any).
var ErrUnsupportedType = errors.New("canonical: unsupported type")

// ErrInvalidUTF8 is returned (wrapped) when a string value or object key is not
// valid UTF-8. An integrity primitive MUST fail closed here: ranging a Go string
// silently replaces each invalid byte with U+FFFD, so distinct invalid byte
// sequences would otherwise collapse to identical canonical bytes and hashes.
var ErrInvalidUTF8 = errors.New("canonical: invalid UTF-8")

// ErrDuplicateKey is returned (wrapped) when two distinct object keys collapse
// to the same name after CR/CRLF->LF normalization (e.g. "a\rb" and "a\nb").
// Emitting both would produce an ambiguous object with duplicate names, so the
// collision fails closed.
var ErrDuplicateKey = errors.New("canonical: duplicate key after line-ending normalization")

// Canonicalize returns the canonical UTF-8 encoding of v, terminated by exactly
// one trailing LF.
func Canonicalize(v any) ([]byte, error) {
	var sb strings.Builder
	if err := encode(v, &sb); err != nil {
		return nil, err
	}
	sb.WriteByte('\n')
	return []byte(sb.String()), nil
}

// Hash returns the lowercase-hex SHA-256 of Canonicalize(v).
func Hash(v any) (string, error) {
	b, err := Canonicalize(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// encode writes the canonical encoding of v into sb. It supports the narrow
// value set only; every other dynamic type fails closed with ErrUnsupportedType.
func encode(v any, sb *strings.Builder) error {
	switch x := v.(type) {
	case nil:
		sb.WriteString("null")
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case string:
		return encodeString(x, sb)
	case json.Number:
		return encodeJSONNumber(x, sb)
	case int:
		sb.WriteString(strconv.FormatInt(int64(x), 10))
	case int8:
		sb.WriteString(strconv.FormatInt(int64(x), 10))
	case int16:
		sb.WriteString(strconv.FormatInt(int64(x), 10))
	case int32:
		sb.WriteString(strconv.FormatInt(int64(x), 10))
	case int64:
		sb.WriteString(strconv.FormatInt(x, 10))
	case uint:
		sb.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint8:
		sb.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint16:
		sb.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint32:
		sb.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint64:
		sb.WriteString(strconv.FormatUint(x, 10))
	case float32:
		return encodeFloat(float64(x), v, sb)
	case float64:
		return encodeFloat(x, v, sb)
	case []any:
		return encodeArray(x, sb)
	case map[string]any:
		return encodeObject(x, sb)
	default:
		return fmt.Errorf("canonical: %T: %w", v, ErrUnsupportedType)
	}
	return nil
}

// numErr wraps ErrNonIntegerNumber for a numeric value that cannot be rendered
// as a canonical integer (fractional, non-finite, or out of int64/uint64 range).
func numErr(v any) error {
	return fmt.Errorf("canonical: %v: %w", v, ErrNonIntegerNumber)
}

// encodeJSONNumber renders a json.Number only when its literal is a clean
// integer; anything with a fraction or exponent fails closed.
func encodeJSONNumber(n json.Number, sb *strings.Builder) error {
	s := string(n)
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		sb.WriteString(strconv.FormatInt(i, 10))
		return nil
	}
	if u, err := strconv.ParseUint(s, 10, 64); err == nil {
		sb.WriteString(strconv.FormatUint(u, 10))
		return nil
	}
	return numErr(n)
}

// encodeFloat renders a float only when it is finite, exactly integral, and
// within int64/uint64 range. A round-trip through the integer type confirms
// both the integrality and the range in one step (fail-closed on any mismatch).
func encodeFloat(f float64, orig any, sb *strings.Builder) error {
	// Reject NaN and ±Inf. NaN is never equal to itself; ±Inf falls outside the
	// finite MaxFloat64 magnitude.
	const maxFloat64 = 1.7976931348623157e308
	if f != f || f > maxFloat64 || f < -maxFloat64 {
		return numErr(orig)
	}
	// twoPow64 (2^64) is exactly representable as a float64. The bound MUST be a
	// strict "< 2^64": the largest uint64 (2^64-1) is NOT representable as a
	// float64 and rounds up to 2^64, so a "<= MaxUint64" literal bound would
	// admit exactly 2^64 into uint64(f), which is implementation-specific and can
	// saturate to serialize 2^64 as 18446744073709551615 on some architectures.
	const twoPow64 = 18446744073709551616.0
	if f >= 0 && f < twoPow64 { // [0, 2^64)
		if u := uint64(f); float64(u) == f {
			sb.WriteString(strconv.FormatUint(u, 10))
			return nil
		}
		return numErr(orig)
	}
	if f < 0 && f >= -9223372036854775808.0 { // [MinInt64, 0)
		if i := int64(f); float64(i) == f {
			sb.WriteString(strconv.FormatInt(i, 10))
			return nil
		}
		return numErr(orig)
	}
	return numErr(orig)
}

// normalizeLineEndings converts CRLF then any lone CR to LF. CR is converted to
// a distinct LF byte, never deleted. Line-ending normalization preserves UTF-8
// validity (it only rewrites the ASCII bytes CR and LF).
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// encodeString writes a canonical JSON string: it fails closed on invalid UTF-8,
// normalizes line endings to LF, then applies minimal escaping. Non-ASCII runes
// are emitted literally as UTF-8 and "/" is never escaped.
func encodeString(s string, sb *strings.Builder) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("canonical: %w", ErrInvalidUTF8)
	}
	normalized := normalizeLineEndings(s)

	sb.WriteByte('"')
	for _, r := range normalized {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\t':
			sb.WriteString(`\t`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		default:
			if r < 0x20 {
				// Other control characters: \u00xx with lowercase hex.
				sb.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
	return nil
}

// encodeArray writes elements in order (array order is semantic, never sorted).
func encodeArray(arr []any, sb *strings.Builder) error {
	sb.WriteByte('[')
	for i, e := range arr {
		if i > 0 {
			sb.WriteByte(',')
		}
		if err := encode(e, sb); err != nil {
			return err
		}
	}
	sb.WriteByte(']')
	return nil
}

// encodeObject writes keys sorted ascending by the Go string (UTF-8 byte) order
// of their NORMALIZED (post CR->LF) form. Sorting and emitting both use the
// normalized key so the emitted key order always matches the emitted key bytes,
// and any two distinct raw keys that normalize to the same name are rejected as
// a duplicate-key collision (fail closed). Invalid-UTF-8 keys also fail closed.
func encodeObject(m map[string]any, sb *strings.Builder) error {
	type keyEntry struct {
		normalized string
		value      any
	}
	entries := make([]keyEntry, 0, len(m))
	seen := make(map[string]struct{}, len(m))
	for k, v := range m {
		if !utf8.ValidString(k) {
			return fmt.Errorf("canonical: object key: %w", ErrInvalidUTF8)
		}
		nk := normalizeLineEndings(k)
		if _, dup := seen[nk]; dup {
			return fmt.Errorf("canonical: key %q: %w", nk, ErrDuplicateKey)
		}
		seen[nk] = struct{}{}
		entries = append(entries, keyEntry{normalized: nk, value: v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].normalized < entries[j].normalized })

	sb.WriteByte('{')
	for i, e := range entries {
		if i > 0 {
			sb.WriteByte(',')
		}
		// e.normalized is already CR-normalized and valid UTF-8, so encodeString
		// re-emits identical bytes to the sort key.
		if err := encodeString(e.normalized, sb); err != nil {
			return err
		}
		sb.WriteByte(':')
		if err := encode(e.value, sb); err != nil {
			return err
		}
	}
	sb.WriteByte('}')
	return nil
}
