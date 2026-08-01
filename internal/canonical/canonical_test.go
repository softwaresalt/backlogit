package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// hexOf returns the lowercase-hex SHA-256 of s. Used by the golden helper to
// derive the expected hash of a canonical byte string for the non-pinned cases.
func hexOf(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

// assertGolden pins a single input to its exact canonical byte string AND its
// expected lowercase-hex SHA-256. It checks Canonicalize bytes == wantCanonical
// and Hash == wantHash so a change to either canonicalization OR the hashing
// scheme is caught independently.
func assertGolden(t *testing.T, input any, wantCanonical, wantHash string) {
	t.Helper()
	gotBytes, err := Canonicalize(input)
	if err != nil {
		t.Fatalf("Canonicalize(%v) unexpected error: %v", input, err)
	}
	if string(gotBytes) != wantCanonical {
		t.Errorf("Canonicalize bytes mismatch:\n got=%q\nwant=%q", string(gotBytes), wantCanonical)
	}
	gotHash, err := Hash(input)
	if err != nil {
		t.Fatalf("Hash(%v) unexpected error: %v", input, err)
	}
	if gotHash != wantHash {
		t.Errorf("Hash mismatch:\n got=%q\nwant=%q", gotHash, wantHash)
	}
}

func TestCanonicalLineEndingsIdentical(t *testing.T) {
	// LF and CRLF in a string value must normalize to the same bytes and hash.
	lf := map[string]any{"s": "a\nb"}
	crlf := map[string]any{"s": "a\r\nb"}

	hLF, err := Hash(lf)
	if err != nil {
		t.Fatalf("Hash(lf) error: %v", err)
	}
	hCRLF, err := Hash(crlf)
	if err != nil {
		t.Fatalf("Hash(crlf) error: %v", err)
	}
	if hLF != hCRLF {
		t.Errorf("LF vs CRLF hash differ: lf=%s crlf=%s", hLF, hCRLF)
	}
}

func TestCanonicalLoneCRNotDeleted(t *testing.T) {
	// A lone CR must normalize to LF (a distinct byte), never be deleted into
	// adjacent text. "a\rb" -> "a\nb", which must NOT collapse to "ab".
	cr := map[string]any{"s": "a\rb"}
	want := "{\"s\":\"a\\nb\"}\n"
	gotBytes, err := Canonicalize(cr)
	if err != nil {
		t.Fatalf("Canonicalize(cr) error: %v", err)
	}
	if string(gotBytes) != want {
		t.Errorf("lone CR canonical mismatch:\n got=%q\nwant=%q", string(gotBytes), want)
	}

	hCR, err := Hash(cr)
	if err != nil {
		t.Fatalf("Hash(cr) error: %v", err)
	}
	hAB, err := Hash(map[string]any{"s": "ab"})
	if err != nil {
		t.Fatalf("Hash(ab) error: %v", err)
	}
	if hCR == hAB {
		t.Errorf("CR vanished into adjacent text: Hash(a\\rb)==Hash(ab)==%s", hCR)
	}
}

func TestCanonicalKeySorting(t *testing.T) {
	in := map[string]any{"b": 1, "a": 2}
	want := "{\"a\":2,\"b\":1}\n"
	assertGolden(t, in, want, hexOf(want))
}

func TestCanonicalArrayOrderChangesHash(t *testing.T) {
	h12, err := Hash([]any{1, 2})
	if err != nil {
		t.Fatalf("Hash([1,2]) error: %v", err)
	}
	h21, err := Hash([]any{2, 1})
	if err != nil {
		t.Fatalf("Hash([2,1]) error: %v", err)
	}
	if h12 == h21 {
		t.Errorf("array reorder did not change hash: %s", h12)
	}
}

func TestCanonicalNonIntegerErrors(t *testing.T) {
	cases := []struct {
		name  string
		input any
	}{
		{"fractional-float", 3.5},
		{"fractional-json-number", json.Number("3.5")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Canonicalize(tc.input)
			if !errors.Is(err, ErrNonIntegerNumber) {
				t.Errorf("want ErrNonIntegerNumber, got %v", err)
			}
		})
	}
}

func TestCanonicalIntegralNumbersAccepted(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  string
	}{
		{"integral-float64", float64(3), "3\n"},
		{"integral-json-number", json.Number("3"), "3\n"},
		{"negative-int64", int64(-7), "-7\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGolden(t, tc.input, tc.want, hexOf(tc.want))
		})
	}
}

type unsupportedStruct struct {
	X int
}

func TestCanonicalUnsupportedTypeErrors(t *testing.T) {
	cases := []struct {
		name  string
		input any
	}{
		{"chan", make(chan int)},
		{"struct", unsupportedStruct{X: 1}},
		{"typed-map", map[string]int{"a": 1}},
		{"typed-slice", []int{1, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Canonicalize(tc.input)
			if !errors.Is(err, ErrUnsupportedType) {
				t.Errorf("want ErrUnsupportedType, got %v", err)
			}
		})
	}
}

func TestCanonicalTrailingNewlineEnforced(t *testing.T) {
	v := map[string]any{"a": int64(1)}
	got, err := Canonicalize(v)
	if err != nil {
		t.Fatalf("Canonicalize error: %v", err)
	}
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Fatalf("canonical bytes must end with LF: %q", string(got))
	}
	if len(got) >= 2 && got[len(got)-2] == '\n' {
		t.Errorf("canonical bytes must end with exactly one LF, not two: %q", string(got))
	}
	// sha256 of canonical WITHOUT the trailing LF must differ from Hash(v).
	withoutLF := hexOf(string(got[:len(got)-1]))
	full, err := Hash(v)
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}
	if withoutLF == full {
		t.Errorf("trailing LF not included in hash domain")
	}
}

