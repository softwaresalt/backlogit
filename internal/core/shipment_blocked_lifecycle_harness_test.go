package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

const ur1BSlotInstructionEnv = "BACKLOGIT_UR1B_SLOT_INSTRUCTION"

type ur1BSlotInstruction struct {
	Root       string `json:"root"`
	ShipmentID string `json:"shipment_id"`
	ReadyPath  string `json:"ready_path"`
	StartPath  string `json:"start_path"`
	ResultPath string `json:"result_path"`
}

type ur1BSlotResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func TestShipmentBlockedActiveSlotSubprocessHelper(t *testing.T) {
	instructionPath := os.Getenv(ur1BSlotInstructionEnv)
	if instructionPath == "" {
		t.Skip("subprocess-only active-slot helper")
	}
	data, err := os.ReadFile(instructionPath)
	require.NoError(t, err)
	var instruction ur1BSlotInstruction
	require.NoError(t, json.Unmarshal(data, &instruction))
	ws, err := NewWorkspace(context.Background(), instruction.Root)
	require.NoError(t, err)
	defer func() { require.NoError(t, ws.Close()) }()
	require.NoError(t, os.WriteFile(instruction.ReadyPath, []byte("ready"), 0o644))

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, statErr := os.Stat(instruction.StartPath); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			require.NoError(t, statErr)
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "active-slot subprocess start barrier timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}

	_, claimErr := ClaimShipment(context.Background(), ws, instruction.ShipmentID)
	result := ur1BSlotResult{Success: claimErr == nil}
	if claimErr != nil {
		result.Error = claimErr.Error()
	}
	resultData, err := json.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(instruction.ResultPath, resultData, 0o644))
}

