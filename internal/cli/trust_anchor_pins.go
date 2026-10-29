package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/core"
)

// Environment variables that provision the external trust pin set. Both are
// operator-owned and read from outside the workspace config.
const (
	// envTrustPins holds comma/whitespace separated pin entries. An entry is
	// a bare fingerprint (pinned but unbound; resolution fails closed) or
	// <fingerprint>=<role>[;not_before=<RFC3339>][;not_after=<RFC3339>], which
	// binds the externally authoritative role and validity window (168.008-T).
	envTrustPins = "BACKLOGIT_TRUST_PINS"
	// envTrustPinsFile names an operator-owned file with one pin entry per
	// line ('#' starts a comment).
	envTrustPinsFile = "BACKLOGIT_TRUST_PINS_FILE"
)

// envPinProvider is the CLI's core.ExternalPinSetProvider. It reports
// core.ErrTrustPinSetAbsent when neither variable provisions any pin.
type envPinProvider struct{}

// ExternalPinSet implements core.ExternalPinSetProvider.
func (envPinProvider) ExternalPinSet(context.Context) (*core.ExternalPinSet, error) {
	var fps []string
	if v := os.Getenv(envTrustPins); v != "" {
		fps = append(fps, splitPins(v)...)
	}
	if path := os.Getenv(envTrustPinsFile); path != "" {
		data, err := os.ReadFile(path) //nolint:gosec // operator-provided pin file path outside the workspace
		if err != nil {
			return nil, fmt.Errorf("%w: read %s: %w", core.ErrTrustPinSetAbsent, envTrustPinsFile, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if i := strings.Index(line, "#"); i >= 0 {
				line = line[:i]
			}
			fps = append(fps, splitPins(line)...)
		}
	}
	if len(fps) == 0 {
		return nil, core.ErrTrustPinSetAbsent
	}
	return buildPinSet(fps)
}

// buildPinSet parses pin entries into a pin set. A malformed entry fails
// closed with core.ErrTrustPinSetAbsent.
func buildPinSet(entries []string) (*core.ExternalPinSet, error) {
	var bound []core.PinBinding
	var bare []string
	for _, e := range entries {
		fp, spec, hasSpec := strings.Cut(e, "=")
		if !hasSpec {
			bare = append(bare, fp)
			continue
		}
		parts := strings.Split(spec, ";")
		b := core.PinBinding{Fingerprint: fp, Role: parts[0]}
		if fp == "" || b.Role == "" {
			return nil, fmt.Errorf("%w: pin entry %q needs a fingerprint and role", core.ErrTrustPinSetAbsent, e)
		}
		for _, kv := range parts[1:] {
			k, v, _ := strings.Cut(kv, "=")
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return nil, fmt.Errorf("%w: pin entry %q: %s must be RFC3339: %w", core.ErrTrustPinSetAbsent, e, k, err)
			}
			switch k {
			case "not_before":
				b.NotBefore = &t
			case "not_after":
				b.NotAfter = &t
			default:
				return nil, fmt.Errorf("%w: pin entry %q: unknown field %q", core.ErrTrustPinSetAbsent, e, k)
			}
		}
		bound = append(bound, b)
	}
	set := core.NewBoundExternalPinSet(bound...)
	if len(bare) == 0 {
		return set, nil
	}
	// Merge unbound pins: they authorize the fingerprint for pin-gate checks
	// only; the resolver denies them (role/validity unverifiable).
	return core.MergePinSets(set, core.NewExternalPinSet(bare...)), nil
}

func splitPins(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\r' || r == '\n' })
}

// loadPins returns the external pin set, or nil (fail closed) when absent.
func loadPins(ctx context.Context) *core.ExternalPinSet {
	pins, err := envPinProvider{}.ExternalPinSet(ctx)
	if err != nil {
		return nil
	}
	return pins
}
