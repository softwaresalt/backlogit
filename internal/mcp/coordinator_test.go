package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// TestU2_CoordinatorFIFOMutationOrderAndConcurrentReads gates SC3 and R7/R8.
func TestU2_CoordinatorFIFOMutationOrderAndConcurrentReads(t *testing.T) {
	// U1 owns the declared public seams. U2 owns the new private scheduling
	// fields and Server wiring: inspect source, never fabricate declarations.
	source, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	require.NoError(t, err)
	wired := false
	ast.Inspect(source, func(n ast.Node) bool {
		field, ok := n.(*ast.Field)
		if !ok {
			return true
		}
		for _, name := range field.Names {
			if name.Name == "coordinator" {
				ptr, pointer := field.Type.(*ast.StarExpr)
				if pointer {
					typ, named := ptr.X.(*ast.Ident)
					wired = named && typ.Name == "OperationCoordinator"
				}
			}
		}
		return true
	})
	assert.True(t, wired, "U2: Server coordinator *OperationCoordinator is not declared in server.go")

	for _, tc := range []struct {
		name string
		args map[string]any // SDK dynamic arguments, deliberately not coerced.
		want OperationKind
	}{
		{"backlogit_get_item", nil, OpReadOnly},
		{"backlogit_list_templates", nil, OpReadOnly},
		{"backlogit_poll_hook_events", nil, OpReadOnly},
		{"backlogit_custody_verify", nil, OpReadOnly},
		{"backlogit_doctor", nil, OpReadOnly},
		{"backlogit_doctor", map[string]any{"fix_orphans": true}, OpMutating},
		{"backlogit_docs_migrate", nil, OpReadOnly},
		{"backlogit_docs_migrate", map[string]any{"apply": true}, OpMutating},
		{"backlogit_merge_sync", map[string]any{"dry_run": true}, OpReadOnly},
		{"backlogit_merge_sync", map[string]any{"dry_run": false}, OpMutating},
		{"backlogit_sync_index", nil, OpExclusiveBarrier},
		{"unknown_tool", nil, OpMutating},
	} {
		t.Run(fmt.Sprintf("%s/%v", tc.name, tc.args), func(t *testing.T) {
			assert.Equal(t, tc.want, ClassifyToolOperation(tc.name, tc.args), "U2 classification: %s %v", tc.name, tc.args)
		})
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	c := NewOperationCoordinator(CoordinatorConfig{})
	var workers sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		workers.Wait()
		assert.NoError(t, c.Close(), "U2: synchronous coordinator shutdown")
	})
	// This is a behavior assertion against the already-declared Execute seam,
	// not a build-error RED or a test-side replacement coordinator.
	called := false
	result, err := c.Execute(ctx, nil, "backlogit_create_item", mcplib.CallToolRequest{},
		func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			called = true
			return mcplib.NewToolResultText("smoke"), nil
		})
	require.NoError(t, err, "U2: coordinated mutation must execute synchronously")
	require.True(t, called)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	// Reflect only on U2-owned state so this harness compiles before it exists.
	// Channel Len/Cap are concurrency-safe and never receive or forge grants.
	queue := reflect.ValueOf(c).Elem().FieldByName("mutationQueue")
	require.True(t, queue.IsValid(), "U2: mutationQueue channel is not declared in coordinator.go")
	require.Equal(t, reflect.Chan, queue.Kind())
	require.Equal(t, 32, queue.Cap())
	wait := func(predicate func() bool) {
		for !predicate() {
			select {
			case <-ctx.Done():
				t.Fatal("U2: timed out awaiting observable admission")
			default:
				runtime.Gosched()
			}
		}
	}
	var mu sync.Mutex
	var active atomic.Int32
	var overlap atomic.Bool
	order := make([]int, 0, 8)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	outcomes := make(chan error, 8)
	for i := range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			res, callErr := c.Execute(ctx, nil, "backlogit_create_item", mcplib.CallToolRequest{},
				func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
					if active.Add(1) != 1 {
						overlap.Store(true)
					}
					defer active.Add(-1)
					mu.Lock()
					order = append(order, i)
					mu.Unlock()
					if i == 0 {
						close(firstStarted)
						select {
						case <-releaseFirst:
						case <-ctx.Done():
							return nil, ctx.Err()
						}
					}
					return mcplib.NewToolResultText("write"), nil
				})
			if callErr == nil && (res == nil || res.IsError) {
				callErr = context.Canceled
			}
			outcomes <- callErr
		}()
		if i == 0 {
			select {
			case <-firstStarted:
			case <-ctx.Done():
				t.Fatal("U2: first FIFO holder did not start")
			}
		} else {
			wait(func() bool { return queue.Len() == i })
		}
	}

	readStarted := make(chan struct{}, 4)
	releaseReads := make(chan struct{})
	readOutcomes := make(chan error, 4)
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, readErr := c.Execute(ctx, nil, "backlogit_get_item", mcplib.CallToolRequest{},
				func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
					readStarted <- struct{}{}
					select {
					case <-releaseReads:
						return mcplib.NewToolResultText("read"), nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				})
			readOutcomes <- readErr
		}()
	}
	for range 4 {
		select {
		case <-readStarted:
		case <-ctx.Done():
			t.Fatal("U2: four readers must run together while a mutation holds its turn")
		}
	}
	close(releaseFirst)
	for range 8 {
		require.NoError(t, <-outcomes)
	}
	mu.Lock()
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7}, order, "U2: FIFO admitted writes cannot overlap or disappear")
	mu.Unlock()
	assert.False(t, overlap.Load(), "U2: mutating handlers must never own overlapping turns")

	barrierStarted := make(chan struct{})
	barrierDone := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, barrierErr := c.Execute(ctx, nil, "backlogit_sync_index", mcplib.CallToolRequest{},
			func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
				close(barrierStarted)
				return mcplib.NewToolResultText("rehydrated"), nil
			})
		barrierDone <- barrierErr
	}()
	// Keep all readers held for an observation window; the barrier cannot run.
	select {
	case <-barrierStarted:
		t.Fatal("U2: sync_index ran before active readers drained")
	case err := <-barrierDone:
		t.Fatalf("U2: barrier returned before read drain: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseReads)
	for range 4 {
		require.NoError(t, <-readOutcomes)
	}
	require.NoError(t, <-barrierDone)
	select {
	case <-barrierStarted:
	default:
		t.Fatal("U2: sync_index handler did not run after drain")
	}

	u2ReviewfixWorkspaceAcquisition(t)
}

// u2ReviewfixWorkspaceAcquisition exercises public dispatch, not direct handlers.
// The existing private fixtures operate only inside the command's short TEMP.
func u2ReviewfixWorkspaceAcquisition(t *testing.T) {
	closeServer := func(t *testing.T, s *Server) {
		t.Helper()
		t.Cleanup(func() {
			require.NoError(t, s.coordinator.Close())
			if s.Workspace != nil {
				require.NoError(t, s.Workspace.Close())
			}
		})
	}
	t.Run("C1_poisoned_journal_diagnostic_acquisition", func(t *testing.T) {
		root, shipmentID, snapshotRef := newPoisonedNormalizationWorkspace(t)
		s := NewServerForRoot(root)
		closeServer(t, s)
		t.Run("doctor", func(t *testing.T) {
			result := custodyTestInvoke(t, t.Context(), s, "backlogit_doctor", map[string]any{
				"check_orphans": false, "check_duplicates": false,
			})
			assert.False(t, result.IsError, "C1: diagnostic doctor must bypass poisoned ordinary initialization: %s", custodyTestText(t, result))
			assert.Contains(t, custodyTestText(t, result), "invalid_shipment_lifecycle_journal")
		})
		t.Run("normalize_blocked_shipment", func(t *testing.T) {
			result := custodyTestInvoke(t, t.Context(), s, "backlogit_normalize_blocked_shipment", map[string]any{
				"id": shipmentID, "snapshot_ref": snapshotRef, "by": "u2-c1",
			})
			assert.False(t, result.IsError, "C1: normalizer must use diagnostic acquisition: %s", custodyTestText(t, result))
			ordinary, err := core.NewWorkspace(t.Context(), root)
			assert.Error(t, err, "C1: unrelated poison must still reject ordinary acquisition")
			if ordinary != nil {
				require.NoError(t, ordinary.Close())
			}
			assert.Nil(t, ordinary)
		})
	})
	for _, tool := range []string{"backlogit_custody_preview", "backlogit_custody_verify"} {
		t.Run("C1_cold_zero_write/"+tool, func(t *testing.T) {
			repo := custodyTestNewRepo(t)
			custodyTestRemoveDB(t, repo.root)
			custodyTestWrite(t, repo.root, custodyTestStageFile, custodyTestRevised)
			s := NewServerForRoot(repo.root)
			closeServer(t, s)
			require.Nil(t, s.Workspace)
			before := custodyTestSnapshot(t, repo.root)
			// A refused read must also be zero-write. Assert this prerequisite
			// before the valid call: current unconditional initialization both
			// writes SQLite and exposes a separate reentrant manifest mutex bug.
			// No skip or success fallback: GREEN must reach the valid call below.
			refused := custodyTestInvoke(t, t.Context(), s, tool, nil)
			assert.True(t, refused.IsError, "C1: missing shipment_id must be rejected")
			assert.Equal(t, before, custodyTestSnapshot(t, repo.root), "C1: even a refused cold custody read must be zero-write")
			require.True(t, s.Workspace == nil, "C1: refused cold custody read must not initialize SQLite before handler validation")
			args := map[string]any{"shipment_id": repo.stageShip, "branch": custodyTestBranch}
			if tool == "backlogit_custody_preview" {
				args["candidates"] = custodyTestJSON(t, []core.CustodyCandidateSpec{{
					Path: custodyTestStageFile, ChangeKind: "modified", StagedState: "unstaged_only",
					Contributor: "stage", SourceKind: "stage_artifact",
				}})
			}
			result := custodyTestInvoke(t, t.Context(), s, tool, args)
			assert.False(t, result.IsError, "C1: cold custody read succeeds: %s", custodyTestText(t, result))
			assert.True(t, s.Workspace == nil, "C1: custody read must retain handler-specific read-only acquisition")
			assert.Equal(t, before, custodyTestSnapshot(t, repo.root), "C1: cold custody read is zero-write")
			_, err := os.Stat(filepath.Join(repo.root, ".backlogit", "backlogit.db"))
			assert.True(t, os.IsNotExist(err), "C1: cold custody read must not create SQLite")
		})
	}
	t.Run("C1_corrupt_sqlite_canonical_custody_fallback", func(t *testing.T) {
		repo := custodyTestNewRepo(t)
		custodyTestWrite(t, repo.root, custodyTestStageFile, custodyTestRevised)
		// Derive valid receipt pins before corrupting the cache, so the fallback
		// regression cannot be hidden behind a failing preview prerequisite.
		pinsWS, err := core.NewReadOnlyWorkspace(t.Context(), repo.root)
		require.NoError(t, err)
		preview, err := core.PreviewCustodyCandidates(t.Context(), pinsWS, core.CustodyPreviewOptions{
			ShipmentID: repo.stageShip, Branch: custodyTestBranch,
			Candidates: []core.CustodyCandidateSpec{{
				Path: custodyTestStageFile, ChangeKind: "modified", StagedState: "unstaged_only",
				Contributor: "stage", SourceKind: "stage_artifact",
			}},
		})
		require.NoError(t, err)
		require.NoError(t, pinsWS.Close())
		custodyTestCorruptDB(t, repo.root)
		s := NewServerForRoot(repo.root)
		closeServer(t, s)
		spec := custodyTestJSON(t, core.CustodyReceipt{
			ReceiptID: "rcpt-u2-c1", Revision: 1, Kind: "stage_to_ship_handoff",
			AuthorityProfile: "agent_workflow_handoff", Status: "offered",
			ProducerRole: "stage", AcceptingRole: "ship", AllowedAction: "stage_artifact_commit",
			CandidateSetDigest: preview.CandidateSetDigest,
			Entries:            []core.CustodyEntry{custodyTestEntry(custodyTestStageFile, "stage", "stage_artifact")},
		})
		t.Run("offer", func(t *testing.T) {
			result := custodyTestInvoke(t, t.Context(), s, "backlogit_custody_offer", map[string]any{
				"shipment_id": repo.stageShip, "spec": spec,
			})
			assert.False(t, result.IsError, "C1: corrupt-cache offer must use canonical fallback: %s", custodyTestText(t, result))
			if !result.IsError {
				resp := custodyTestMutation(t, result)
				assert.Equal(t, "offered", resp.Status)
				assert.Equal(t, "unavailable", resp.DB)
			}
		})
		// Seed a valid canonical offer through the DB-optional public core API,
		// independently of the MCP offer outcome; accept must get its own RED.
		ws, err := core.NewReadOnlyWorkspace(t.Context(), repo.root)
		require.NoError(t, err)
		defer func() { require.NoError(t, ws.Close()) }()
		ws.HeaderDef, err = config.LoadHeaderDef(ws.StorageRoot)
		require.NoError(t, err)
		var receipt core.CustodyReceipt
		require.NoError(t, json.Unmarshal([]byte(spec), &receipt))
		if len(custodyTestEnvelope(t, repo.root, repo.stageShip).Receipts) == 0 {
			_, err = core.OfferCustodyHandoff(t.Context(), ws, core.CustodyOfferOptions{
				ShipmentID: repo.stageShip, Spec: receipt,
			})
			require.NoError(t, err)
		}
		t.Run("accept", func(t *testing.T) {
			result := custodyTestInvoke(t, t.Context(), s, "backlogit_custody_accept", map[string]any{
				"shipment_id": repo.stageShip, "receipt_id": "rcpt-u2-c1", "by": "ship",
			})
			assert.False(t, result.IsError, "C1: corrupt-cache accept must use canonical fallback: %s", custodyTestText(t, result))
			if !result.IsError {
				resp := custodyTestMutation(t, result)
				assert.Equal(t, "accepted", resp.Status)
				assert.Equal(t, "unavailable", resp.DB)
			}
		})
	})
	t.Run("C1_exclusive_cold_dispatch_barrier", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlogit"), 0o755))
		require.NoError(t, config.WriteDefaults(filepath.Join(root, ".backlogit")))
		s := NewServerForRoot(root)
		closeServer(t, s)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		var workers sync.WaitGroup
		t.Cleanup(func() { cancel(); workers.Wait() })
		first := make(chan struct{})
		release := make(chan struct{})
		second := make(chan struct{})
		outcomes := make(chan error, 2)
		// Register read-only probes under existing read classifications. They
		// observe dispatch exclusion without acquiring or mutating a workspace.
		s.addTool(mcplib.NewTool("backlogit_custody_verify"), func(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			close(first)
			select {
			case <-release:
				return mcplib.NewToolResultText("first"), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		s.addTool(mcplib.NewTool("backlogit_custody_preview"), func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			close(second)
			return mcplib.NewToolResultText("second"), nil
		})
		invoke := func(tool string) {
			workers.Add(1)
			go func() {
				defer workers.Done()
				_, err := s.InvokeTool(ctx, tool, mcplib.CallToolRequest{})
				outcomes <- err
			}()
		}
		invoke("backlogit_custody_verify")
		select {
		case <-first:
		case <-ctx.Done():
			t.Fatal("C1: first cold dispatch did not start")
		}
		invoke("backlogit_custody_preview")
		for len(s.coordinator.readTokens) != 2 {
			select {
			case <-ctx.Done():
				t.Fatal("C1: second reader did not acquire coordinator admission")
			default:
				runtime.Gosched()
			}
		}
		select {
		case <-second:
			t.Error("C1: second cold handler bypassed exclusive dispatch barrier")
		case <-time.After(25 * time.Millisecond):
		}
		close(release)
		require.NoError(t, <-outcomes)
		require.NoError(t, <-outcomes)
	})
}

// TestU2_CoordinatorQueueFullAndCallerCancellationSafety gates SC4 and R9.
func TestU2_CoordinatorQueueFullAndCallerCancellationSafety(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	c := NewOperationCoordinator(CoordinatorConfig{})
	var workers sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		workers.Wait()
		assert.NoError(t, c.Close(), "U2: cancelled callers cannot strand shutdown")
	})
	// Already cancelled calls must return the typed, retryable envelope without
	// executing next. All RED assertions below target existing U1 declarations.
	busy := func(res *mcplib.CallToolResult, err error, code string) {
		require.NoError(t, err, "U2: admission failure must be a tool error envelope")
		require.NotNil(t, res)
		require.True(t, res.IsError)
		require.Len(t, res.Content, 1)
		text, ok := res.Content[0].(mcplib.TextContent)
		require.True(t, ok)
		var envelope BusyErrorResponse
		require.NoError(t, json.Unmarshal([]byte(text.Text), &envelope))
		assert.Equal(t, "server_busy", envelope.Error)
		assert.Equal(t, code, envelope.Code)
		assert.True(t, envelope.Retryable)
		assert.NotEmpty(t, envelope.Message)
		assert.NotEmpty(t, envelope.Remediation)
	}
	var partial atomic.Int32
	next := func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		partial.Add(1)
		return mcplib.NewToolResultText("must not run"), nil
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	res, err := c.Execute(cancelled, nil, "backlogit_create_item", mcplib.CallToolRequest{}, next)
	busy(res, err, "request_cancelled")
	require.Zero(t, partial.Load())

	queue := reflect.ValueOf(c).Elem().FieldByName("mutationQueue")
	require.True(t, queue.IsValid(), "U2: mutationQueue channel is not declared in coordinator.go")
	require.Equal(t, 32, queue.Cap())
	wait := func(predicate func() bool) {
		for !predicate() {
			select {
			case <-ctx.Done():
				t.Fatal("U2: bounded queue/cancellation failed to make progress")
			default:
				runtime.Gosched()
			}
		}
	}
	holderStarted := make(chan struct{})
	releaseHolder := make(chan struct{})
	done := make(chan error, 33)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, holderErr := c.Execute(ctx, nil, "backlogit_create_item", mcplib.CallToolRequest{},
			func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
				close(holderStarted)
				select {
				case <-releaseHolder:
					return mcplib.NewToolResultText("holder"), nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
		done <- holderErr
	}()
	select {
	case <-holderStarted:
	case <-ctx.Done():
		t.Fatal("U2: mutation holder did not start")
	}
	queuedCtx, cancelQueued := context.WithCancel(ctx)
	defer cancelQueued()
	cancelledDone := make(chan struct {
		result *mcplib.CallToolResult
		err    error
	}, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		result, callErr := c.Execute(queuedCtx, nil, "backlogit_create_item", mcplib.CallToolRequest{}, next)
		cancelledDone <- struct {
			result *mcplib.CallToolResult
			err    error
		}{result, callErr}
	}()
	wait(func() bool { return queue.Len() == 1 })
	for i := 2; i <= 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, callErr := c.Execute(ctx, nil, "backlogit_create_item", mcplib.CallToolRequest{},
				func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
					return mcplib.NewToolResultText("successor"), nil
				})
			if callErr == nil && (result == nil || result.IsError) {
				callErr = context.Canceled
			}
			done <- callErr
		}()
		wait(func() bool { return queue.Len() == i })
	}
	res, err = c.Execute(ctx, nil, "backlogit_create_item", mcplib.CallToolRequest{}, next)
	busy(res, err, "mutation_queue_full")
	cancelQueued()
	select {
	case outcome := <-cancelledDone:
		busy(outcome.result, outcome.err, "request_cancelled")
	case <-ctx.Done():
		t.Fatal("U2: queued cancellation did not return while predecessor was held")
	}
	require.Zero(t, partial.Load(), "U2: rejected and abandoned mutations must not execute")
	close(releaseHolder)
	for range 32 {
		require.NoError(t, <-done, "U2: abandoned FIFO waiter stranded a successor")
	}

	// Partial barrier drain: hold one read token, observe the other three being
	// acquired, cancel, and require all four readers can subsequently enter.
	tokens := reflect.ValueOf(c).Elem().FieldByName("readTokens")
	require.True(t, tokens.IsValid(), "U2: readTokens channel is not declared in coordinator.go")
	require.Equal(t, 4, tokens.Cap())
	readerStarted := make(chan struct{})
	releaseReader := make(chan struct{})
	readerDone := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, readErr := c.Execute(ctx, nil, "backlogit_get_item", mcplib.CallToolRequest{},
			func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
				close(readerStarted)
				select {
				case <-releaseReader:
					return mcplib.NewToolResultText("read"), nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
		readerDone <- readErr
	}()
	select {
	case <-readerStarted:
	case <-ctx.Done():
		t.Fatal("U2: held reader did not start")
	}
	barrierCtx, cancelBarrier := context.WithCancel(ctx)
	defer cancelBarrier()
	barrierDone := make(chan struct {
		result *mcplib.CallToolResult
		err    error
	}, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		result, callErr := c.Execute(barrierCtx, nil, "backlogit_sync_index", mcplib.CallToolRequest{}, next)
		barrierDone <- struct {
			result *mcplib.CallToolResult
			err    error
		}{result, callErr}
	}()
	// D1 specifies a pool of available tokens drained by the barrier.
	wait(func() bool { return tokens.Len() == 0 })
	cancelBarrier()
	select {
	case outcome := <-barrierDone:
		busy(outcome.result, outcome.err, "request_cancelled")
	case <-ctx.Done():
		t.Fatal("U2: partially admitted barrier stranded cancellation")
	}
	assert.Zero(t, partial.Load())
	wait(func() bool { return tokens.Len() == 3 })
	close(releaseReader)
	require.NoError(t, <-readerDone)
	wait(func() bool { return tokens.Len() == 4 })
	// A fresh mutation must still own and release its synchronous turn.
	res, err = c.Execute(ctx, nil, "backlogit_create_item", mcplib.CallToolRequest{},
		func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			return mcplib.NewToolResultText("recovered"), nil
		})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.IsError)
}

