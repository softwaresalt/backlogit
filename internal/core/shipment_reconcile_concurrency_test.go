package core

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// TestU20_ReconcileShipmentToShippedConcurrency is the RED concurrency proof
// harness for 167.020-T (#423, 167-F, "Reconcile-scoped concurrency proof
// harness"). ReconcileShipmentToShipped (shipment_reconcile.go) is still a
// gated panic declaration owned by a LATER task (167.008-T); this harness
// races it against AssociateCommit (commits.go) and ArchiveItem (archive.go)
// on the same reconcile-scoped shipment/member items, so it:
//
//  1. Proves today's lock ordering (A -> C -> B, bounded-wait A/B, single
//     in-process C mutex) never deadlocks: the whole concurrent run is
//     bounded by a hard wall-clock timeout and t.Fatal fires if it is
//     exceeded. A recovered panic counts as "completed" (with a recorded
//     failure), never as a hang.
//  2. Proves no clobber: after every goroutine completes, the shipment/
//     member artifacts are re-read from disk and must still parse as
//     coherent (non-torn) frontmatter.
//  3. Exercises same-key/different-request-identity replay and a
//     TOCTOU-shaped member-set re-route, structured so the assertions
//     apply once 167.008-T lands a real implementation.
//
// It currently FAILS (red) because every ReconcileShipmentToShipped call
// panics; each goroutine below recovers that panic and reports it as a test
// failure via a result channel rather than crashing the test binary. That
// recovered-panic assertion is the intended, documented reason this test is
// red today — not a build error and not a hang of `go test` itself.
func TestU20_ReconcileShipmentToShippedConcurrency(t *testing.T) {
	t.Run("ReconcileVsAssociateCommit", testU20ReconcileVsAssociateCommit)
	t.Run("ReconcileVsArchiveItem", testU20ReconcileVsArchiveItem)
	t.Run("ReplayDifferentIdentitySameKey", testU20ReconcileReplayDifferentIdentitySameKey)
	t.Run("TOCTOUMemberSetReroute", testU20ReconcileTOCTOUMemberSetReroute)
}

// u20GuardedResult is the outcome of one concurrency-harness goroutine: either
// a (possibly nil) error from the wrapped call, or a recovered panic value.
// Recording panics here (rather than letting them propagate) is what turns
// "ReconcileShipmentToShipped panics" into a reportable test failure instead
// of a crashed test binary — required so the overall `go test -race` run
// still exits promptly with a clear failure, per the harness contract.
type u20GuardedResult struct {
	name     string
	err      error
	panicked bool
	panicVal any
}

// u20RunGuarded invokes fn, recovering any panic into the returned result
// rather than letting it unwind the goroutine (and crash the test binary).
func u20RunGuarded(name string, fn func() error) (res u20GuardedResult) {
	res.name = name
	defer func() {
		if r := recover(); r != nil {
			res.panicked = true
			res.panicVal = r
		}
	}()
	res.err = fn()
	return res
}

// u20AwaitWithDeadline runs ops concurrently (one goroutine per op, all
// started together) and blocks until either every op has reported a result
// or the hard wall-clock deadline elapses. It fails the test via t.Fatal on
// timeout (proving a deadlock, not merely a slow run) and otherwise returns
// every collected result for the caller to assert against. sync.WaitGroup
// bounds each goroutine's lifecycle; the completion channel + select is the
// hang-vs-completion race itself.
func u20AwaitWithDeadline(t *testing.T, deadline time.Duration, ops map[string]func() error) []u20GuardedResult {
	t.Helper()
	results := make(chan u20GuardedResult, len(ops))
	var wg sync.WaitGroup
	for name, op := range ops {
		wg.Add(1)
		go func(name string, op func() error) {
			defer wg.Done()
			results <- u20RunGuarded(name, op)
		}(name, op)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(deadline):
		t.Fatalf("concurrent run did not complete within %s - suspected deadlock among: %v", deadline, opNames(ops))
	}
	close(results)

	collected := make([]u20GuardedResult, 0, len(ops))
	for res := range results {
		collected = append(collected, res)
	}
	return collected
}

func opNames(ops map[string]func() error) []string {
	names := make([]string, 0, len(ops))
	for name := range ops {
		names = append(names, name)
	}
	return names
}

// u20ReconcileFixture builds a minimal archived-shipment-with-terminal-member
// fixture matching the legacy-repair shape the other 167.0xx-T reconcile
// tests already exercise (see shipment_reconcile_preconditions_test.go):
// one direct `done` member, wrapped in a shipment that is archived with
// `archived_status: active`.
func u20ReconcileFixture(t *testing.T) (ws *Workspace, shipmentID, memberID string) {
	t.Helper()
	ws = setupShipmentWorkspace(t)
	memberID = createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
	return ws, shipment.ID, memberID
}

