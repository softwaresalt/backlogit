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
//   - object keys sorted by Go string (UTF-8 byte) order — a deliberate
//     divergence from RFC 8785's UTF-16 ordering,
//   - CR/CRLF normalized to LF in string values (never deleted),
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
)

// ErrNonIntegerNumber is returned (wrapped) when a numeric value cannot be
// represented as a canonical integer. Callers MUST carry non-integer numbers as
// strings so the canonical form stays fail-closed and lossless.
var ErrNonIntegerNumber = errors.New("canonical: non-integer number")

// ErrUnsupportedType is returned (wrapped) when a value's dynamic type is not
// part of the supported canonical value set (nil, bool, integer numerics,
// string, []any, map[string]any).
var ErrUnsupportedType = errors.New("canonical: unsupported type")

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
		encodeString(x, sb)
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
	if f >= 0 && f <= 18446744073709551615.0 { // [0, 2^64)
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

// encodeString writes a canonical JSON string: line endings normalized to LF,
// then minimal escaping. Non-ASCII runes are emitted literally as UTF-8 and "/"
// is never escaped.
func encodeString(s string, sb *strings.Builder) {
	// Normalize line endings first: CRLF -> LF, then any lone CR -> LF. CR is
	// converted to a distinct LF byte, never deleted.
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

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

// encodeObject writes keys sorted ascending by Go string (UTF-8 byte) order.
func encodeObject(m map[string]any, sb *strings.Builder) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		encodeString(k, sb)
		sb.WriteByte(':')
		if err := encode(m[k], sb); err != nil {
			return err
		}
	}
	sb.WriteByte('}')
	return nil
}
