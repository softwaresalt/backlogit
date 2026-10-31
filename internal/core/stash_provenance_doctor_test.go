package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// u2Write writes a minimal Markdown artifact (frontmatter body supplied by
// the caller) into the queue or archive directory of the workspace.
func u2Write(t *testing.T, ws *Workspace, dir, id, frontmatter string) string {
	t.Helper()
	d := filepath.Join(ws.StorageRoot, dir)
	require.NoError(t, os.MkdirAll(d, 0o755))
	p := filepath.Join(d, id+".md")
	require.NoError(t, os.WriteFile(p, []byte("---\n"+frontmatter+"---\nbody\n"), 0o644))
	return p
}

func u2Artifact(t *testing.T, ws *Workspace, id, artifactType, status, parent, extra string) {
	t.Helper()
	fm := fmt.Sprintf("id: %s\ntitle: t %s\nstatus: %s\nartifact_type: %s\n", id, id, status, artifactType)
	if parent != "" {
		fm += "parent_id: " + parent + "\n"
	}
	u2Write(t, ws, "queue", id, fm+extra)
}

func u2ShipmentFM(t *testing.T, ws *Workspace, id, status, extra string, items ...string) {
	t.Helper()
	list := "custom_fields:\n  items:\n"
	for _, it := range items {
		list += "    - " + it + "\n"
	}
	if len(items) == 0 {
		list = ""
	}
	u2Artifact(t, ws, id, "shipment", status, "", list+extra)
}

