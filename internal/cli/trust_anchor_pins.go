package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/softwaresalt/backlogit/internal/core"
)

// Environment variables that provision the external trust pin set. Both are
// operator-owned and read from outside the workspace config.
const (
	// envTrustPins holds comma/whitespace separated anchor fingerprints.
	envTrustPins = "BACKLOGIT_TRUST_PINS"
	// envTrustPinsFile names an operator-owned file with one fingerprint per
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
	return core.NewExternalPinSet(fps...), nil
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