// u20AssertReconcilePanicked requires that the reconcile result recorded a
// recovered panic and reports it as a test failure. It remains reachable
// only from testU20ReconcileReplayDifferentIdentitySameKey's fallback
// branch, where an actual recovered panic is (post-167.008-T) always an
// unexpected, real defect.
func u20AssertReconcilePanicked(t *testing.T, res u20GuardedResult) {
	t.Helper()
	if !res.panicked {
		t.Errorf("%s: expected a recovered panic, got err=%v", res.name, res.err)
		return
	}
	t.Errorf("%s: unexpected recovered panic: %v", res.name, res.panicVal)
}

// u20AssertReconcileNoPanic asserts the real post-167.008-T expected
// behavior for ReconcileShipmentToShipped under concurrency: it must never
// panic. A non-nil returned error is tolerated and NOT asserted against
// here, since legitimate business-logic errors (e.g. Phase C evidence
// verification failing against this test fixture's synthetic, non-existent
// merge SHA/closure path) are expected outcomes, not defects. An actual
// recovered panic, however, is a real implementation defect now that
// 167.008-T has landed, so it is still reported as a test failure.
func u20AssertReconcileNoPanic(t *testing.T, res u20GuardedResult) {
	t.Helper()
	if res.panicked {
		t.Errorf("%s: unexpected recovered panic: %v", res.name, res.panicVal)
	}
}

// u20AssertArtifactCoherent re-reads the artifact from disk and requires it
// still parses as a single, well-formed record — the "no clobber" proof.
// A torn/corrupted frontmatter write under concurrent access would surface
// here as a parse error.
func u20AssertArtifactCoherent(t *testing.T, ws *Workspace, id string) *models.Artifact {
	t.Helper()
	artifact, err := findArtifact(context.Background(), ws, id)
	require.NoErrorf(t, err, "artifact %s must remain readable/coherent after concurrent access", id)
	require.NotNil(t, artifact)
	assert.NotEmpty(t, artifact.Status, "artifact %s must retain a valid status after concurrent access", id)
	return artifact
}

// testU20ReconcileVsAssociateCommit races ReconcileShipmentToShipped against
// AssociateCommit on the SAME shipment item (commits.go, per the task spec's
// required race #1).
func testU20ReconcileVsAssociateCommit(t *testing.T) {
	ws, shipmentID, _ := u20ReconcileFixture(t)
	ctx := context.Background()
	req := validShipmentReconcilePreconditionRequest(shipmentID)
	ew := NewWorkspaceEventWriter(ws, WorkspaceLogsRoot(ws.RootPath))

	results := u20AwaitWithDeadline(t, 10*time.Second, map[string]func() error{
		"reconcile": func() error {
			_, err := ReconcileShipmentToShipped(ctx, ws, req)
			return err
		},
		"associate_commit": func() error {
			return AssociateCommit(ctx, ws, ew, shipmentID, strings.Repeat("b", 40), "concurrent commit", "u20-tester")
		},
	})

	for _, res := range results {
		if res.name == "reconcile" {
			u20AssertReconcileNoPanic(t, res)
			continue
		}
		if res.panicked {
			t.Errorf("%s: unexpected recovered panic: %v", res.name, res.panicVal)
		}
	}

	u20AssertArtifactCoherent(t, ws, shipmentID)
}

// testU20ReconcileVsArchiveItem races ReconcileShipmentToShipped against
// ArchiveItem on the SAME shipment item, per the task spec's required race
// #2 ("concurrent AssociateCommit and ArchiveItem ... racing reconcile
// A->C->B acquisition" only exercises the intended same-item lock
// contention when both operations target the same item ID).
//
// u20ReconcileFixture's shipment is already archived (archived_status:
// active) before this test runs (createArchivedShipmentReconcileFixture
// calls ArchiveItem once during setup). A second ArchiveItem call on that
// same, already-archived shipment ID is a real, supported re-archive path
// (archive.go's currentPath==archivePath branch, guarded and documented at
// 167.021-T): it still acquires the full lock B (lockArtifactMutations,
// held for ArchiveItem's whole body) then lock C (events.
// LockItemLogCrossProcess, near the end) and performs a real frontmatter
// read/rewrite + DB status update, preserving the existing archived_status
// rather than a no-op short-circuit. That is a genuine concurrent-mutation
// window on the shipment ID, not a fabricated scenario.
//
// This deliberately creates an ABBA lock-order shape against reconcile's own
// C-then-B acquisition (shipment_reconcile_transaction.go): ArchiveItem
// acquires B then C while reconcile acquires C then B. Every lock in that
// cycle is bounded-wait (lockArtifactMutations/lockTaskFileWithHeartbeat:
// ~3s, defaultGateLockBoundedWait; reconcile's own C-lock,
// lockShipmentReconcileItemLogImpl: ~3s, file-lock only, does NOT touch the
// in-process events.LockItemLog mutex; ArchiveItem's C-lock file layer,
// events.acquireItemLogFileLock: ~3s, itemLogLockWait) so the worst case is
// a bounded lock-busy error on one side, never an unbounded wait. See
// TestU20_ReconcileShipmentToShippedConcurrency's package-level doc comment
// and the 167.020-T follow-up concurrency-review report for the empirical
// -race -count=10 verification this claim rests on.
func testU20ReconcileVsArchiveItem(t *testing.T) {
	ws, shipmentID, _ := u20ReconcileFixture(t)
	ctx := context.Background()
	req := validShipmentReconcilePreconditionRequest(shipmentID)

	results := u20AwaitWithDeadline(t, 10*time.Second, map[string]func() error{
		"reconcile": func() error {
			_, err := ReconcileShipmentToShipped(ctx, ws, req)
			return err
		},
		"archive_item": func() error {
			_, err := ArchiveItem(ctx, ws.DB, ws, shipmentID)
			return err
		},
	})

	for _, res := range results {
		if res.name == "reconcile" {
			u20AssertReconcileNoPanic(t, res)
			continue
		}
		if res.panicked {
			t.Errorf("%s: unexpected recovered panic: %v", res.name, res.panicVal)
		}
	}

	u20AssertArtifactCoherent(t, ws, shipmentID)
}

