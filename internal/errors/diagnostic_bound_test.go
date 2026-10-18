package errors

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundDiagnostic(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		wantUnchanged bool
		wantTruncated bool
	}{
		{"empty", "", true, false},
		{"short", "hello", true, false},
		{"exactly at budget", strings.Repeat("a", DiagnosticBudget), true, false},
		{"one over budget", strings.Repeat("a", DiagnosticBudget+1), false, true},
		{"far over budget", strings.Repeat("b", DiagnosticBudget*10), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BoundDiagnostic(tt.in)
			if tt.wantUnchanged && got != tt.in {
				t.Errorf("expected unchanged input, got len %d", len(got))
			}
			if len(got) > DiagnosticBudget {
				t.Errorf("len %d exceeds budget %d", len(got), DiagnosticBudget)
			}
			if has := strings.HasSuffix(got, TruncationMarker); has != tt.wantTruncated {
				t.Errorf("marker present = %v, want %v", has, tt.wantTruncated)
			}
			if !utf8.ValidString(got) {
				t.Errorf("result is not valid UTF-8")
			}
		})
	}
}

func TestBoundDiagnosticMultiByteStraddle(t *testing.T) {
	runes := []struct {
		name string
		r    string
	}{
		{"2-byte", "é"},
		{"3-byte", "世"},
		{"4-byte", "😀"},
	}
	cutPoint := DiagnosticBudget - len(TruncationMarker)
	for _, rr := range runes {
		width := len(rr.r)
		for offset := 0; offset < width+1; offset++ {
			t.Run(rr.name, func(t *testing.T) {
				start := cutPoint - offset
				in := strings.Repeat("a", start) + strings.Repeat(rr.r, 200)
				if !utf8.ValidString(in) {
					t.Fatalf("test input invalid")
				}
				got := BoundDiagnostic(in)
				if !utf8.ValidString(got) {
					t.Errorf("offset %d: invalid UTF-8 result", offset)
				}
				if len(got) > DiagnosticBudget {
					t.Errorf("offset %d: len %d exceeds budget", offset, len(got))
				}
				if !strings.HasSuffix(got, TruncationMarker) {
					t.Errorf("offset %d: missing marker", offset)
				}
			})
		}
	}
}
