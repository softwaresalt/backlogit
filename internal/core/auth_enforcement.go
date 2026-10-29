package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AuthEnforcement is the enforcement mode for a live reconcile invocation.
type AuthEnforcement string

const (
	// AuthEnforcementAuthenticated requires a verified authorization token.
	AuthEnforcementAuthenticated AuthEnforcement = "authenticated"
	// AuthEnforcementConfirmation is the v1 self-suppliable --confirm/TTY guard.
	AuthEnforcementConfirmation AuthEnforcement = "confirmation"
)

// Sentinel errors for required-auth enforcement (170.009-T).
var (
	// ErrAuthPolicyDenied reports an invocation denied fail-closed because
	// required-auth is externally enabled but cannot be satisfied.
	ErrAuthPolicyDenied = errors.New("authenticated authorization required: invocation denied (fail closed)")
	// ErrAuthPolicyMissing reports an absent workspace auth policy.
	ErrAuthPolicyMissing = errors.New("workspace auth policy missing")
	// ErrAuthPolicyMalformed reports an unparseable workspace auth policy.
	ErrAuthPolicyMalformed = errors.New("workspace auth policy malformed")
	// ErrRequiredAuthUnassertable reports that the external required-auth
	// state could not be determined; it is never treated as "not required".
	ErrRequiredAuthUnassertable = errors.New("external required-auth state unassertable")
)

// RequiredAuthProvider asserts, from outside the workspace (the external pin
// root of trust or an externally protected flag), whether authenticated
// authorization is required.
type RequiredAuthProvider interface {
	// RequiredAuthEnabled reports the external required-auth state.
	RequiredAuthEnabled(ctx context.Context) (bool, error)
}

type workspaceAuthPolicy struct {
	Required *bool `yaml:"required"`
}

// loadWorkspaceAuthPolicy reads the in-workspace auth policy at path. A
// missing file wraps ErrAuthPolicyMissing; anything unparseable wraps
// ErrAuthPolicyMalformed.
func loadWorkspaceAuthPolicy(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, ErrAuthPolicyMissing
	}
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrAuthPolicyMalformed, err)
	}
	var pol workspaceAuthPolicy
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&pol); err != nil || pol.Required == nil {
		if err == nil {
			err = errors.New("required member absent")
		}
		return false, fmt.Errorf("%w: %w", ErrAuthPolicyMalformed, err)
	}
	return *pol.Required, nil
}

// ResolveAuthEnforcement decides the enforcement mode. The external provider
// is authoritative: when it asserts required-auth, a missing or malformed
// workspace policy denies (fail closed) and a workspace policy can never
// downgrade enforcement. When it asserts not-required the workspace policy
// may still opt in; a missing or malformed policy then keeps the v1
// confirmation guard. A nil or failing provider is unassertable and denies.
func ResolveAuthEnforcement(ctx context.Context, provider RequiredAuthProvider, policyPath string) (AuthEnforcement, error) {
	if provider == nil {
		return "", fmt.Errorf("%w: %w: no provider", ErrAuthPolicyDenied, ErrRequiredAuthUnassertable)
	}
	external, err := provider.RequiredAuthEnabled(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %w: %w", ErrAuthPolicyDenied, ErrRequiredAuthUnassertable, err)
	}
	workspaceRequired, perr := loadWorkspaceAuthPolicy(policyPath)
	if external {
		if perr != nil {
			return "", fmt.Errorf("%w: %w", ErrAuthPolicyDenied, perr)
		}
		return AuthEnforcementAuthenticated, nil
	}
	if perr == nil && workspaceRequired {
		return AuthEnforcementAuthenticated, nil
	}
	return AuthEnforcementConfirmation, nil
}