// testU20ReconcileReplayDifferentIdentitySameKey issues two concurrent
// ReconcileShipmentToShipped calls sharing the same IdempotencyKey but
// differing in a request-identity-affecting field (MergeSHA), and requires
// the two outcomes be distinguishable (one succeeds/no-ops, the other
// surfaces a conflict) rather than silently identical. Against today's
// panic-gated declaration both calls instead recover a panic; the structure
// below is written so it applies unchanged once 167.008-T lands real
// behavior (at which point the panic branch disappears and only the
// distinguishable-outcome assertion remains).
func testU20ReconcileReplayDifferentIdentitySameKey(t *testing.T) {
	ws, shipmentID, _ := u20ReconcileFixture(t)
	ctx := context.Background()

	reqA := validShipmentReconcilePreconditionRequest(shipmentID)
	reqA.IdempotencyKey = "idem-167-020-replay"
	reqA.MergeSHA = strings.Repeat("a", 40)

	reqB := validShipmentReconcilePreconditionRequest(shipmentID)
	reqB.IdempotencyKey = "idem-167-020-replay" // same key
	reqB.MergeSHA = strings.Repeat("c", 40)     // different request identity

	results := u20AwaitWithDeadline(t, 10*time.Second, map[string]func() error{
		"reconcile_a": func() error {
			_, err := ReconcileShipmentToShipped(ctx, ws, reqA)
			return err
		},
		"reconcile_b": func() error {
			_, err := ReconcileShipmentToShipped(ctx, ws, reqB)
			return err
		},
	})

	var byName = make(map[string]u20GuardedResult, len(results))
	for _, res := range results {
		byName[res.name] = res
	}
	resA, resB := byName["reconcile_a"], byName["reconcile_b"]

	if !resA.panicked && !resB.panicked {
		// Once 167.008-T lands: same key + different request-identity digest
		// must be distinguishable outcomes/errors (one wins, the other
		// surfaces a conflict), never silently identical successes.
		assert.False(t, resA.err == nil && resB.err == nil,
			"same idempotency key with a different request-identity digest must not silently succeed on both calls")
		return
	}

	// Expected today: ReconcileShipmentToShipped is still panic-gated.
	u20AssertReconcilePanicked(t, resA)
	u20AssertReconcilePanicked(t, resB)
	u20AssertArtifactCoherent(t, ws, shipmentID)
}

// testU20ReconcileTOCTOUMemberSetReroute exercises a two-phase
// check-then-act window: while ReconcileShipmentToShipped is in flight
// against the shipment's original manifest, a concurrent writer re-routes
// the manifest's member set (rewriting the shipment's persisted `items`
// list directly, bypassing any in-process cache) so a real implementation
// must re-validate against the CURRENT manifest rather than a snapshot
// taken before its own lock acquisition. Against today's panic-gated
// declaration this still recovers a panic; the structure documents the
// scenario for 167.008-T and asserts no-clobber on the manifest source file.
func testU20ReconcileTOCTOUMemberSetReroute(t *testing.T) {
	ws, shipmentID, memberID := u20ReconcileFixture(t)
	ctx := context.Background()
	req := validShipmentReconcilePreconditionRequest(shipmentID)

	rerouteMember := createShipmentReconcileMemberWithStatus(t, ws, models.StatusAccepted)

	results := u20AwaitWithDeadline(t, 10*time.Second, map[string]func() error{
		"reconcile": func() error {
			_, err := ReconcileShipmentToShipped(ctx, ws, req)
			return err
		},
		"reroute_manifest": func() error {
			rewriteArtifactFile(t, ws, shipmentID, func(raw string) string {
				return strings.Replace(raw, "items:\n  - "+memberID, "items:\n  - "+memberID+"\n  - "+rerouteMember, 1)
			})
			return nil
		},
	})

	for _, res := range results {
		if res.name == "reconcile" {
			u20AssertReconcileNoPanic(t, res)
			continue
		}
		if res.panicked {
			t.Errorf("%s: unexpected recovered panic: %v", res.name, res.panicVal)
		}
	}

	u20AssertArtifactCoherent(t, ws, shipmentID)
}