func TestUR1B_BlockPersistsMetadataPreimageAndMemberDisposition(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	fixture := newURBlockedActiveFixture(t, ws)
	ctx := context.Background()

	before := make(map[string]string, len(fixture.members))
	memberPreimages := make([]*models.Artifact, 0, len(fixture.members))
	for _, member := range fixture.members {
		current := loadURCanonicalArtifact(t, ws, member.ID)
		before[member.ID] = string(current.Status)
		memberPreimages = append(memberPreimages, cloneArtifact(current))
	}
	shipmentBefore := loadURCanonicalArtifact(t, ws, fixture.shipment.ID)
	if shipmentBefore.CustomFields == nil {
		shipmentBefore.CustomFields = map[string]any{}
	}
	shipmentBefore.CustomFields["branch"] = "feat/ur-blocked-lifecycle"
	forceURArtifactFixture(t, ws, shipmentBefore)

	opts := BlockOptions{
		Reason:              "waiting: external system\nstatus: must remain opaque",
		BlockedBy:           "wave-3-harness",
		ResumeCheckpointRef: "checkpoint-ur1b.json",
	}
	var firstMutation sync.Once
	observedPrewriteIntent := false
	var durableJournalPath string
	persistArtifactPreLockHook = func(_ string) {
		firstMutation.Do(func() {
			observedPrewriteIntent = true
			durableJournalPath = requireDurableLifecycleIntentPreimageUR(
				t,
				ws,
				"block",
				shipmentBefore,
				memberPreimages,
			)
			assertURArtifactEqual(t, shipmentBefore, loadURCanonicalArtifact(t, ws, shipmentBefore.ID))
			for _, member := range memberPreimages {
				assertURArtifactEqual(t, member, loadURCanonicalArtifact(t, ws, member.ID))
			}
		})
	}
	defer func() { persistArtifactPreLockHook = nil }()

	blocked, err := BlockShipment(ctx, ws, fixture.shipment.ID, opts)
	require.NoError(t, err, "BlockShipment must implement the governed active-to-blocked transition")
	require.NotNil(t, blocked)
	assert.True(t, observedPrewriteIntent,
		"the internal pre-write seam must observe durable intent and complete preimage before the first artifact mutation")

	blocked = loadURCanonicalArtifact(t, ws, fixture.shipment.ID)
	assert.Equal(t, models.ArtifactStatus(ShipmentBlocked), blocked.Status)
	require.NotNil(t, blocked.CustomFields)
	assert.Equal(t, opts.Reason, blocked.CustomFields["blocked_reason"])
	assert.Equal(t, opts.BlockedBy, blocked.CustomFields["blocked_by"])
	assert.Equal(t, opts.ResumeCheckpointRef, blocked.CustomFields["resume_checkpoint_ref"])
	assert.Equal(t, "feat/ur-blocked-lifecycle", blocked.CustomFields["branch"], "branch association must survive")
	switch blockedAt := blocked.CustomFields["blocked_at"].(type) {
	case string:
		_, parseErr := time.Parse(time.RFC3339, blockedAt)
		assert.NoError(t, parseErr)
	case time.Time:
		assert.False(t, blockedAt.IsZero(), "blocked_at must be a valid timestamp")
	default:
		assert.Fail(t, "blocked_at must be an RFC3339 timestamp", "got %T", blockedAt)
	}

	snapshot := statusSnapshotUR(blocked.CustomFields)
	for memberID, status := range before {
		assert.Equal(t, status, snapshot[memberID], "preimage status for member %s", memberID)
		member := loadURCanonicalArtifact(t, ws, memberID)
		assert.Equal(t, models.StatusQueued, member.Status, "blocked shipment cannot retain active member %s", memberID)
	}

	itemEvents := requireCorrelatedLifecycleIntentCommitUR(
		t,
		ws,
		fixture.shipment.ID,
		"block",
		ShipmentBlocked,
		durableJournalPath,
	)
	correlationID := readURLifecycleJournalCorrelation(t, durableJournalPath)
	intentIndex, intent, commitIndex, _, found := findCorrelatedLifecyclePairUR(
		itemEvents,
		"block",
		ShipmentBlocked,
		correlationID,
	)
	require.True(t, found)
	intentPreimage := statusSnapshotUR(intent.Delta)
	for memberID, status := range before {
		assert.Equal(t, status, intentPreimage[memberID], "durable intent must carry complete preimage for %s", memberID)
	}
	statusIndex := -1
	for index := intentIndex + 1; index < commitIndex; index++ {
		event := itemEvents[index]
		if event.EventType != "shipment_status_changed" ||
			exactEventCorrelationUR(event) != correlationID ||
			eventLifecycleOperationUR(event) != "block" ||
			eventTargetUR(event) != string(ShipmentBlocked) {
			continue
		}
		statusIndex = index
		assert.Equal(t, opts.Reason, event.Delta["reason"])
		assert.Equal(t, opts.ResumeCheckpointRef, event.Delta["resume_checkpoint_ref"])
		actor := event.Actor
		if actor == "" {
			actor, _ = event.Delta["actor"].(string)
		}
		if actor == "" {
			actor, _ = event.Delta["blocked_by"].(string)
		}
		assert.Equal(t, opts.BlockedBy, actor)
		break
	}
	require.GreaterOrEqual(t, statusIndex, 0, "authoritative shipment_status_changed event")
	require.Less(t, intentIndex, statusIndex, "intent and preimage must be durable before transition evidence")
	require.Less(t, statusIndex, commitIndex, "commit must follow transition evidence")
}

