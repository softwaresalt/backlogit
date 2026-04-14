package hooks

import (
	"context"
	"errors"
	"testing"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestValidateStatusTransition_ValidTransition(t *testing.T) {
	hook := ValidateStatusTransition(nil)
	hc := HookContext{
		OldValues: map[string]any{"status": "queued"},
		NewValues: map[string]any{"status": "active"},
	}
	if err := hook(context.Background(), hc); err != nil {
		t.Fatalf("expected valid transition to succeed, got: %v", err)
	}
}

func TestValidateStatusTransition_InvalidTransition(t *testing.T) {
	hook := ValidateStatusTransition(nil)
	hc := HookContext{
		OldValues: map[string]any{"status": "queued"},
		NewValues: map[string]any{"status": "done"},
	}
	err := hook(context.Background(), hc)
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
	if !errors.Is(err, blerrors.ErrInvalidStatusTransition) {
		t.Fatalf("expected ErrInvalidStatusTransition, got: %v", err)
	}
}

func TestValidateStatusTransition_NoStatusChange(t *testing.T) {
	hook := ValidateStatusTransition(nil)
	hc := HookContext{
		OldValues: map[string]any{"status": "active"},
		NewValues: map[string]any{"title": "new title"},
	}
	if err := hook(context.Background(), hc); err != nil {
		t.Fatalf("expected no-op when no status in NewValues, got: %v", err)
	}
}

func TestValidateStatusTransition_SameStatus(t *testing.T) {
	hook := ValidateStatusTransition(nil)
	hc := HookContext{
		OldValues: map[string]any{"status": "active"},
		NewValues: map[string]any{"status": "active"},
	}
	if err := hook(context.Background(), hc); err != nil {
		t.Fatalf("expected no-op when status unchanged, got: %v", err)
	}
}

func TestValidateStatusTransition_CustomTransitions(t *testing.T) {
	custom := map[string][]string{
		"open":   {"closed"},
		"closed": {},
	}
	hook := ValidateStatusTransition(custom)

	// Valid custom transition
	hc := HookContext{
		OldValues: map[string]any{"status": "open"},
		NewValues: map[string]any{"status": "closed"},
	}
	if err := hook(context.Background(), hc); err != nil {
		t.Fatalf("expected valid custom transition, got: %v", err)
	}

	// Invalid custom transition (no transitions from closed)
	hc2 := HookContext{
		OldValues: map[string]any{"status": "closed"},
		NewValues: map[string]any{"status": "open"},
	}
	err := hook(context.Background(), hc2)
	if err == nil {
		t.Fatal("expected error for transition from closed")
	}
	if !errors.Is(err, blerrors.ErrInvalidStatusTransition) {
		t.Fatalf("expected ErrInvalidStatusTransition, got: %v", err)
	}
}

func TestValidateStatusTransition_MissingOldStatus(t *testing.T) {
	hook := ValidateStatusTransition(nil)
	hc := HookContext{
		OldValues: map[string]any{},
		NewValues: map[string]any{"status": "active"},
	}
	if err := hook(context.Background(), hc); err != nil {
		t.Fatalf("expected no-op when old status is empty, got: %v", err)
	}
}

func TestValidateStatusTransition_AllDefaultTransitions(t *testing.T) {
	hook := ValidateStatusTransition(nil)
	validTransitions := []struct {
		from string
		to   string
	}{
		{"queued", "active"},
		{"queued", "blocked"},
		{"active", "done"},
		{"active", "blocked"},
		{"active", "review"},
		{"blocked", "active"},
		{"review", "done"},
		{"review", "accepted"},
		{"review", "rejected"},
		{"done", "archived"},
	}
	for _, tt := range validTransitions {
		hc := HookContext{
			OldValues: map[string]any{"status": tt.from},
			NewValues: map[string]any{"status": tt.to},
		}
		if err := hook(context.Background(), hc); err != nil {
			t.Errorf("expected valid transition %s->%s, got: %v", tt.from, tt.to, err)
		}
	}
}