func TestCanonicalEmptyVsAbsentDistinct(t *testing.T) {
	hEmptyArr, err := Hash(map[string]any{"a": []any{}})
	if err != nil {
		t.Fatalf("Hash empty-array error: %v", err)
	}
	hAbsent, err := Hash(map[string]any{})
	if err != nil {
		t.Fatalf("Hash absent error: %v", err)
	}
	hNil, err := Hash(map[string]any{"a": nil})
	if err != nil {
		t.Fatalf("Hash nil error: %v", err)
	}
	if hEmptyArr == hAbsent {
		t.Errorf("{\"a\":[]} must differ from {}")
	}
	if hNil == hAbsent {
		t.Errorf("{\"a\":null} must differ from {}")
	}
	if hEmptyArr == hNil {
		t.Errorf("{\"a\":[]} must differ from {\"a\":null}")
	}
}

func TestCanonicalGoldenVectors(t *testing.T) {
	t.Run("evidence-event-like", func(t *testing.T) {
		in := map[string]any{
			"event":    "gate_passed",
			"item":     "106.001-T",
			"ran":      true,
			"children": []any{"x", "y"},
		}
		wantCanonical := "{\"children\":[\"x\",\"y\"],\"event\":\"gate_passed\",\"item\":\"106.001-T\",\"ran\":true}\n"
		// Hardcoded pin observed on first green run (independent of canonicalization).
		wantHash := "c1af990367c7a6186e6a4bb427440d2f1091bf3da9350ddfebc300f2e1c21156"
		assertGolden(t, in, wantCanonical, wantHash)
	})

	t.Run("second-generic-nested", func(t *testing.T) {
		in := map[string]any{
			"list": []any{
				map[string]any{"k": int64(1)},
				map[string]any{"k": int64(2)},
			},
			"meta": map[string]any{
				"tags":    []any{"alpha", "beta"},
				"version": int64(2),
			},
			"nested": map[string]any{
				"deep": map[string]any{
					"count": int64(0),
					"flag":  false,
					"note":  "line1\nline2",
				},
			},
		}
		wantCanonical := "{\"list\":[{\"k\":1},{\"k\":2}],\"meta\":{\"tags\":[\"alpha\",\"beta\"],\"version\":2},\"nested\":{\"deep\":{\"count\":0,\"flag\":false,\"note\":\"line1\\nline2\"}}}\n"
		// Hardcoded pin observed on first green run (independent of canonicalization).
		wantHash := "c92bc0309b8fdb79bdfd6127b23ac4082885d366390c3338d2a833de7f7260a6"
		assertGolden(t, in, wantCanonical, wantHash)
	})
}

func TestCanonicalOneSeam(t *testing.T) {
	// Evidence-event, formal-report-like, and a generic payload must all hash
	// through the same canonical.Hash seam.
	payloads := map[string]any{
		"evidence_event": map[string]any{
			"event": "gate_passed",
			"item":  "106.001-T",
		},
		"formal_report": map[string]any{
			"report":  "formal",
			"checks":  []any{"a", "b"},
			"passed":  true,
			"summary": "ok",
		},
		"generic": map[string]any{
			"x": int64(1),
			"y": []any{"z"},
		},
	}
	for name, p := range payloads {
		t.Run(name, func(t *testing.T) {
			h, err := Hash(p)
			if err != nil {
				t.Fatalf("Hash error: %v", err)
			}
			if len(h) != 64 {
				t.Errorf("expected 64-char hex hash, got %d chars: %q", len(h), h)
			}
			if _, err := hex.DecodeString(h); err != nil {
				t.Errorf("hash is not valid hex: %v", err)
			}
		})
	}
}

func TestCanonicalTimestampZVsOffsetDiffer(t *testing.T) {
	// Demonstrates why callers MUST emit UTC "Z": equivalent instants written
	// with "Z" vs "+00:00" are distinct strings and therefore distinct hashes.
	hZ, err := Hash(map[string]any{"ts": "2026-07-31T01:00:00Z"})
	if err != nil {
		t.Fatalf("Hash Z error: %v", err)
	}
	hOffset, err := Hash(map[string]any{"ts": "2026-07-31T01:00:00+00:00"})
	if err != nil {
		t.Fatalf("Hash offset error: %v", err)
	}
	if hZ == hOffset {
		t.Errorf("Z and +00:00 timestamps must hash differently")
	}
}

func TestCanonicalControlCharEscaping(t *testing.T) {
	// Control chars below 0x20 (other than the named escapes) render as \u00xx
	// with lowercase hex; "/" is NOT escaped; non-ASCII runes are emitted
	// literally as UTF-8.
	in := map[string]any{"s": "a\x01b/\u00e9"}
	want := "{\"s\":\"a\\u0001b/\u00e9\"}\n"
	got, err := Canonicalize(in)
	if err != nil {
		t.Fatalf("Canonicalize error: %v", err)
	}
	if string(got) != want {
		t.Errorf("control/utf8 escaping mismatch:\n got=%q\nwant=%q", string(got), want)
	}
	if strings.Contains(string(got), "\\/") {
		t.Errorf("solidus must not be escaped")
	}
}