func TestUR1B_UnblockIsTargetAwareAndPreservesResumptionEvidence(t *testing.T) {
	tests := []struct {
		name               string
		target             ShipmentStatus
		wantShipmentStatus models.ArtifactStatus
	}{
		{
			name:               "queued_preserves_snapshot_and_leaves_members_queued",
			target:             ShipmentQueued,
			wantShipmentStatus: models.StatusQueued,
		},
		{
			name:               "active_restores_exact_snapshot",
			target:             ShipmentActive,
			wantShipmentStatus: models.StatusActive,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			fixture := newURBlockedActiveFixture(t, ws)
			ctx := context.Background()
			checkpoint := fmt.Sprintf("%s-checkpoint.json", test.name)

			_, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
				Reason:              "temporary blocker",
				BlockedBy:           "block-actor",
				ResumeCheckpointRef: checkpoint,
			})
			require.NoError(t, err)
			blocked := loadURCanonicalArtifact(t, ws, fixture.shipment.ID)
			blockSnapshot := statusSnapshotUR(blocked.CustomFields)
			assert.Len(t, blockSnapshot, len(fixture.members), "snapshot must identify every heterogeneous member")

			competing, err := CreateShipment(ctx, ws, "active-slot competitor "+test.name, nil)
			require.NoError(t, err)
			claimedCompeting, err := ClaimShipment(ctx, ws, competing.ID)
			require.NoError(t, err,
				"blocking the original shipment must release its active slot for another claim")
			assert.Equal(t, models.StatusActive, claimedCompeting.Status)

			if test.target == ShipmentActive {
				occupiedRefusalPreimage := snapshotURAggregate(t, ws, fixture.shipment.ID)
				_, err = UnblockShipment(ctx, ws, fixture.shipment.ID, UnblockOptions{
					Target:      ShipmentActive,
					Confirm:     true,
					UnblockedBy: "unblock-actor",
				})
				require.Error(t, err, "unblock-to-active must refuse while another shipment owns the active slot")
				requireURAggregateUnchanged(t, ws, occupiedRefusalPreimage)

				_, err = BlockShipment(ctx, ws, competing.ID, BlockOptions{
					Reason:    "release slot for resume",
					BlockedBy: "slot-test",
				})
				require.NoError(t, err)
			}

			unblockShipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
			unblockMemberPreimages := make([]*models.Artifact, 0, len(fixture.members))
			for _, member := range fixture.members {
				unblockMemberPreimages = append(
					unblockMemberPreimages,
					cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID)),
				)
			}
			var durableJournalPath string
			var firstMutation sync.Once
			persistArtifactPreLockHook = func(_ string) {
				firstMutation.Do(func() {
					durableJournalPath = requireDurableLifecycleIntentPreimageUR(
						t,
						ws,
						"unblock",
						unblockShipmentPreimage,
						unblockMemberPreimages,
					)
				})
			}
			t.Cleanup(func() { persistArtifactPreLockHook = nil })
			unblocked, err := UnblockShipment(ctx, ws, fixture.shipment.ID, UnblockOptions{
				Target:      test.target,
				Confirm:     true,
				UnblockedBy: "unblock-actor",
			})
			persistArtifactPreLockHook = nil
			require.NoError(t, err, "confirmed governed unblock to %s", test.target)
			require.NotNil(t, unblocked)
			require.NotEmpty(t, durableJournalPath,
				"unblock must persist its durable intent before its first artifact mutation")

			unblocked = loadURCanonicalArtifact(t, ws, fixture.shipment.ID)
			assert.Equal(t, test.wantShipmentStatus, unblocked.Status)
			require.NotNil(t, unblocked.CustomFields)
			for _, key := range []string{"blocked_reason", "blocked_at", "blocked_by"} {
				assert.NotContains(t, unblocked.CustomFields, key, "unblock must clear %s", key)
			}
			assert.Equal(t, checkpoint, unblocked.CustomFields["resume_checkpoint_ref"], "checkpoint linkage must survive")
			assert.Equal(t, blockSnapshot, statusSnapshotUR(unblocked.CustomFields), "member snapshot must remain machine-readable")

			for _, member := range fixture.members {
				current := loadURCanonicalArtifact(t, ws, member.ID)
				if test.target == ShipmentQueued {
					assert.Equal(t, models.StatusQueued, current.Status,
						"queued shipment must leave member %s queued", member.ID)
				} else {
					assert.Equal(t, blockSnapshot[member.ID], string(current.Status), "active resume restores exact pre-block status")
				}
			}
			requireCorrelatedLifecycleIntentCommitUR(
				t,
				ws,
				fixture.shipment.ID,
				"unblock",
				test.target,
				durableJournalPath,
			)
		})
	}
}

func TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot(t *testing.T) {
	t.Run("invalid_requests_preserve_the_complete_aggregate", func(t *testing.T) {
		tests := []struct {
			name    string
			arrange func(*testing.T, *Workspace) *models.Artifact
			act     func(context.Context, *Workspace, string) error
		}{
			{
				name: "block_requires_active_shipment",
				arrange: func(t *testing.T, ws *Workspace) *models.Artifact {
					shipment, err := CreateShipment(context.Background(), ws, "queued block refusal", nil)
					require.NoError(t, err)
					return shipment
				},
				act: func(ctx context.Context, ws *Workspace, id string) error {
					_, err := BlockShipment(ctx, ws, id, BlockOptions{Reason: "not active"})
					return err
				},
			},
			{
				name: "block_requires_non_empty_reason",
				arrange: func(t *testing.T, ws *Workspace) *models.Artifact {
					return newURBlockedActiveFixture(t, ws).shipment
				},
				act: func(ctx context.Context, ws *Workspace, id string) error {
					_, err := BlockShipment(ctx, ws, id, BlockOptions{Reason: " \t"})
					return err
				},
			},
			{
				name: "unblock_to_queued_requires_confirmation",
				arrange: func(t *testing.T, ws *Workspace) *models.Artifact {
					fixture := newURBlockedActiveFixture(t, ws)
					_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{Reason: "seed blocked"})
					require.NoError(t, err)
					return loadURCanonicalArtifact(t, ws, fixture.shipment.ID)
				},
				act: func(ctx context.Context, ws *Workspace, id string) error {
					_, err := UnblockShipment(ctx, ws, id, UnblockOptions{Target: ShipmentQueued, Confirm: false})
					return err
				},
			},
			{
				name: "unblock_to_active_requires_confirmation",
				arrange: func(t *testing.T, ws *Workspace) *models.Artifact {
					fixture := newURBlockedActiveFixture(t, ws)
					_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{Reason: "seed blocked"})
					require.NoError(t, err)
					return loadURCanonicalArtifact(t, ws, fixture.shipment.ID)
				},
				act: func(ctx context.Context, ws *Workspace, id string) error {
					_, err := UnblockShipment(ctx, ws, id, UnblockOptions{Target: ShipmentActive, Confirm: false})
					return err
				},
			},
			{
				name: "unblock_refuses_unsupported_target",
				arrange: func(t *testing.T, ws *Workspace) *models.Artifact {
					fixture := newURBlockedActiveFixture(t, ws)
					_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{Reason: "seed blocked"})
					require.NoError(t, err)
					return loadURCanonicalArtifact(t, ws, fixture.shipment.ID)
				},
				act: func(ctx context.Context, ws *Workspace, id string) error {
					_, err := UnblockShipment(ctx, ws, id, UnblockOptions{Target: ShipmentShipped, Confirm: true})
					return err
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				ws := setupShipmentWorkspace(t)
				shipment := test.arrange(t, ws)
				before := snapshotURAggregate(t, ws, shipment.ID)
				err := test.act(context.Background(), ws, shipment.ID)
				require.Error(t, err)
				assert.NotErrorIs(t, err, blerrors.ErrNotImplemented,
					"request must reach lifecycle validation, not a declaration stub")
				requireURAggregateUnchanged(t, ws, before)
			})
		}
	})

	t.Run("canonical_scan_is_authoritative_and_fails_closed", func(t *testing.T) {
		t.Run("stale_index_cannot_hide_an_active_shipment", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			fixture := newURBlockedActiveFixture(t, ws)
			_, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{Reason: "prepare authoritative scan"})
			require.NoError(t, err)

			canonicalCompetitor, err := CreateShipment(ctx, ws, "canonical active competitor", nil)
			require.NoError(t, err)
			activeCompetitor := cloneArtifact(canonicalCompetitor)
			activeCompetitor.Status = models.StatusActive
			forceURArtifactFixture(t, ws, activeCompetitor)
			staleIndex := cloneArtifact(activeCompetitor)
			staleIndex.Status = models.StatusQueued
			require.NoError(t, bldb.UpsertItem(ctx, ws.DB, staleIndex))

			second, err := NewWorkspace(ctx, ws.RootPath)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, second.Close()) })
			before := snapshotURAggregate(t, ws, fixture.shipment.ID)
			_, err = UnblockShipment(ctx, second, fixture.shipment.ID, UnblockOptions{
				Target: ShipmentActive, Confirm: true, UnblockedBy: "second-workspace",
			})
			require.Error(t, err, "canonical active shipment must occupy the slot despite a stale queued index row")
			requireURAggregateUnchanged(t, ws, before)
		})

		t.Run("stale_active_index_cannot_occupy_a_canonically_free_slot", func(t *testing.T) {
			tests := []struct {
				name string
				act  func(*testing.T, context.Context, *Workspace)
			}{
				{
					name: "canonical_queued_permits_claim",
					act: func(t *testing.T, ctx context.Context, ws *Workspace) {
						stale, err := CreateShipment(ctx, ws, "stale-active queued row", nil)
						require.NoError(t, err)
						staleIndex := cloneArtifact(stale)
						staleIndex.Status = models.StatusActive
						require.NoError(t, bldb.UpsertItem(ctx, ws.DB, staleIndex))

						candidate, err := CreateShipment(ctx, ws, "claim despite stale-active row", nil)
						require.NoError(t, err)
						claimed, err := ClaimShipment(ctx, ws, candidate.ID)
						require.NoError(t, err,
							"canonical queued authority must override a stale-active SQLite projection")
						assert.Equal(t, models.StatusActive, claimed.Status)
						assert.Equal(t, models.StatusQueued, loadURCanonicalArtifact(t, ws, stale.ID).Status)
					},
				},
				{
					name: "canonical_blocked_permits_unblock",
					act: func(t *testing.T, ctx context.Context, ws *Workspace) {
						staleFixture := newURBlockedActiveFixture(t, ws)
						_, err := BlockShipment(ctx, ws, staleFixture.shipment.ID, BlockOptions{
							Reason: "canonical blocked stale-index fixture",
						})
						require.NoError(t, err)
						staleIndex := cloneArtifact(loadURCanonicalArtifact(t, ws, staleFixture.shipment.ID))
						staleIndex.Status = models.StatusActive
						require.NoError(t, bldb.UpsertItem(ctx, ws.DB, staleIndex))

						resumeFixture := newURBlockedActiveFixture(t, ws)
						_, err = BlockShipment(ctx, ws, resumeFixture.shipment.ID, BlockOptions{
							Reason: "resume candidate",
						})
						require.NoError(t, err)
						unblocked, err := UnblockShipment(ctx, ws, resumeFixture.shipment.ID, UnblockOptions{
							Target:  ShipmentActive,
							Confirm: true,
						})
						require.NoError(t, err,
							"canonical blocked authority must override a stale-active SQLite projection")
						assert.Equal(t, models.StatusActive, unblocked.Status)
						assert.Equal(t, models.StatusBlocked,
							loadURCanonicalArtifact(t, ws, staleFixture.shipment.ID).Status)
					},
				},
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					ws := setupShipmentWorkspace(t)
					test.act(t, context.Background(), ws)
				})
			}
		})

		t.Run("unparseable_canonical_candidate_refuses_activation", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			candidate, err := CreateShipment(ctx, ws, "scan-failure candidate", nil)
			require.NoError(t, err)
			malformedPath := filepath.Join(workspaceStorageRoot(ws), "queue", "S-malformed-active-slot.md")
			require.NoError(t, os.WriteFile(malformedPath,
				[]byte("---\nid: S-malformed-active-slot\nartifact_type: shipment\nstatus: [unterminated\n---\n"), 0o644))
			before := snapshotURAggregate(t, ws, candidate.ID)

			_, err = ClaimShipment(ctx, ws, candidate.ID)
			require.Error(t, err, "an indeterminate canonical active-slot scan must fail closed")
			requireURAggregateUnchanged(t, ws, before)
		})
	})

	t.Run("claim_and_unblock_serialize_across_boundaries", func(t *testing.T) {
		t.Run("workspace_handles", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			fixture := newURBlockedActiveFixture(t, ws)
			_, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
				Reason:    "prepare concurrent resume",
				BlockedBy: "serialization-harness",
			})
			require.NoError(t, err)
			competing, err := CreateShipment(ctx, ws, "concurrent claim competitor", nil)
			require.NoError(t, err)
			second, err := NewWorkspace(ctx, ws.RootPath)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, second.Close()) })

			start := make(chan struct{})
			results := make(chan error, 2)
			var workers sync.WaitGroup
			workers.Add(2)
			go func() {
				defer workers.Done()
				<-start
				_, unblockErr := UnblockShipment(ctx, ws, fixture.shipment.ID, UnblockOptions{
					Target:      ShipmentActive,
					Confirm:     true,
					UnblockedBy: "concurrent-resumer",
				})
				results <- unblockErr
			}()
			go func() {
				defer workers.Done()
				<-start
				_, claimErr := ClaimShipment(ctx, second, competing.ID)
				results <- claimErr
			}()
			close(start)
			workers.Wait()
			close(results)

			successes := 0
			for result := range results {
				if result == nil {
					successes++
				}
			}
			assert.Equal(t, 1, successes,
				"claim and unblock-to-active must share one cross-workspace active slot")

			active := 0
			for _, id := range []string{fixture.shipment.ID, competing.ID} {
				if loadURCanonicalArtifact(t, ws, id).Status == models.StatusActive {
					active++
				}
			}
			assert.Equal(t, 1, active, "cross-workspace serialization must leave exactly one active shipment")
		})

		t.Run("processes", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			fixture := newURBlockedActiveFixture(t, ws)
			_, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
				Reason:    "prepare cross-process resume",
				BlockedBy: "process-serialization-harness",
			})
			require.NoError(t, err)
			competing, err := CreateShipment(ctx, ws, "cross-process claim competitor", nil)
			require.NoError(t, err)

			instructionPath := filepath.Join(ws.RootPath, "ur1b-slot-instruction.json")
			instruction := ur1BSlotInstruction{
				Root:       ws.RootPath,
				ShipmentID: competing.ID,
				ReadyPath:  filepath.Join(ws.RootPath, "ur1b-slot-ready"),
				StartPath:  filepath.Join(ws.RootPath, "ur1b-slot-start"),
				ResultPath: filepath.Join(ws.RootPath, "ur1b-slot-result.json"),
			}
			instructionData, err := json.Marshal(instruction)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(instructionPath, instructionData, 0o644))

			cmd := exec.Command(os.Args[0], "-test.run=^TestShipmentBlockedActiveSlotSubprocessHelper$", "-test.count=1")
			cmd.Env = append(os.Environ(), ur1BSlotInstructionEnv+"="+instructionPath)
			require.NoError(t, cmd.Start())
			var finishOnce sync.Once
			var waitErr error
			finishChild := func(kill bool) {
				finishOnce.Do(func() {
					if kill {
						if killErr := cmd.Process.Kill(); killErr != nil {
							t.Logf("active-slot child cleanup kill: %v", killErr)
						}
					}
					waitErr = cmd.Wait()
				})
			}
			t.Cleanup(func() { finishChild(true) })

			deadline := time.Now().Add(10 * time.Second)
			for {
				if _, statErr := os.Stat(instruction.ReadyPath); statErr == nil {
					break
				} else if !os.IsNotExist(statErr) {
					require.NoError(t, statErr)
				}
				if time.Now().After(deadline) {
					require.FailNow(t, "active-slot subprocess readiness timed out")
				}
				time.Sleep(5 * time.Millisecond)
			}
			require.NoError(t, os.WriteFile(instruction.StartPath, []byte("start"), 0o644))
			_, unblockErr := UnblockShipment(ctx, ws, fixture.shipment.ID, UnblockOptions{
				Target:      ShipmentActive,
				Confirm:     true,
				UnblockedBy: "cross-process-resumer",
			})
			finishChild(false)
			require.NoError(t, waitErr)

			resultData, err := os.ReadFile(instruction.ResultPath)
			require.NoError(t, err)
			var childResult ur1BSlotResult
			require.NoError(t, json.Unmarshal(resultData, &childResult))
			successes := 0
			if unblockErr == nil {
				successes++
			}
			if childResult.Success {
				successes++
			}
			assert.Equal(t, 1, successes,
				"cross-process claim and unblock-to-active must permit exactly one winner; child error: %s",
				childResult.Error)

			active := 0
			for _, id := range []string{fixture.shipment.ID, competing.ID} {
				if loadURCanonicalArtifact(t, ws, id).Status == models.StatusActive {
					active++
				}
			}
			assert.Equal(t, 1, active, "cross-process serialization must leave exactly one canonical active shipment")
		})
	})
}
