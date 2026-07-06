package gate

import (
	"context"
	stderrors "errors"
	"testing"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// fakeGit resolves only the refs in its set (plus HEAD unless withheld).
type fakeGit struct {
	resolvable map[string]bool
	err        error
}

func (f fakeGit) Verify(_ context.Context, ref string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.resolvable[ref], nil
}

func TestResolveBaseRef(t *testing.T) {
	tests := []struct {
		name       string
		in         BaseRefInput
		resolvable map[string]bool
		wantRef    string
		wantNonDef bool
		wantErr    bool
	}{
		{
			name:       "auto picks origin/HEAD first",
			resolvable: map[string]bool{"HEAD": true, "origin/HEAD": true, "origin/main": true, "main": true},
			wantRef:    "origin/HEAD",
		},
		{
			name:       "auto falls back to origin/main",
			resolvable: map[string]bool{"HEAD": true, "origin/main": true, "main": true},
			wantRef:    "origin/main",
		},
		{
			name:       "auto falls back to main",
			resolvable: map[string]bool{"HEAD": true, "main": true},
			wantRef:    "main",
		},
		{
			name:       "explicit config base_ref used and verified",
			in:         BaseRefInput{ConfigBaseRef: "release/1.x"},
			resolvable: map[string]bool{"HEAD": true, "release/1.x": true, "origin/HEAD": true},
			wantRef:    "release/1.x",
			wantNonDef: true,
		},
		{
			name:       "explicit base equal to default is not non-default",
			in:         BaseRefInput{ConfigBaseRef: "origin/HEAD"},
			resolvable: map[string]bool{"HEAD": true, "origin/HEAD": true},
			wantRef:    "origin/HEAD",
			wantNonDef: false,
		},
		{
			name:       "gate_base override used when config is auto",
			in:         BaseRefInput{ConfigBaseRef: "auto", GateBase: "feature-base"},
			resolvable: map[string]bool{"HEAD": true, "feature-base": true, "origin/HEAD": true},
			wantRef:    "feature-base",
			wantNonDef: true,
		},
		{
			name:       "invalid explicit base_ref -> config error",
			in:         BaseRefInput{ConfigBaseRef: "nope"},
			resolvable: map[string]bool{"HEAD": true, "origin/HEAD": true},
			wantErr:    true,
		},
		{
			name:       "invalid explicit gate_base -> config error",
			in:         BaseRefInput{GateBase: "nope"},
			resolvable: map[string]bool{"HEAD": true, "origin/HEAD": true},
			wantErr:    true,
		},
		{
			name:       "unresolvable default while auto -> config error",
			resolvable: map[string]bool{"HEAD": true},
			wantErr:    true,
		},
		{
			name:       "HEAD does not resolve -> config error",
			resolvable: map[string]bool{"origin/HEAD": true},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveBaseRef(context.Background(), fakeGit{resolvable: tt.resolvable}, tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				if !stderrors.Is(err, bkerrors.ErrGateConfig) {
					t.Fatalf("err = %v, want ErrGateConfig", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Ref != tt.wantRef {
				t.Fatalf("ref = %q, want %q", got.Ref, tt.wantRef)
			}
			if got.NonDefault != tt.wantNonDef {
				t.Fatalf("nonDefault = %v, want %v", got.NonDefault, tt.wantNonDef)
			}
		})
	}
}
