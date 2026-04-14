package hooks

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// TestRegisterAndFirePre verifies that a registered pre-hook is invoked.
func TestRegisterAndFirePre(t *testing.T) {
	r := NewHookRunner()
	called := false
	r.Register(HookCreateArtifact, PhasePre, HookRegistration{
		Name:     "test-pre",
		Priority: 100,
		Fn: func(ctx context.Context, hc HookContext) error {
			called = true
			return nil
		},
	})

	if err := r.FirePre(context.Background(), HookCreateArtifact, HookContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected pre-hook to be called")
	}
}

// TestFirePrePriorityOrder verifies that pre-hooks fire in ascending priority order.
func TestFirePrePriorityOrder(t *testing.T) {
	r := NewHookRunner()
	var order []int

	for _, p := range []int{300, 100, 200} {
		priority := p
		r.Register(HookUpdateArtifact, PhasePre, HookRegistration{
			Name:     fmt.Sprintf("hook-%d", priority),
			Priority: priority,
			Fn: func(ctx context.Context, hc HookContext) error {
				order = append(order, priority)
				return nil
			},
		})
	}

	if err := r.FirePre(context.Background(), HookUpdateArtifact, HookContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 hooks to fire, got %d", len(order))
	}
	if order[0] != 100 || order[1] != 200 || order[2] != 300 {
		t.Errorf("expected order [100 200 300], got %v", order)
	}
}

// TestFirePreErrorStopsExecution verifies that an error from a pre-hook prevents
// subsequent hooks from running.
func TestFirePreErrorStopsExecution(t *testing.T) {
	r := NewHookRunner()
	secondCalled := false
	sentinel := errors.New("pre-hook failure")

	r.Register(HookArchiveItem, PhasePre, HookRegistration{
		Name:     "fail-first",
		Priority: 10,
		Fn: func(ctx context.Context, hc HookContext) error {
			return sentinel
		},
	})
	r.Register(HookArchiveItem, PhasePre, HookRegistration{
		Name:     "should-not-run",
		Priority: 20,
		Fn: func(ctx context.Context, hc HookContext) error {
			secondCalled = true
			return nil
		},
	})

	err := r.FirePre(context.Background(), HookArchiveItem, HookContext{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error in chain, got: %v", err)
	}
	if secondCalled {
		t.Error("second hook should not have been called after first hook errored")
	}
}

// TestFirePostSwallowsErrors verifies that a post-hook error is not returned to
// the caller.
func TestFirePostSwallowsErrors(t *testing.T) {
	r := NewHookRunner()
	r.Register(HookShipShipment, PhasePost, HookRegistration{
		Name:     "error-post",
		Priority: 100,
		Fn: func(ctx context.Context, hc HookContext) error {
			return errors.New("post-hook failure")
		},
	})

	// FirePost must not panic and must not propagate the error.
	r.FirePost(context.Background(), HookShipShipment, HookContext{})
}

// TestFirePostPriorityOrder verifies that post-hooks fire in ascending priority order.
func TestFirePostPriorityOrder(t *testing.T) {
	r := NewHookRunner()
	var order []int

	for _, p := range []int{30, 10, 20} {
		priority := p
		r.Register(HookAdoptItem, PhasePost, HookRegistration{
			Name:     fmt.Sprintf("post-%d", priority),
			Priority: priority,
			Fn: func(ctx context.Context, hc HookContext) error {
				order = append(order, priority)
				return nil
			},
		})
	}

	r.FirePost(context.Background(), HookAdoptItem, HookContext{})
	if len(order) != 3 {
		t.Fatalf("expected 3 post-hooks to fire, got %d", len(order))
	}
	if order[0] != 10 || order[1] != 20 || order[2] != 30 {
		t.Errorf("expected order [10 20 30], got %v", order)
	}
}

// TestEmptyHookList verifies that firing with no registered hooks is a no-op.
func TestEmptyHookList(t *testing.T) {
	r := NewHookRunner()

	if err := r.FirePre(context.Background(), HookMoveShipmentStatus, HookContext{}); err != nil {
		t.Errorf("unexpected error from empty pre list: %v", err)
	}
	r.FirePost(context.Background(), HookMoveShipmentStatus, HookContext{})
}

// TestConcurrentRegistrationAndFiring exercises Register and FirePre/FirePost
// concurrently to detect data races (run with -race).
func TestConcurrentRegistrationAndFiring(t *testing.T) {
	r := NewHookRunner()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			r.Register(HookCreateArtifact, PhasePre, HookRegistration{
				Name:     fmt.Sprintf("concurrent-%d", idx),
				Priority: idx,
				Fn: func(ctx context.Context, hc HookContext) error {
					return nil
				},
			})
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.FirePre(context.Background(), HookCreateArtifact, HookContext{})
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.FirePost(context.Background(), HookCreateArtifact, HookContext{})
		}()
	}

	wg.Wait()
}

// TestSnapshotBeforeExecute verifies that a hook which registers a new hook
// during execution does not deadlock, because firing uses a snapshot taken
// before releasing the lock.
func TestSnapshotBeforeExecute(t *testing.T) {
	r := NewHookRunner()
	registered := false

	r.Register(HookUpdateArtifact, PhasePre, HookRegistration{
		Name:     "register-during-fire",
		Priority: 100,
		Fn: func(ctx context.Context, hc HookContext) error {
			// Registering while a fire is in progress must not deadlock.
			r.Register(HookUpdateArtifact, PhasePre, HookRegistration{
				Name:     "registered-during-fire",
				Priority: 200,
				Fn: func(ctx context.Context, hc HookContext) error {
					return nil
				},
			})
			registered = true
			return nil
		},
	})

	if err := r.FirePre(context.Background(), HookUpdateArtifact, HookContext{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !registered {
		t.Fatal("first hook should have run and registered the second hook")
	}
}

// TestHookPointConstants verifies that all six HookPoint constants have the
// expected string values.
func TestHookPointConstants(t *testing.T) {
	cases := []struct {
		got  HookPoint
		want string
	}{
		{HookCreateArtifact, "create_artifact"},
		{HookUpdateArtifact, "update_artifact"},
		{HookArchiveItem, "archive_item"},
		{HookShipShipment, "ship_shipment"},
		{HookAdoptItem, "adopt_item"},
		{HookMoveShipmentStatus, "move_shipment_status"},
	}
	for _, tc := range cases {
		if string(tc.got) != tc.want {
			t.Errorf("HookPoint constant: got %q, want %q", tc.got, tc.want)
		}
	}
}
