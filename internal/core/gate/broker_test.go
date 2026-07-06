package gate

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

type fakeRunner struct {
	res     GateResult
	err     error
	gotArgs []string
}

func (f *fakeRunner) Run(_ context.Context, args []string, _ string, _ []string) (GateResult, error) {
	f.gotArgs = args
	return f.res, f.err
}

// blockingVersion blocks until the context is cancelled, modeling a wedged or
// malicious `autoharness version` probe. It returns the context error so the
// broker's timeout classification can be exercised.
type blockingVersion struct{}

func (blockingVersion) Version(ctx context.Context) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func allRefs() map[string]bool {
	return map[string]bool{"HEAD": true, "origin/HEAD": true, "origin/main": true, "main": true}
}

func TestBroker_Evaluate(t *testing.T) {
	t.Run("disabled proceeds without running", func(t *testing.T) {
		r := &fakeRunner{}
		b := &Broker{Runner: r, Git: fakeGit{resolvable: allRefs()}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledFalse}
		ev, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if err != nil {
			t.Fatal(err)
		}
		if ev.Ran || ev.Decision.Kind != DecisionProceed {
			t.Fatalf("expected no-run proceed, got %+v", ev)
		}
		if r.gotArgs != nil {
			t.Fatal("runner should not have been called")
		}
	})

	t.Run("auto binary-not-found fails open without running gate", func(t *testing.T) {
		r := &fakeRunner{}
		b := &Broker{Runner: r, Git: fakeGit{resolvable: allRefs()}, Version: fakeVersion{err: bkerrors.ErrGateBinaryNotFound}, Enabled: EnabledAuto}
		ev, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if err != nil {
			t.Fatal(err)
		}
		if ev.Decision.Kind != DecisionProceed || ev.Ran {
			t.Fatalf("expected fail-open proceed, got %+v", ev)
		}
	})

	t.Run("true binary-not-found is setup error", func(t *testing.T) {
		b := &Broker{Runner: &fakeRunner{}, Git: fakeGit{resolvable: allRefs()}, Version: fakeVersion{err: bkerrors.ErrGateBinaryNotFound}, Enabled: EnabledTrue}
		_, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if !stderrors.Is(err, bkerrors.ErrGateSetup) {
			t.Fatalf("err = %v, want setup", err)
		}
	})

	t.Run("enforced pass proceeds and ran", func(t *testing.T) {
		r := &fakeRunner{res: GateResult{ExitCode: 0}}
		b := &Broker{Runner: r, Git: fakeGit{resolvable: allRefs()}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledTrue}
		ev, err := b.Evaluate(context.Background(), Request{ItemID: "082.002-T", WorkspaceRoot: "/repo"})
		if err != nil {
			t.Fatal(err)
		}
		if ev.Decision.Kind != DecisionProceed || !ev.Ran || !ev.Enforced {
			t.Fatalf("unexpected: %+v", ev)
		}
		// argv carries the resolved base + pinned HEAD + task.
		if !containsArg(r.gotArgs, "--base") || !containsArg(r.gotArgs, "origin/HEAD") || !containsArg(r.gotArgs, "082.002-T") {
			t.Fatalf("argv missing expected fields: %v", r.gotArgs)
		}
	})

	t.Run("enforced block below threshold", func(t *testing.T) {
		r := &fakeRunner{res: GateResult{ExitCode: 1, Stdout: []byte(`{"repeated_failure":{"count":1,"threshold":3,"reached":false,"action":"block"}}`)}}
		b := &Broker{Runner: r, Git: fakeGit{resolvable: allRefs()}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledTrue}
		ev, _ := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if ev.Decision.Kind != DecisionBlock {
			t.Fatalf("expected block, got %v", ev.Decision.Kind)
		}
	})

	t.Run("enforced reached+block redirects to queued", func(t *testing.T) {
		r := &fakeRunner{res: GateResult{ExitCode: 1, Stdout: []byte(`{"repeated_failure":{"count":3,"threshold":3,"reached":true,"action":"block"}}`)}}
		b := &Broker{Runner: r, Git: fakeGit{resolvable: allRefs()}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledTrue}
		ev, _ := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if ev.Decision.Kind != DecisionRedirectQueued {
			t.Fatalf("expected redirect_queued, got %v", ev.Decision.Kind)
		}
	})

	t.Run("unverifiable base ref is config error", func(t *testing.T) {
		b := &Broker{Runner: &fakeRunner{}, Git: fakeGit{resolvable: map[string]bool{"HEAD": true}}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledTrue, ConfigBaseRef: "nope"}
		_, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if !stderrors.Is(err, bkerrors.ErrGateConfig) {
			t.Fatalf("err = %v, want config", err)
		}
	})

	t.Run("auto-discovery base failure fails open under auto", func(t *testing.T) {
		// HEAD resolves but no default-branch candidate does (e.g., a repo with no
		// origin and no main). With no explicit override, auto must fail open.
		b := &Broker{Runner: &fakeRunner{}, Git: fakeGit{resolvable: map[string]bool{"HEAD": true}}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledAuto}
		ev, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if err != nil {
			t.Fatalf("auto fail-open err = %v, want nil", err)
		}
		if ev.Decision.Kind != DecisionProceed {
			t.Fatalf("decision = %v, want proceed", ev.Decision.Kind)
		}
		if ev.Enforced {
			t.Fatalf("Enforced = true, want false on a fail-open")
		}
	})

	t.Run("auto-discovery base failure fails closed under true", func(t *testing.T) {
		b := &Broker{Runner: &fakeRunner{}, Git: fakeGit{resolvable: map[string]bool{"HEAD": true}}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledTrue}
		_, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
		if !stderrors.Is(err, bkerrors.ErrGateConfig) {
			t.Fatalf("err = %v, want config error under true", err)
		}
	})

	t.Run("explicit gate-base failure is config error even under auto", func(t *testing.T) {
		// An explicit --gate-base that does not resolve must NOT fail open, even
		// under auto — a mistyped privileged override must surface.
		b := &Broker{Runner: &fakeRunner{}, Git: fakeGit{resolvable: map[string]bool{"HEAD": true}}, Version: fakeVersion{ver: "1.4.7"}, Enabled: EnabledAuto}
		_, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: ".", GateBase: "no-such-ref"})
		if !stderrors.Is(err, bkerrors.ErrGateConfig) {
			t.Fatalf("err = %v, want config error for explicit override", err)
		}
	})

	t.Run("version probe is bounded by the timeout (no unbounded hang)", func(t *testing.T) {
		// Regression: the timeout must cover the version probe (and base
		// resolution), not just the gate run. The completion path holds the task
		// lock across Evaluate, so an unbounded probe would pin the lock forever.
		b := &Broker{Runner: &fakeRunner{}, Git: fakeGit{resolvable: allRefs()}, Version: blockingVersion{}, Enabled: EnabledTrue, TimeoutSeconds: 1}
		done := make(chan error, 1)
		go func() {
			_, err := b.Evaluate(context.Background(), Request{ItemID: "x", WorkspaceRoot: "."})
			done <- err
		}()
		select {
		case err := <-done:
			// Returned rather than hanging: the internal deadline cancelled the
			// probe. Under enabled:true a probe failure is a setup-class refusal.
			if !stderrors.Is(err, bkerrors.ErrGateSetup) {
				t.Fatalf("err = %v, want setup after probe timeout", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Evaluate hung: version probe was not bounded by TimeoutSeconds")
		}
	})
}