func u2ArchiveStash(t *testing.T, ws *Workspace, stashID, harvested, deliberation string) {
	t.Helper()
	dir := filepath.Join(ws.StorageRoot, "archive")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	entry := map[string]any{
		"id": stashID, "priority": "high", "kind": "bug", "text": "forced text " + stashID,
		"archived_at": "2026-01-01T00:00:00Z", "reason": "harvested", "harvested_artifact_id": harvested,
	}
	if deliberation != "" {
		entry["deliberation_id"] = deliberation
	}
	raw, err := json.Marshal(entry)
	require.NoError(t, err)
	f, err := os.OpenFile(filepath.Join(dir, "stash.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.Write(append(raw, '\n'))
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func u2CustomFields(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	cf, _ := fm["custom_fields"].(map[string]any)
	return cf
}

func u2ReadFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return raw
}

// u2StashFixture seeds a historical carrier (stale provenance) and a canonical
// target without any source_stash_id.
func u2StashFixture(t *testing.T) (*Workspace, string, string) {
	t.Helper()
	ws := setupShipmentWorkspace(t)
	u2ArchiveStash(t, ws, "ST0001", "001-F", "")
	u2Artifact(t, ws, "001-F", "feature", "active", "",
		"custom_fields:\n  source_stash_id: ST0001\n  source_stash_kind: feature\n  source_stash_text: stale\n  source_deliberation_id: D-STALE\n")
	u2Artifact(t, ws, "002-F", "feature", "active", "", "")
	return ws, filepath.Join(ws.StorageRoot, "queue", "002-F.md"), filepath.Join(ws.StorageRoot, "archive", "provenance_corrections.jsonl")
}

func u2ForceReq() StashProvenanceCorrectionRequest {
	return StashProvenanceCorrectionRequest{
		StashID: "ST0001", CanonicalDeliveryArtifactID: "002-F",
		Reason: "operator repoint", Actor: "op", Force: true,
	}
}

// TestU2_StashProvenanceForceAndDoctorManifestChecks is the U2 (208.002-T)
// harness for CorrectStashProvenance --force and Doctor manifest checks.
func TestU2_StashProvenanceForceAndDoctorManifestChecks(t *testing.T) {
	ctx := context.Background()

	t.Run("force_requires_flag_for_target_without_source_stash_id", func(t *testing.T) {
		ws, _, _ := u2StashFixture(t)
		req := u2ForceReq()
		req.Force = false
		_, err := CorrectStashProvenance(ctx, ws, req)
		require.ErrorIs(t, err, blerrors.ErrValidation)
	})

	t.Run("force_stamps_full_snapshot_and_records_forced", func(t *testing.T) {
		ws, target, corrections := u2StashFixture(t)
		res, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.NoError(t, err)
		assert.Equal(t, StashProvenanceCorrected, res.Outcome)
		assert.True(t, res.Forced)

		cf := u2CustomFields(t, target)
		assert.Equal(t, "ST0001", cf["source_stash_id"])
		assert.Equal(t, "high", cf["source_stash_priority"])
		assert.Equal(t, "bug", cf["source_stash_kind"])
		assert.Equal(t, "forced text ST0001", cf["source_stash_text"])
		assert.Equal(t, "stash.jsonl", cf["source_stash_path"])
		_, hasDelib := cf["source_deliberation_id"]
		assert.False(t, hasDelib, "empty archived deliberation must not be stamped")

		raw := u2ReadFile(t, corrections)
		var c ProvenanceCorrection
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &c))
		assert.True(t, c.Forced)
		assert.Equal(t, "002-F", c.CanonicalDeliveryArtifactID)

		evRaw := u2ReadFile(t, events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), "002-F"))
		assert.Contains(t, string(evRaw), "stash_provenance_corrected")
		assert.Contains(t, string(evRaw), `"forced":true`)

		var prio, kind, delib, itemID string
		require.NoError(t, ws.DB.QueryRow(`SELECT priority, kind, COALESCE(deliberation_id,'') FROM stash_entries WHERE stash_id='ST0001'`).Scan(&prio, &kind, &delib))
		assert.Equal(t, "high", prio)
		assert.Equal(t, "bug", kind)
		assert.Equal(t, "", delib)
		require.NoError(t, ws.DB.QueryRow(`SELECT item_id FROM stash_links WHERE stash_id='ST0001'`).Scan(&itemID))
		assert.Equal(t, "002-F", itemID)

		// Idempotent replay of the same forced request.
		again, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.NoError(t, err)
		assert.Equal(t, StashProvenanceNoOp, again.Outcome)
	})

	t.Run("force_sets_deliberation_when_archived_entry_has_one", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		u2ArchiveStash(t, ws, "ST0001", "001-F", "D-REAL")
		u2Artifact(t, ws, "001-F", "feature", "active", "", "custom_fields:\n  source_stash_id: ST0001\n")
		u2Artifact(t, ws, "002-F", "feature", "active", "", "custom_fields:\n  source_deliberation_id: D-OLD\n")
		_, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.NoError(t, err)
		cf := u2CustomFields(t, filepath.Join(ws.StorageRoot, "queue", "002-F.md"))
		assert.Equal(t, "D-REAL", cf["source_deliberation_id"])
	})

	t.Run("force_supersedes_prior_conflicting_correction", func(t *testing.T) {
		ws, target, _ := u2StashFixture(t)
		u2Artifact(t, ws, "003-F", "feature", "active", "", "custom_fields:\n  source_stash_id: ST0001\n")
		prior := u2ForceReq()
		prior.CanonicalDeliveryArtifactID = "003-F"
		prior.Force = false
		_, err := CorrectStashProvenance(ctx, ws, prior)
		require.NoError(t, err)

		nonForced := u2ForceReq()
		nonForced.Force = false
		_, err = CorrectStashProvenance(ctx, ws, nonForced)
		require.ErrorIs(t, err, blerrors.ErrValidation, "non-forced conflict must still be rejected")

		res, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.NoError(t, err)
		assert.Equal(t, StashProvenanceCorrected, res.Outcome)
		assert.Equal(t, "ST0001", u2CustomFields(t, target)["source_stash_id"])
	})

	t.Run("force_rejects_sole_carrier_of_another_stash", func(t *testing.T) {
		ws, target, corrections := u2StashFixture(t)
		u2Artifact(t, ws, "002-F", "feature", "active", "", "custom_fields:\n  source_stash_id: OTHER01\n")
		u2ArchiveStash(t, ws, "OTHER01", "002-F", "")
		before := u2ReadFile(t, target)
		_, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.ErrorIs(t, err, blerrors.ErrValidation)
		assert.Equal(t, before, u2ReadFile(t, target))
		assert.Nil(t, u2ReadFile(t, corrections))
	})

	t.Run("malformed_corrections_preflight_leaves_target_untouched", func(t *testing.T) {
		ws, target, corrections := u2StashFixture(t)
		require.NoError(t, os.WriteFile(corrections, []byte("{not json\n"), 0o644))
		before := u2ReadFile(t, target)
		_, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.Error(t, err)
		assert.Equal(t, before, u2ReadFile(t, target))
		assert.Equal(t, "{not json\n", string(u2ReadFile(t, corrections)))
	})

	t.Run("event_append_failure_leaves_ledger_and_target_untouched", func(t *testing.T) {
		ws, target, corrections := u2StashFixture(t)
		logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), "002-F")
		require.NoError(t, os.MkdirAll(logPath, 0o755)) // a directory blocks the append
		before := u2ReadFile(t, target)
		_, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.Error(t, err)
		assert.Equal(t, before, u2ReadFile(t, target))
		assert.Nil(t, u2ReadFile(t, corrections))
	})

	t.Run("target_persist_failure_rolls_back_ledger_and_markdown", func(t *testing.T) {
		ws, target, corrections := u2StashFixture(t)
		before := u2ReadFile(t, target)
		_, err := ws.DB.Exec(`DROP TABLE stash_links`)
		require.NoError(t, err)
		_, err = CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.Error(t, err)
		assert.Equal(t, before, u2ReadFile(t, target), "target .md must be restored")
		assert.Nil(t, u2ReadFile(t, corrections), "corrections ledger must be restored")
	})

	t.Run("lock_order_item_log_before_artifact_before_stash", func(t *testing.T) {
		ws, _, _ := u2StashFixture(t)
		var mu sync.Mutex
		var order []string
		prevB := artifactMutationLockBarrierHook
		artifactMutationLockBarrierHook = func(id string) {
			if id == "002-F" {
				mu.Lock()
				order = append(order, "B")
				mu.Unlock()
			}
		}
		t.Cleanup(func() { artifactMutationLockBarrierHook = prevB })
		restoreC := events.SetItemLogLockBarrierHookForTest(func(id string) {
			if id == "002-F" {
				mu.Lock()
				order = append(order, "C")
				mu.Unlock()
			}
		})
		t.Cleanup(restoreC)
		_, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.NoError(t, err)
		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, order)
		assert.Equal(t, "C", order[0], "item-log lock must be acquired before the artifact mutation lock: %v", order)
		bSeen := false
		for _, o := range order {
			if o == "B" {
				bSeen = true
			} else if bSeen {
				t.Fatalf("item-log lock requested while artifact lock held: %v", order)
			}
		}
		assert.True(t, bSeen)
	})

	t.Run("concurrent_force_calls_serialize", func(t *testing.T) {
		ws, target, _ := u2StashFixture(t)
		var wg sync.WaitGroup
		results := make([]*StashProvenanceCorrectionResult, 4)
		errs := make([]error, 4)
		for i := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i], errs[i] = CorrectStashProvenance(ctx, ws, u2ForceReq())
			}()
		}
		wg.Wait()
		corrected := 0
		for i := range results {
			require.NoError(t, errs[i])
			if results[i].Outcome == StashProvenanceCorrected {
				corrected++
			}
		}
		assert.Equal(t, 1, corrected, "exactly one call applies the correction; the rest are no-ops")
		assert.Equal(t, "ST0001", u2CustomFields(t, target)["source_stash_id"])
	})

	t.Run("rehydrate_replay_keeps_canonical_snapshot", func(t *testing.T) {
		ws, _, _ := u2StashFixture(t)
		_, err := CorrectStashProvenance(ctx, ws, u2ForceReq())
		require.NoError(t, err)
		_, err = bldb.Rehydrate(ctx, ws.StorageRoot, ws.DB)
		require.NoError(t, err)
		var prio, kind, text, path, delib, itemID string
		require.NoError(t, ws.DB.QueryRow(`SELECT priority, kind, text, source_path, COALESCE(deliberation_id,'') FROM stash_entries WHERE stash_id='ST0001'`).Scan(&prio, &kind, &text, &path, &delib))
		assert.Equal(t, "high", prio)
		assert.Equal(t, "bug", kind)
		assert.Equal(t, "forced text ST0001", text)
		assert.Equal(t, "stash.jsonl", path)
		assert.Equal(t, "", delib, "stale historical deliberation must be cleared")
		require.NoError(t, ws.DB.QueryRow(`SELECT item_id FROM stash_links WHERE stash_id='ST0001'`).Scan(&itemID))
		assert.Equal(t, "002-F", itemID)
	})

	t.Run("rehydrate_preserves_legacy_non_forced_correction", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		u2ArchiveStash(t, ws, "ST0001", "001-F", "")
		u2Artifact(t, ws, "001-F", "feature", "active", "",
			"custom_fields:\n  source_stash_id: ST0001\n  source_stash_kind: historic\n  source_stash_text: historic text\n")
		u2Artifact(t, ws, "002-F", "feature", "active", "", "custom_fields:\n  source_stash_id: ST0001\n")
		req := u2ForceReq()
		req.Force = false
		_, err := CorrectStashProvenance(ctx, ws, req)
		require.NoError(t, err)
		_, err = bldb.Rehydrate(ctx, ws.StorageRoot, ws.DB)
		require.NoError(t, err)
		var itemID string
		require.NoError(t, ws.DB.QueryRow(`SELECT item_id FROM stash_links WHERE stash_id='ST0001'`).Scan(&itemID))
		assert.Equal(t, "002-F", itemID)
	})

	t.Run("doctor_manifest_findings", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		// Dangling member.
		u2Artifact(t, ws, "010-F", "feature", "active", "", "")
		u2ShipmentFM(t, ws, "901-S", "queued", "", "010-F", "999-T")
		report, err := Doctor(ctx, ws, &DoctorOptions{})
		require.NoError(t, err)
		f := u2Find(report, FindingDanglingShipmentMember)
		require.Len(t, f, 1)
		assert.Equal(t, DoctorSeverityError, f[0].Severity)
		assert.Equal(t, "901-S", f[0].ArtifactID)
		assert.True(t, report.HasErrors())
		assert.Equal(t, "dangling_shipment_member", string(FindingDanglingShipmentMember))
	})

	t.Run("doctor_duplicate_membership", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		u2Artifact(t, ws, "010-F", "feature", "active", "", "")
		u2ShipmentFM(t, ws, "901-S", "queued", "labels:\n  - dag-root\n", "010-F")
		u2ShipmentFM(t, ws, "902-S", "active", "", "010-F")
		report, err := Doctor(ctx, ws, &DoctorOptions{})
		require.NoError(t, err)
		f := u2Find(report, FindingDuplicateShipmentMembership)
		require.NotEmpty(t, f)
		assert.Equal(t, DoctorSeverityError, f[0].Severity)
	})

	t.Run("doctor_split_feature_variants", func(t *testing.T) {
		cases := []struct {
			name string
			a, b string
		}{
			{"feature_and_task", "010-F", "010.001-T"},
			{"task_and_task", "010.001-T", "010.002-T"},
			{"task_and_subtask", "010.001-T", "010.002.001-ST"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ws := setupShipmentWorkspace(t)
				u2Artifact(t, ws, "010-F", "feature", "active", "", "")
				u2Artifact(t, ws, "010.001-T", "task", "queued", "010-F", "")
				u2Artifact(t, ws, "010.002-T", "task", "queued", "010-F", "")
				u2Artifact(t, ws, "010.002.001-ST", "subtask", "queued", "010.002-T", "")
				u2ShipmentFM(t, ws, "901-S", "queued", "labels:\n  - dag-root\n", tc.a)
				u2ShipmentFM(t, ws, "902-S", "queued", "dependencies:\n  - 901-S\n", tc.b)
				report, err := Doctor(ctx, ws, &DoctorOptions{})
				require.NoError(t, err)
				f := u2Find(report, FindingSplitFeatureAcrossShipments)
				require.NotEmpty(t, f, "split must be flagged")
				assert.Equal(t, DoctorSeverityError, f[0].Severity)
				assert.Empty(t, u2Find(report, FindingDanglingShipmentMember))
			})
		}
	})

	t.Run("doctor_same_feature_single_shipment_and_archived_ignored", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		u2Artifact(t, ws, "010-F", "feature", "active", "", "")
		u2Artifact(t, ws, "010.001-T", "task", "queued", "010-F", "")
		u2ShipmentFM(t, ws, "901-S", "queued", "", "010-F", "010.001-T")
		u2Write(t, ws, "archive", "902-S",
			"id: 902-S\ntitle: old\nstatus: archived\nartifact_type: shipment\ncustom_fields:\n  items:\n    - 010.001-T\n    - 777-T\n")
		report, err := Doctor(ctx, ws, &DoctorOptions{})
		require.NoError(t, err)
		for _, ft := range []DoctorFindingType{FindingSplitFeatureAcrossShipments, FindingDuplicateShipmentMembership, FindingDanglingShipmentMember} {
			assert.Empty(t, u2Find(report, ft), string(ft))
		}
	})

	t.Run("doctor_forward_shipment_dependency", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		u2Artifact(t, ws, "010-F", "feature", "active", "", "")
		u2Artifact(t, ws, "011-F", "feature", "active", "", "dependencies:\n  - 010-F\n")
		// S1 holds 011-F which depends on 010-F in S2, while S2 depends on S1.
		u2ShipmentFM(t, ws, "901-S", "queued", "labels:\n  - dag-root\n", "011-F")
		u2ShipmentFM(t, ws, "902-S", "queued", "dependencies:\n  - 901-S\n", "010-F")
		report, err := Doctor(ctx, ws, &DoctorOptions{})
		require.NoError(t, err)
		f := u2Find(report, FindingForwardShipmentDependency)
		require.NotEmpty(t, f)
		assert.Equal(t, DoctorSeverityError, f[0].Severity)
		assert.Equal(t, "forward_shipment_dependency", string(f[0].Type))
	})

	t.Run("doctor_unsequenced_queued_shipment_advisory", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		u2Artifact(t, ws, "010-F", "feature", "active", "", "")
		u2Artifact(t, ws, "011-F", "feature", "active", "", "")
		u2ShipmentFM(t, ws, "901-S", "queued", "", "010-F")
		report, err := Doctor(ctx, ws, &DoctorOptions{})
		require.NoError(t, err)
		assert.Empty(t, u2Find(report, FindingUnsequencedQueuedShipment), "single queued shipment is never flagged")

		u2ShipmentFM(t, ws, "902-S", "queued", "", "011-F")
		report, err = Doctor(ctx, ws, &DoctorOptions{})
		require.NoError(t, err)
		f := u2Find(report, FindingUnsequencedQueuedShipment)
		require.Len(t, f, 2)
		assert.Equal(t, DoctorFindingSeverity(""), f[0].Severity, "advisory")
		assert.False(t, report.HasErrors())

		u2ShipmentFM(t, ws, "901-S", "queued", "labels:\n  - dag-root\n", "010-F")
		u2ShipmentFM(t, ws, "902-S", "queued", "dependencies:\n  - 901-S\n", "011-F")
		report, err = Doctor(ctx, ws, &DoctorOptions{})
		require.NoError(t, err)
		assert.Empty(t, u2Find(report, FindingUnsequencedQueuedShipment))
		assert.False(t, report.HasErrors())
	})
}

func u2Find(report *DoctorReport, ft DoctorFindingType) []DoctorFinding {
	var out []DoctorFinding
	for _, f := range report.Findings {
		if f.Type == ft {
			out = append(out, f)
		}
	}
	return out
}