// u2NotificationSession is a minimal SDK fixture, not the future U3 dispatcher.
// No transport, request map, session lifecycle, or production stub is scaffolded.
type u2NotificationSession struct {
	id            string
	ready         bool
	notifications chan mcplib.JSONRPCNotification
}

func (s *u2NotificationSession) Initialize()       { s.ready = true }
func (s *u2NotificationSession) Initialized() bool { return s.ready }
func (s *u2NotificationSession) SessionID() string { return s.id }
func (s *u2NotificationSession) NotificationChannel() chan<- mcplib.JSONRPCNotification {
	return s.notifications
}

// TestU2_CoordinatorProgressNotificationsIsolatedToCallerSession gates R10/D8.
func TestU2_CoordinatorProgressNotificationsIsolatedToCallerSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	c := NewOperationCoordinator(CoordinatorConfig{})
	t.Cleanup(func() { assert.NoError(t, c.Close()) })
	srv := mcpserver.NewMCPServer("u2-only", "test")
	caller := &u2NotificationSession{"caller", true, make(chan mcplib.JSONRPCNotification, 16)}
	peer := &u2NotificationSession{"peer", true, make(chan mcplib.JSONRPCNotification, 16)}
	require.NoError(t, srv.RegisterSession(ctx, caller))
	require.NoError(t, srv.RegisterSession(ctx, peer))
	defer srv.UnregisterSession(ctx, caller.id)
	defer srv.UnregisterSession(ctx, peer.id)
	var req mcplib.CallToolRequest
	require.NoError(t, json.Unmarshal([]byte(`{"params":{"name":"backlogit_create_item","_meta":{"progressToken":"u2-caller-token"}}}`), &req))
	called := 0
	next := func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		called++
		return mcplib.NewToolResultText("synchronous-result"), nil
	}
	res, err := c.Execute(srv.WithContext(ctx, caller), srv, req.Params.Name, req, next)
	require.NoError(t, err, "U2: progress must preserve synchronous tools/call result")
	require.NotNil(t, res)
	require.False(t, res.IsError)
	assert.Equal(t, 1, called)
	require.Len(t, caller.notifications, 3, "U2: caller must receive queued, started, completed progress")
	for _, stage := range []string{"queued", "started", "completed"} {
		n := <-caller.notifications
		assert.Equal(t, "notifications/progress", n.Method)
		assert.Equal(t, "u2-caller-token", n.Params.AdditionalFields["progressToken"])
		assert.Contains(t, n.Params.AdditionalFields["message"], stage)
		assert.Contains(t, n.Params.AdditionalFields, "progress")
	}
	assert.Empty(t, peer.notifications, "U2: peer session must receive no caller progress")
	req.Params.Meta = nil
	res, err = c.Execute(srv.WithContext(ctx, caller), srv, req.Params.Name, req, next)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Empty(t, caller.notifications, "U2: no meta token means no progress")
	caller.ready = false
	require.NoError(t, json.Unmarshal([]byte(`{"params":{"name":"backlogit_create_item","_meta":{"progressToken":17}}}`), &req))
	res, err = c.Execute(srv.WithContext(ctx, caller), srv, req.Params.Name, req, next)
	require.NoError(t, err, "U2: unsupported session notification capability cannot abort mutation")
	require.NotNil(t, res)
	assert.Empty(t, caller.notifications)
	assert.Empty(t, peer.notifications)

	t.Run("C1_handler_error_context_and_sentinel_chain", func(t *testing.T) {
		sentinel := errors.New("u2-c1-handler-sentinel")
		for _, tool := range []string{"backlogit_get_item", "backlogit_create_item", "backlogit_sync_index"} {
			t.Run(tool, func(t *testing.T) {
				expected := mcplib.NewToolResultText("partial-result")
				result, callErr := c.Execute(ctx, nil, tool, mcplib.CallToolRequest{},
					func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
						return expected, sentinel
					})
				require.Error(t, callErr)
				assert.ErrorIs(t, callErr, sentinel, "C1: wrapping must preserve handler sentinel")
				assert.NotSame(t, sentinel, callErr, "C1: handler errors require a contextual wrapper")
				assert.Contains(t, callErr.Error(), tool, "C1: execution error must identify the dispatched tool")
				assert.Same(t, expected, result, "C1: wrapping must preserve synchronous handler result")
			})
		}
	})
}
