package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

const ur3CrashInstructionEnv = "BACKLOGIT_UR3_CRASH_INSTRUCTION"
const ur10SubprocessInstructionEnv = "BACKLOGIT_UR10_SUBPROCESS_INSTRUCTION"

type ur3OperationIntent struct {
	SchemaVersion  string              `json:"schema_version"`
	CorrelationID  string              `json:"correlation_id"`
	Phase          string              `json:"phase"`
	Operation      string              `json:"operation"`
	RecoveryPolicy string              `json:"recovery_policy"`
	ShipmentID     string              `json:"shipment_id"`
	Target         string              `json:"target"`
	Reason         string              `json:"reason,omitempty"`
	BlockedBy      string              `json:"blocked_by,omitempty"`
	SnapshotRef    string              `json:"snapshot_ref,omitempty"`
	Preimage       urLifecyclePreimage `json:"preimage"`
}

type ur3CrashInstruction struct {
	Root          string             `json:"root"`
	ReadyPath     string             `json:"ready_path"`
	FailurePath   string             `json:"failure_path"`
	Intent        ur3OperationIntent `json:"intent"`
	TornArtifacts []*models.Artifact `json:"torn_artifacts"`
}

type ur3CrashPoint struct {
	JournalPath           string             `json:"journal_path"`
	Journal               ur3OperationIntent `json:"journal"`
	PersistedArtifactID   string             `json:"persisted_artifact_id"`
	PersistedArtifactPath string             `json:"persisted_artifact_path"`
	PersistedArtifact     *models.Artifact   `json:"persisted_artifact"`
	RealOperationWrite    bool               `json:"real_operation_write"`
}

type ur10SubprocessInstruction struct {
	Mode          string `json:"mode"`
	Root          string `json:"root"`
	ShipmentID    string `json:"shipment_id,omitempty"`
	ItemID        string `json:"item_id,omitempty"`
	ReadyPath     string `json:"ready_path,omitempty"`
	StartPath     string `json:"start_path,omitempty"`
	AttemptedPath string `json:"attempted_path,omitempty"`
	ContendedPath string `json:"contended_path,omitempty"`
	AcquiredPath  string `json:"acquired_path,omitempty"`
	ContinuePath  string `json:"continue_path,omitempty"`
	DonePath      string `json:"done_path"`
}

func setupUR3Workspace(t *testing.T) (string, *Workspace) {
	t.Helper()

	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))
	ws, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	return root, ws
}

func writeUR3Intent(t *testing.T, ws *Workspace, intent ur3OperationIntent) string {
	t.Helper()

	opsRoot := shipmentOpsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(opsRoot, 0o755))
	data, err := json.MarshalIndent(intent, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(opsRoot, shipmentLifecycleJournalName(intent.CorrelationID))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	require.NoError(t, err)
	require.NoError(t, file.Chmod(0o644))
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	require.NoError(t, writeErr)
	require.NoError(t, syncErr, "intent and complete preimage must be durable before torn mutations")
	require.NoError(t, closeErr)
	return path
}

func readUR3OperationIntent(t *testing.T, path string) ur3OperationIntent {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var intent ur3OperationIntent
	require.NoError(t, json.Unmarshal(data, &intent))
	return intent
}

func reopenUR3WorkspaceCleanly(t *testing.T, root string, current *Workspace) *Workspace {
	t.Helper()

	require.NoError(t, current.Close())
	reopened, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	return reopened
}

func requireUR3IntentResolved(t *testing.T, crash ur3CrashPoint) ur3OperationIntent {
	t.Helper()

	var wantPhase string
	switch crash.Journal.RecoveryPolicy {
	case "rollback":
		wantPhase = "compensated"
	case "roll_forward":
		wantPhase = "committed"
	default:
		require.FailNow(t, "unknown recovery policy",
			"journal %s has unsupported policy %q", crash.JournalPath, crash.Journal.RecoveryPolicy)
	}
	data, err := os.ReadFile(crash.JournalPath)
	require.NoError(t, err, "operation journal must be retained with an auditable terminal phase")
	var state ur3OperationIntent
	require.NoError(t, json.Unmarshal(data, &state))
	require.Equal(t, wantPhase, state.Phase, "terminal journal phase must follow recovery policy")
	require.Equal(t, crash.Journal.CorrelationID, state.CorrelationID,
		"recovery must retain the original actual-operation correlation")
	require.Equal(t, crash.Journal.Operation, state.Operation,
		"recovery must retain the original operation")
	require.Equal(t, crash.Journal.ShipmentID, state.ShipmentID,
		"recovery must retain the original shipment")
	require.Equal(t, crash.Journal.RecoveryPolicy, state.RecoveryPolicy,
		"recovery must retain the original policy")
	require.Equal(t, crash.Journal.Preimage, state.Preimage,
		"recovery must retain the complete original preimage")
	require.Equal(t, crash.Journal.Target, state.Target,
		"recovery must retain the original target")
	return state
}

func assertUR3ArtifactPreimage(t *testing.T, want, got *models.Artifact) {
	t.Helper()

	assertURArtifactEqual(t, want, got)
}

func requireUR3ActualOperationOutcome(
	t *testing.T,
	ws *Workspace,
	crash ur3CrashPoint,
) (*models.Artifact, map[string]*models.Artifact) {
	t.Helper()

	terminal := requireUR3IntentResolved(t, crash)
	shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, terminal.ShipmentID))
	members := make(map[string]*models.Artifact, len(terminal.Preimage.Members))
	for _, preimage := range terminal.Preimage.Members {
		members[preimage.ID] = cloneArtifact(loadURCanonicalArtifact(t, ws, preimage.ID))
	}

	switch terminal.Phase {
	case "compensated":
		require.Equal(t, artifactCodecViewUR(t, terminal.Preimage.Shipment), artifactCodecViewUR(t, shipment),
			"compensated recovery must restore the exact shipment preimage")
		for _, preimage := range terminal.Preimage.Members {
			require.Equal(t, artifactCodecViewUR(t, preimage), artifactCodecViewUR(t, members[preimage.ID]),
				"compensated recovery must restore the exact member preimage for %s", preimage.ID)
		}
	case "committed":
		switch terminal.Operation {
		case "block":
			requireUR3CommittedBlockOutcome(t, terminal, shipment, members)
		case "unblock":
			requireUR3CommittedUnblockOutcome(t, terminal, shipment, members)
		default:
			require.FailNow(t, "unsupported committed actual operation",
				"operation %q has no deterministic target-state assertion", terminal.Operation)
		}
	default:
		require.FailNow(t, "actual operation journal is not terminal",
			"journal %s retained phase %q", crash.JournalPath, terminal.Phase)
	}
	requireUR3CorrelatedTerminalEvidence(t, ws, crash.Journal, terminal)
	canonical := make([]*models.Artifact, 0, len(members)+1)
	canonical = append(canonical, shipment)
	for _, preimage := range terminal.Preimage.Members {
		canonical = append(canonical, members[preimage.ID])
	}
	requireUR3PostSyncConvergence(t, ws, canonical...)
	return shipment, members
}

func requireUR3CommittedBlockOutcome(
	t *testing.T,
	terminal ur3OperationIntent,
	shipment *models.Artifact,
	members map[string]*models.Artifact,
) {
	t.Helper()

	require.Equal(t, string(ShipmentBlocked), terminal.Target)
	require.Equal(t, models.StatusBlocked, shipment.Status)
	require.NotNil(t, shipment.CustomFields)

	blockedAt, found := shipment.CustomFields["blocked_at"]
	require.True(t, found, "committed block must retain blocked_at")
	switch value := blockedAt.(type) {
	case string:
		_, err := time.Parse(time.RFC3339, value)
		require.NoError(t, err, "committed block blocked_at must be RFC3339")
	case time.Time:
		require.False(t, value.IsZero(), "committed block blocked_at must be non-zero")
	default:
		require.FailNow(t, "committed block blocked_at has invalid type", "got %T", blockedAt)
	}

	memberStatuses := make(map[string]string, len(terminal.Preimage.Members))
	for _, preimage := range terminal.Preimage.Members {
		memberStatuses[preimage.ID] = string(preimage.Status)
	}
	wantShipment := cloneArtifact(terminal.Preimage.Shipment)
	wantShipment.Status = models.StatusBlocked
	wantShipment.UpdatedAt = shipment.UpdatedAt
	if wantShipment.CustomFields == nil {
		wantShipment.CustomFields = map[string]any{}
	}
	wantShipment.CustomFields["blocked_reason"] = terminal.Reason
	wantShipment.CustomFields["blocked_by"] = terminal.BlockedBy
	wantShipment.CustomFields["blocked_at"] = blockedAt
	wantShipment.CustomFields["member_status_snapshot"] = memberStatuses
	if terminal.SnapshotRef == "" {
		delete(wantShipment.CustomFields, "resume_checkpoint_ref")
	} else {
		wantShipment.CustomFields["resume_checkpoint_ref"] = terminal.SnapshotRef
	}
	require.Equal(t, artifactCodecViewUR(t, wantShipment), artifactCodecViewUR(t, shipment),
		"committed block must produce the complete target shipment and preserve unrelated metadata")

	for _, preimage := range terminal.Preimage.Members {
		got, found := members[preimage.ID]
		require.True(t, found, "committed block result missing member %s", preimage.ID)
		want := cloneArtifact(preimage)
		want.Status = models.StatusQueued
		want.UpdatedAt = got.UpdatedAt
		require.Equal(t, artifactCodecViewUR(t, want), artifactCodecViewUR(t, got),
			"committed block must queue member %s without losing unrelated state", preimage.ID)
	}
}

func requireUR3CommittedUnblockOutcome(
	t *testing.T,
	terminal ur3OperationIntent,
	shipment *models.Artifact,
	members map[string]*models.Artifact,
) {
	t.Helper()

	require.Contains(t, []string{string(ShipmentQueued), string(ShipmentActive)}, terminal.Target)
	wantShipment := cloneArtifact(terminal.Preimage.Shipment)
	wantShipment.Status = models.ArtifactStatus(terminal.Target)
	wantShipment.UpdatedAt = shipment.UpdatedAt
	for _, key := range []string{"blocked_reason", "blocked_at", "blocked_by"} {
		delete(wantShipment.CustomFields, key)
	}
	require.Equal(t, artifactCodecViewUR(t, wantShipment), artifactCodecViewUR(t, shipment),
		"committed unblock must produce the complete target shipment and preserve resumption evidence")

	snapshot := statusSnapshotUR(terminal.Preimage.Shipment.CustomFields)
	for _, preimage := range terminal.Preimage.Members {
		got, found := members[preimage.ID]
		require.True(t, found, "committed unblock result missing member %s", preimage.ID)
		wantStatus := models.StatusQueued
		if terminal.Target == string(ShipmentActive) {
			snapshotStatus, exists := snapshot[preimage.ID]
			require.True(t, exists, "active unblock snapshot missing member %s", preimage.ID)
			wantStatus = models.ArtifactStatus(snapshotStatus)
		}
		want := cloneArtifact(preimage)
		want.Status = wantStatus
		want.UpdatedAt = got.UpdatedAt
		require.Equal(t, artifactCodecViewUR(t, want), artifactCodecViewUR(t, got),
			"committed unblock must apply the exact target disposition to member %s", preimage.ID)
	}
}

func runUR3CrashSubprocess(
	t *testing.T,
	root string,
	ws *Workspace,
	intent ur3OperationIntent,
	tornArtifacts ...*models.Artifact,
) ur3CrashPoint {
	t.Helper()

	require.NoError(t, ws.Close(), "the setup owner must release SQLite before the crash subprocess opens it")
	instructionPath := filepath.Join(root, "ur3-crash-instruction.json")
	readyPath := filepath.Join(root, "ur3-crash-ready")
	failurePath := filepath.Join(root, "ur3-crash-failure")
	instruction := ur3CrashInstruction{
		Root:          root,
		ReadyPath:     readyPath,
		FailurePath:   failurePath,
		Intent:        intent,
		TornArtifacts: tornArtifacts,
	}
	data, err := json.Marshal(instruction)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(instructionPath, data, 0o644))

	cmd := exec.Command(os.Args[0], "-test.run=^TestShipmentBlockedRecoverySubprocessHelper$", "-test.count=1")
	cmd.Env = append(os.Environ(), ur3CrashInstructionEnv+"="+instructionPath)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Start())

	var stopOnce sync.Once
	var killErr error
	var waitErr error
	stopChild := func() {
		stopOnce.Do(func() {
			killErr = cmd.Process.Kill()
			waitErr = cmd.Wait()
		})
	}
	t.Cleanup(func() {
		stopChild()
		if killErr != nil {
			t.Logf("idempotent crash-child cleanup kill: %v", killErr)
		}
		if waitErr == nil {
			t.Log("idempotent crash-child cleanup observed a clean child exit")
		}
	})

	var crash ur3CrashPoint
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if readyData, readErr := os.ReadFile(readyPath); readErr == nil {
			require.NoError(t, json.Unmarshal(readyData, &crash))
			ready = true
			break
		} else if !os.IsNotExist(readErr) {
			require.NoError(t, readErr)
		}
		if failure, readErr := os.ReadFile(failurePath); readErr == nil {
			stopChild()
			require.FailNow(t, "missing actual-operation/failpoint behavior",
				"the child invoked real %s but returned before the successful operation-owned write failpoint: %s\n%s",
				intent.Operation, string(failure), output.String())
		} else if !os.IsNotExist(readErr) {
			require.NoError(t, readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		stopChild()
		require.FailNow(t, "missing actual-operation/failpoint behavior",
			"kill error: %v; wait error: %v; output:\n%s", killErr, waitErr, output.String())
	}

	stopChild()
	require.NoError(t, killErr, "test must terminate the operation owner without a clean Workspace.Close")
	require.Error(t, waitErr, "a killed crash subprocess must not report clean termination")
	require.NotEmpty(t, crash.JournalPath, "failpoint must communicate the actual operation journal path")
	require.NotEmpty(t, crash.Journal.CorrelationID, "actual operation journal correlation")
	require.Equal(t, intent.Operation, crash.Journal.Operation)
	require.Equal(t, intent.ShipmentID, crash.Journal.ShipmentID)
	if intent.SnapshotRef == "" {
		require.Contains(t, []string{"rollback", "roll_forward"}, crash.Journal.RecoveryPolicy,
			"actual operation must durably select a supported deterministic recovery policy")
	} else {
		require.Equal(t, intent.RecoveryPolicy, crash.Journal.RecoveryPolicy,
			"bootstrap must retain its explicit roll-forward policy")
	}
	assertUR3ArtifactPreimage(t, intent.Preimage.Shipment, crash.Journal.Preimage.Shipment)
	require.Len(t, crash.Journal.Preimage.Members, len(intent.Preimage.Members))
	actualMembers := make(map[string]*models.Artifact, len(crash.Journal.Preimage.Members))
	for _, member := range crash.Journal.Preimage.Members {
		actualMembers[member.ID] = member
	}
	for _, member := range intent.Preimage.Members {
		assertUR3ArtifactPreimage(t, member, actualMembers[member.ID])
	}

	persisted, _, parseErr := parseFile(crash.PersistedArtifactPath)
	require.NoError(t, parseErr, "the failpoint must follow a successful persisted artifact write")
	assertURArtifactEqual(t, crash.PersistedArtifact, persisted)
	preimageByID := make(map[string]*models.Artifact, len(intent.Preimage.Members)+1)
	preimageByID[intent.Preimage.Shipment.ID] = intent.Preimage.Shipment
	for _, member := range intent.Preimage.Members {
		preimageByID[member.ID] = member
	}
	require.NotEqual(t,
		artifactCodecViewUR(t, preimageByID[crash.PersistedArtifactID]),
		artifactCodecViewUR(t, persisted),
		"crash point must follow a discriminating persisted mutation, not a no-op write",
	)
	if intent.SnapshotRef == "" {
		require.True(t, crash.RealOperationWrite,
			"block/unblock crash point must be reached through the real operation-owned writer")
	}
	return crash
}

func discoverUR3OperationJournal(root, shipmentID, operation string) (string, ur3OperationIntent, error) {
	entries, err := os.ReadDir(shipmentOpsRoot(root))
	if err != nil {
		return "", ur3OperationIntent{}, fmt.Errorf("read operation journal directory: %w", err)
	}
	type match struct {
		path  string
		state ur3OperationIntent
	}
	matches := make([]match, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(shipmentOpsRoot(root), entry.Name())
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", ur3OperationIntent{}, fmt.Errorf("read operation journal %s: %w", path, readErr)
		}
		var state ur3OperationIntent
		if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
			continue
		}
		if state.ShipmentID == shipmentID && state.Operation == operation && state.Phase == "intent" {
			matches = append(matches, match{path: path, state: state})
		}
	}
	if len(matches) != 1 {
		return "", ur3OperationIntent{}, fmt.Errorf(
			"expected one actual %s journal for shipment %s, found %d",
			operation,
			shipmentID,
			len(matches),
		)
	}
	return matches[0].path, matches[0].state, nil
}

func requireUR3CorrelatedTerminalEvidence(
	t *testing.T,
	ws *Workspace,
	original ur3OperationIntent,
	terminal ur3OperationIntent,
) {
	t.Helper()

	require.Equal(t, original.CorrelationID, terminal.CorrelationID)
	require.Contains(t, []string{"committed", "compensated"}, terminal.Phase)
	require.NotNil(t, original.Preimage.Shipment, "original journal must retain the canonical shipment preimage")

	wantStatus := original.Target
	if terminal.Phase == "compensated" {
		wantStatus = string(original.Preimage.Shipment.Status)
	}
	itemEvents := readUREvents(t, ws, original.ShipmentID)
	terminalIndex := -1
	statusIndex := -1
	for index, event := range itemEvents {
		if exactEventCorrelationUR(event) != original.CorrelationID ||
			eventLifecycleOperationUR(event) != original.Operation {
			continue
		}
		if event.EventType == "shipment_status_changed" &&
			eventTargetUR(event) == wantStatus {
			statusIndex = index
		}
		if eventPhaseUR(event) == terminal.Phase {
			terminalIndex = index
		}
	}
	require.GreaterOrEqual(t, terminalIndex, 0,
		"append-only audit log must contain a terminal %s event tied to original intent correlation %s",
		terminal.Phase, original.CorrelationID)
	if terminal.Phase == "committed" {
		require.GreaterOrEqual(t, statusIndex, 0,
			"committed recovery must append authoritative shipment_status_changed evidence for target %s under correlation %s",
			original.Target, original.CorrelationID)
		require.LessOrEqual(t, statusIndex, terminalIndex,
			"correlated status evidence must not follow the terminal committed audit event")
		return
	}
	require.GreaterOrEqual(t, statusIndex, 0,
		"compensated recovery must append authoritative shipment_status_changed evidence for restored preimage status %s under correlation %s",
		wantStatus, original.CorrelationID)
	require.LessOrEqual(t, statusIndex, terminalIndex,
		"correlated restored-preimage status evidence must not follow the terminal compensated audit event")
}

func requireUR3PostSyncConvergence(
	t *testing.T,
	ws *Workspace,
	artifacts ...*models.Artifact,
) {
	t.Helper()

	_, err := bldb.Rehydrate(context.Background(), workspaceStorageRoot(ws), ws.DB)
	require.NoError(t, err, "explicit repository sync/rebuild must converge the disposable SQLite projection")
	for _, beforeSync := range artifacts {
		require.NotNil(t, beforeSync)
		canonical := loadURCanonicalArtifact(t, ws, beforeSync.ID)
		indexed, getErr := bldb.GetItem(context.Background(), ws.DB, beforeSync.ID)
		require.NoError(t, getErr)
		require.Equal(t, artifactCodecViewUR(t, canonical), artifactCodecViewUR(t, indexed),
			"post-sync SQLite row must converge to canonical Markdown for %s", beforeSync.ID)
	}
}

func parkUR3CrashChild() {
	for {
		time.Sleep(time.Hour)
	}
}

func TestShipmentBlockedRecoverySubprocessHelper(t *testing.T) {
	instructionPath := os.Getenv(ur3CrashInstructionEnv)
	if instructionPath == "" {
		t.Skip("subprocess-only crash helper")
	}

	data, err := os.ReadFile(instructionPath)
	require.NoError(t, err)
	var instruction ur3CrashInstruction
	require.NoError(t, json.Unmarshal(data, &instruction))
	ws, err := NewWorkspace(context.Background(), instruction.Root)
	require.NoError(t, err)

	if instruction.Intent.SnapshotRef != "" {
		// Branch bootstrap recovery consumes the specified durable snapshot as
		// its real machine-readable input; unlike block/unblock, it is not
		// represented by a public lifecycle operation in this wave.
		writeUR3Intent(t, ws, instruction.Intent)
		for _, artifact := range instruction.TornArtifacts {
			forceURArtifactFixture(t, ws, artifact)
		}
		require.NotEmpty(t, instruction.TornArtifacts)
		journalPath, journal, discoverErr := discoverUR3OperationJournal(
			instruction.Root,
			instruction.Intent.ShipmentID,
			instruction.Intent.Operation,
		)
		require.NoError(t, discoverErr)
		persisted := instruction.TornArtifacts[0]
		persistedPath, pathErr := FindArtifactPath(context.Background(), ws, persisted.ID)
		require.NoError(t, pathErr)
		crash := ur3CrashPoint{
			JournalPath:           journalPath,
			Journal:               journal,
			PersistedArtifactID:   persisted.ID,
			PersistedArtifactPath: persistedPath,
			PersistedArtifact:     persisted,
		}
		readyData, marshalErr := json.Marshal(crash)
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(instruction.ReadyPath, readyData, 0o644))
		parkUR3CrashChild()
	}

	var signalOnce sync.Once
	realWrite := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, path string, durable bool) error {
		if writeErr := realWrite(artifact, path, durable); writeErr != nil {
			return writeErr
		}
		var hookErr error
		signalOnce.Do(func() {
			journalPath, journal, discoverErr := discoverUR3OperationJournal(
				instruction.Root,
				instruction.Intent.ShipmentID,
				instruction.Intent.Operation,
			)
			if discoverErr != nil {
				hookErr = discoverErr
				return
			}
			crash := ur3CrashPoint{
				JournalPath:           journalPath,
				Journal:               journal,
				PersistedArtifactID:   artifact.ID,
				PersistedArtifactPath: path,
				PersistedArtifact:     cloneArtifact(artifact),
				RealOperationWrite:    true,
			}
			readyData, marshalErr := json.Marshal(crash)
			if marshalErr != nil {
				hookErr = fmt.Errorf("marshal crash point: %w", marshalErr)
				return
			}
			if writeErr := os.WriteFile(instruction.ReadyPath, readyData, 0o644); writeErr != nil {
				hookErr = fmt.Errorf("write crash point: %w", writeErr)
				return
			}
			parkUR3CrashChild()
		})
		return hookErr
	}
	var operationErr error
	switch instruction.Intent.Operation {
	case "block":
		_, operationErr = BlockShipment(context.Background(), ws, instruction.Intent.ShipmentID, BlockOptions{
			Reason:              instruction.Intent.Reason,
			BlockedBy:           instruction.Intent.BlockedBy,
			ResumeCheckpointRef: instruction.Intent.SnapshotRef,
		})
	case "unblock":
		_, operationErr = UnblockShipment(context.Background(), ws, instruction.Intent.ShipmentID, UnblockOptions{
			Target:      ShipmentStatus(instruction.Intent.Target),
			Confirm:     true,
			UnblockedBy: instruction.Intent.BlockedBy,
		})
	default:
		operationErr = fmt.Errorf("unsupported actual lifecycle operation %q", instruction.Intent.Operation)
	}
	persistArtifactWriteFn = realWrite
	failure := fmt.Sprintf(
		"real %s returned before a successful operation-owned persisted artifact write; error=%v",
		instruction.Intent.Operation,
		operationErr,
	)
	require.NoError(t, os.WriteFile(instruction.FailurePath, []byte(failure), 0o644))
	parkUR3CrashChild()
}

func TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage(t *testing.T) {
	root, ws := setupUR3Workspace(t)
	fixture := newURBlockedActiveFixture(t, ws)
	shipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
	shipmentPreimage.CustomFields["codec_integer"] = 7
	shipmentPreimage.CustomFields["codec_list"] = []string{"alpha", "beta"}
	forceURArtifactFixture(t, ws, shipmentPreimage)
	shipmentPreimage = cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))

	memberPreimages := make([]*models.Artifact, 0, len(fixture.members))
	for index, member := range fixture.members {
		current := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
		if current.CustomFields == nil {
			current.CustomFields = map[string]any{}
		}
		current.CustomFields["codec_nested"] = map[string]any{"member": index, "enabled": true}
		forceURArtifactFixture(t, ws, current)
		memberPreimages = append(memberPreimages, cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID)))
	}

	intent := ur3OperationIntent{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  "11111111111111111111111111111111",
		Phase:          "intent",
		Operation:      "block",
		RecoveryPolicy: "rollback",
		ShipmentID:     shipmentPreimage.ID,
		Target:         string(ShipmentBlocked),
		Reason:         "crash during member disposition",
		BlockedBy:      "ur3-harness",
		Preimage: urLifecyclePreimage{
			Shipment: shipmentPreimage,
			Members:  memberPreimages,
		},
	}
	crash := runUR3CrashSubprocess(t, root, ws, intent)

	reopened, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err, "workspace startup must recover a journal left by a killed process")
	defer func() { require.NoError(t, reopened.Close()) }()
	recoveredShipment, recoveredMembers := requireUR3ActualOperationOutcome(t, reopened, crash)

	reopenedAgain := reopenUR3WorkspaceCleanly(t, root, reopened)
	reopened = reopenedAgain
	assertURArtifactEqual(t, recoveredShipment, loadURCanonicalArtifact(t, reopenedAgain, shipmentPreimage.ID))
	for _, member := range memberPreimages {
		assertURArtifactEqual(t, recoveredMembers[member.ID], loadURCanonicalArtifact(t, reopenedAgain, member.ID))
	}
	requireUR3IntentResolved(t, crash)

	t.Run("already_open_governed_operation_invokes_pending_recovery", func(t *testing.T) {
		_, openWorkspace := setupUR3Workspace(t)
		t.Cleanup(func() { require.NoError(t, openWorkspace.Close()) })
		interrupted := newURBlockedActiveFixture(t, openWorkspace)
		openShipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, openWorkspace, interrupted.shipment.ID))
		openMemberPreimages := make([]*models.Artifact, 0, len(interrupted.members))
		for _, member := range interrupted.members {
			openMemberPreimages = append(openMemberPreimages,
				cloneArtifact(loadURCanonicalArtifact(t, openWorkspace, member.ID)))
		}
		pending := ur3OperationIntent{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  "22222222222222222222222222222222",
			Phase:          "intent",
			Operation:      "block",
			RecoveryPolicy: "rollback",
			ShipmentID:     openShipmentPreimage.ID,
			Target:         string(ShipmentBlocked),
			Reason:         "interrupted before governed claim",
			Preimage: urLifecyclePreimage{
				Shipment: openShipmentPreimage,
				Members:  openMemberPreimages,
			},
		}
		openIntentPath := writeUR3Intent(t, openWorkspace, pending)
		openCrash := ur3CrashPoint{
			JournalPath: openIntentPath,
			Journal:     readUR3OperationIntent(t, openIntentPath),
		}
		tornMember := cloneArtifact(openMemberPreimages[0])
		tornMember.Status = models.StatusQueued
		tornMember.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, openWorkspace, tornMember)

		candidate, err := CreateShipment(context.Background(), openWorkspace, "claim after pending recovery", nil)
		require.NoError(t, err)
		candidatePreimage := cloneArtifact(loadURCanonicalArtifact(t, openWorkspace, candidate.ID))
		candidateEventsPreimage := readUREvents(t, openWorkspace, candidate.ID)

		_, err = ClaimShipment(context.Background(), openWorkspace, candidate.ID)
		require.Error(t, err,
			"governed ClaimShipment must invoke pending recovery before checking the canonical active slot")

		assertURArtifactEqual(t, openShipmentPreimage,
			loadURCanonicalArtifact(t, openWorkspace, openShipmentPreimage.ID))
		for _, member := range openMemberPreimages {
			assertURArtifactEqual(t, member, loadURCanonicalArtifact(t, openWorkspace, member.ID))
		}
		assertURArtifactEqual(t, candidatePreimage,
			loadURCanonicalArtifact(t, openWorkspace, candidate.ID))
		require.Equal(t, candidateEventsPreimage, readUREvents(t, openWorkspace, candidate.ID),
			"recovery-before-refusal must not mutate the candidate event log")
		terminal := requireUR3IntentResolved(t, openCrash)
		requireUR3CorrelatedTerminalEvidence(t, openWorkspace, openCrash.Journal, terminal)
		requireUR3PostSyncConvergence(
			t,
			openWorkspace,
			append([]*models.Artifact{openShipmentPreimage, candidatePreimage}, openMemberPreimages...)...,
		)
	})
}

func TestUR3_ReopenRollsBackInterruptedUnblockAndRestoresExactBlockedPreimage(t *testing.T) {
	root, ws := setupUR3Workspace(t)
	fixture := newURBlockedActiveFixture(t, ws)

	shipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
	shipmentPreimage.Status = models.StatusBlocked
	shipmentPreimage.CustomFields["blocked_reason"] = "external dependency"
	shipmentPreimage.CustomFields["blocked_at"] = "2026-09-21T00:00:00Z"
	shipmentPreimage.CustomFields["blocked_by"] = "ur3-harness"
	shipmentPreimage.CustomFields["resume_checkpoint_ref"] = "checkpoint-before-unblock.json"
	memberSnapshot := make(map[string]string, len(fixture.members))
	memberPreimages := make([]*models.Artifact, 0, len(fixture.members))
	for _, member := range fixture.members {
		current := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
		memberSnapshot[current.ID] = string(current.Status)
		current.Status = models.StatusQueued
		forceURArtifactFixture(t, ws, current)
		memberPreimages = append(memberPreimages, cloneArtifact(loadURCanonicalArtifact(t, ws, current.ID)))
	}
	shipmentPreimage.CustomFields["member_status_snapshot"] = memberSnapshot
	shipmentPreimage.CustomFields["codec_integer"] = 9
	forceURArtifactFixture(t, ws, shipmentPreimage)
	shipmentPreimage = cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))

	intent := ur3OperationIntent{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  "33333333333333333333333333333333",
		Phase:          "intent",
		Operation:      "unblock",
		RecoveryPolicy: "rollback",
		ShipmentID:     shipmentPreimage.ID,
		Target:         string(ShipmentActive),
		Preimage: urLifecyclePreimage{
			Shipment: shipmentPreimage,
			Members:  memberPreimages,
		},
	}
	crash := runUR3CrashSubprocess(t, root, ws, intent)

	reopened, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err, "workspace startup must recover interrupted unblock after process kill")
	defer func() { require.NoError(t, reopened.Close()) }()
	recoveredShipment, recoveredMembers := requireUR3ActualOperationOutcome(t, reopened, crash)

	reopenedAgain := reopenUR3WorkspaceCleanly(t, root, reopened)
	reopened = reopenedAgain
	assertURArtifactEqual(t, recoveredShipment, loadURCanonicalArtifact(t, reopenedAgain, shipmentPreimage.ID))
	for _, member := range memberPreimages {
		assertURArtifactEqual(t, recoveredMembers[member.ID], loadURCanonicalArtifact(t, reopenedAgain, member.ID))
	}
	requireUR3IntentResolved(t, crash)
}

func TestUR3_BranchBootstrapReopenRollsForwardUsingMachineReadableSnapshot(t *testing.T) {
	root, ws := setupUR3Workspace(t)
	fixture := newURBlockedActiveFixture(t, ws)
	shipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
	shipmentPreimage.CustomFields["branch"] = "feat/preserved-ship-branch"
	shipmentPreimage.CustomFields["codec_integer"] = 11
	forceURArtifactFixture(t, ws, shipmentPreimage)
	shipmentPreimage = cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))

	memberPreimages := make([]*models.Artifact, 0, len(fixture.members))
	memberStatuses := make(map[string]string, len(fixture.members))
	for _, member := range fixture.members {
		current := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
		memberPreimages = append(memberPreimages, current)
		memberStatuses[current.ID] = string(current.Status)
	}

	snapshotRel := filepath.Join("bootstrap", fixture.shipment.ID+".snapshot.json")
	snapshotPath := filepath.Join(workspaceStorageRoot(ws), snapshotRel)
	require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
	snapshot := map[string]any{
		"schema_version":        "shipment-bootstrap-snapshot/v1",
		"shipment_id":           fixture.shipment.ID,
		"branch":                "feat/preserved-ship-branch",
		"target":                string(ShipmentBlocked),
		"blocked_reason":        "snapshot-authoritative branch bootstrap",
		"blocked_at":            "2026-09-21T00:00:00Z",
		"blocked_by":            "snapshot-bootstrap-operator",
		"resume_checkpoint_ref": "checkpoint-bootstrap.json",
		"members":               memberStatuses,
	}
	snapshotData, err := json.MarshalIndent(snapshot, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(snapshotPath, snapshotData, 0o644))

	intent := ur3OperationIntent{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  "44444444444444444444444444444444",
		Phase:          "intent",
		Operation:      "block",
		RecoveryPolicy: "roll_forward",
		ShipmentID:     shipmentPreimage.ID,
		Target:         string(ShipmentBlocked),
		Reason:         "stale journal fallback must not override snapshot",
		BlockedBy:      "stale-journal-operator",
		SnapshotRef:    snapshotRel,
		Preimage: urLifecyclePreimage{
			Shipment: shipmentPreimage,
			Members:  memberPreimages,
		},
	}
	tornShipment := cloneArtifact(shipmentPreimage)
	tornShipment.Status = models.StatusBlocked
	delete(tornShipment.CustomFields, "branch")
	tornShipment.CustomFields["blocked_reason"] = "inconsistent partial bootstrap"
	tornShipment.CustomFields["blocked_by"] = "torn-writer"
	tornShipment.CustomFields["resume_checkpoint_ref"] = "wrong-checkpoint.json"
	tornShipment.CustomFields["member_status_snapshot"] = map[string]string{
		memberPreimages[0].ID: string(models.StatusActive),
	}
	tornShipment.UpdatedAt = models.NowUTC()
	tornMember := cloneArtifact(memberPreimages[0])
	tornMember.Status = models.StatusQueued
	tornMember.UpdatedAt = models.NowUTC()
	tornArtifacts := []*models.Artifact{tornShipment, tornMember}
	crash := runUR3CrashSubprocess(t, root, ws, intent, tornArtifacts...)

	reopened, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err, "workspace startup must roll forward bootstrap state left by killed process")
	defer func() { require.NoError(t, reopened.Close()) }()
	recoveredShipment := loadURCanonicalArtifact(t, reopened, shipmentPreimage.ID)
	require.NotEqual(t, artifactCodecViewUR(t, tornShipment), artifactCodecViewUR(t, recoveredShipment),
		"roll-forward must reconstruct the incomplete torn shipment rather than accept it")
	require.Equal(t, models.StatusBlocked, recoveredShipment.Status)
	require.Equal(t, "feat/preserved-ship-branch", recoveredShipment.CustomFields["branch"])
	require.Equal(t, "snapshot-authoritative branch bootstrap", recoveredShipment.CustomFields["blocked_reason"])
	require.Equal(t, "2026-09-21T00:00:00Z", recoveredShipment.CustomFields["blocked_at"])
	require.Equal(t, "snapshot-bootstrap-operator", recoveredShipment.CustomFields["blocked_by"])
	require.Equal(t, "checkpoint-bootstrap.json", recoveredShipment.CustomFields["resume_checkpoint_ref"])
	require.Equal(t, memberStatuses, statusSnapshotUR(recoveredShipment.CustomFields))
	require.EqualValues(t, 11, recoveredShipment.CustomFields["codec_integer"])
	recoveredMembers := make(map[string]*models.Artifact, len(memberPreimages))
	for _, member := range memberPreimages {
		recoveredMember := loadURCanonicalArtifact(t, reopened, member.ID)
		require.Equal(t, models.StatusQueued, recoveredMember.Status,
			"snapshot member disposition must reconstruct incomplete member %s", member.ID)
		recoveredMembers[member.ID] = cloneArtifact(recoveredMember)
	}
	gotSnapshot, err := os.ReadFile(snapshotPath)
	require.NoError(t, err)
	var normalizedWantSnapshot map[string]any
	var normalizedGotSnapshot map[string]any
	require.NoError(t, json.Unmarshal(snapshotData, &normalizedWantSnapshot))
	require.NoError(t, json.Unmarshal(gotSnapshot, &normalizedGotSnapshot))
	require.Equal(t, normalizedWantSnapshot, normalizedGotSnapshot,
		"bootstrap recovery must preserve the exact normalized authoritative snapshot")
	terminal := requireUR3IntentResolved(t, crash)
	require.Equal(t, "committed", terminal.Phase, "bootstrap recovery is strict roll-forward")
	requireUR3CorrelatedTerminalEvidence(t, reopened, crash.Journal, terminal)
	canonical := make([]*models.Artifact, 0, len(recoveredMembers)+1)
	canonical = append(canonical, recoveredShipment)
	for _, member := range memberPreimages {
		canonical = append(canonical, recoveredMembers[member.ID])
	}
	requireUR3PostSyncConvergence(t, reopened, canonical...)

	reopenedAgain := reopenUR3WorkspaceCleanly(t, root, reopened)
	reopened = reopenedAgain
	assertURArtifactEqual(t, recoveredShipment, loadURCanonicalArtifact(t, reopenedAgain, shipmentPreimage.ID))
	for _, member := range memberPreimages {
		assertURArtifactEqual(t, recoveredMembers[member.ID], loadURCanonicalArtifact(t, reopenedAgain, member.ID))
	}
}

func TestUR10_SubprocessCrashReopenRecovery(t *testing.T) {
	t.Run("crash_during_block_recovers_under_contended_workspace_global_lock", func(t *testing.T) {
		root, ws := setupUR3Workspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		shipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
		shipmentPreimage.CustomFields["codec_integer"] = 17
		forceURArtifactFixture(t, ws, shipmentPreimage)
		shipmentPreimage = cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))

		memberPreimages := make([]*models.Artifact, 0, len(fixture.members))
		for index, member := range fixture.members {
			current := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
			if current.CustomFields == nil {
				current.CustomFields = map[string]any{}
			}
			current.CustomFields["r10_member"] = index
			forceURArtifactFixture(t, ws, current)
			memberPreimages = append(memberPreimages, cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID)))
		}

		candidate, err := CreateShipment(context.Background(), ws, "R10 competing shipment", nil)
		require.NoError(t, err)
		competingFeature, err := CreateArtifact(context.Background(), ws, "R10 competing feature", "feature")
		require.NoError(t, err)
		competingItem, err := CreateArtifact(
			context.Background(),
			ws,
			"R10 competing member",
			"task",
			WithParent(competingFeature.ID),
		)
		require.NoError(t, err)

		competitorInstruction := newUR10Instruction(root, "competitor")
		competitorInstruction.ShipmentID = candidate.ID
		competitorInstruction.ItemID = competingItem.ID
		competitor := startUR10Subprocess(t, competitorInstruction)
		waitUR10Marker(t, competitorInstruction.ReadyPath, "competitor workspace ready")

		intent := ur3OperationIntent{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  "55555555555555555555555555555555",
			Phase:          "intent",
			Operation:      "block",
			RecoveryPolicy: "rollback",
			ShipmentID:     shipmentPreimage.ID,
			Target:         string(ShipmentBlocked),
			Reason:         "R10 crash during block",
			BlockedBy:      "r10-harness",
			Preimage: urLifecyclePreimage{
				Shipment: shipmentPreimage,
				Members:  memberPreimages,
			},
		}
		crash := runUR3CrashSubprocess(t, root, ws, intent)

		recoveryInstruction := newUR10Instruction(root, "recovery")
		recovery := startUR10Subprocess(t, recoveryInstruction)
		waitUR10Marker(t, recoveryInstruction.AcquiredPath, "recovery global lock acquired")

		writeUR10Marker(t, competitorInstruction.StartPath)
		waitUR10Marker(t, competitorInstruction.AttemptedPath, "competing governed operation attempted global lock")
		waitUR10Marker(t, competitorInstruction.ContendedPath, "competing governed operation observed lock contention")
		requireUR10MarkerAbsent(t, competitorInstruction.AcquiredPath,
			"competing governed operation acquired the workspace-global lock before recovery released it")

		writeUR10Marker(t, recoveryInstruction.ContinuePath)
		waitUR10Marker(t, recoveryInstruction.DonePath, "recovery subprocess completed")
		require.NoError(t, recovery.wait(), recovery.outputString())
		waitUR10Marker(t, competitorInstruction.AcquiredPath,
			"competing governed operation acquired the released global lock")
		waitUR10Marker(t, competitorInstruction.DonePath, "competing governed operation completed")
		require.NoError(t, competitor.wait(), competitor.outputString())

		reopened, err := NewWorkspace(context.Background(), root)
		require.NoError(t, err)
		defer func() { require.NoError(t, reopened.Close()) }()
		requireUR3ActualOperationOutcome(t, reopened, crash)

		candidateCanonical := loadURCanonicalArtifact(t, reopened, candidate.ID)
		require.Equal(t, []string{competingItem.ID}, NormalizeShipmentItems(candidateCanonical))
		requireUR3PostSyncConvergence(
			t,
			reopened,
			candidateCanonical,
			loadURCanonicalArtifact(t, reopened, competingItem.ID),
		)
	})

	t.Run("crash_during_unblock_restores_exact_blocked_preimage", func(t *testing.T) {
		root, ws := setupUR3Workspace(t)
		fixture := newURBlockedActiveFixture(t, ws)

		shipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
		shipmentPreimage.Status = models.StatusBlocked
		shipmentPreimage.CustomFields["blocked_reason"] = "R10 external dependency"
		shipmentPreimage.CustomFields["blocked_at"] = "2026-09-22T00:00:00Z"
		shipmentPreimage.CustomFields["blocked_by"] = "r10-harness"
		shipmentPreimage.CustomFields["resume_checkpoint_ref"] = "r10-before-unblock.json"
		memberSnapshot := make(map[string]string, len(fixture.members))
		memberPreimages := make([]*models.Artifact, 0, len(fixture.members))
		for _, member := range fixture.members {
			current := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
			memberSnapshot[current.ID] = string(current.Status)
			current.Status = models.StatusQueued
			forceURArtifactFixture(t, ws, current)
			memberPreimages = append(memberPreimages, cloneArtifact(loadURCanonicalArtifact(t, ws, current.ID)))
		}
		shipmentPreimage.CustomFields["member_status_snapshot"] = memberSnapshot
		shipmentPreimage.CustomFields["codec_integer"] = 19
		forceURArtifactFixture(t, ws, shipmentPreimage)
		shipmentPreimage = cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))

		intent := ur3OperationIntent{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  "66666666666666666666666666666666",
			Phase:          "intent",
			Operation:      "unblock",
			RecoveryPolicy: "rollback",
			ShipmentID:     shipmentPreimage.ID,
			Target:         string(ShipmentActive),
			BlockedBy:      "r10-harness",
			Preimage: urLifecyclePreimage{
				Shipment: shipmentPreimage,
				Members:  memberPreimages,
			},
		}
		crash := runUR3CrashSubprocess(t, root, ws, intent)
		require.Equal(t, "rollback", crash.Journal.RecoveryPolicy)

		runUR10RecoverySubprocess(t, root)

		reopened, err := NewWorkspace(context.Background(), root)
		require.NoError(t, err)
		defer func() { require.NoError(t, reopened.Close()) }()
		recoveredShipment, recoveredMembers := requireUR3ActualOperationOutcome(t, reopened, crash)
		require.Equal(t, "compensated", requireUR3IntentResolved(t, crash).Phase)
		assertURArtifactEqual(t, shipmentPreimage, recoveredShipment)
		for _, member := range memberPreimages {
			assertURArtifactEqual(t, member, recoveredMembers[member.ID])
		}
	})

	t.Run("crash_during_branch_bootstrap_recovers_from_intent_and_snapshot", func(t *testing.T) {
		root, ws := setupUR3Workspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		shipmentPreimage := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
		shipmentPreimage.CustomFields["branch"] = "feat/r10-bootstrap"
		shipmentPreimage.CustomFields["codec_integer"] = 23
		forceURArtifactFixture(t, ws, shipmentPreimage)
		shipmentPreimage = cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))

		memberPreimages := make([]*models.Artifact, 0, len(fixture.members))
		memberStatuses := make(map[string]string, len(fixture.members))
		for _, member := range fixture.members {
			current := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
			memberPreimages = append(memberPreimages, current)
			memberStatuses[current.ID] = string(current.Status)
		}

		snapshotRel := filepath.Join("bootstrap", fixture.shipment.ID+".r10.snapshot.json")
		snapshotPath := filepath.Join(workspaceStorageRoot(ws), snapshotRel)
		require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
		snapshot := map[string]any{
			"schema_version":        ShipmentBlockedSnapshotSchemaVersion,
			"shipment_id":           fixture.shipment.ID,
			"branch":                "feat/r10-bootstrap",
			"target":                string(ShipmentBlocked),
			"blocked_reason":        "R10 snapshot-authoritative bootstrap",
			"blocked_at":            "2026-09-22T00:00:00Z",
			"blocked_by":            "r10-bootstrap-operator",
			"resume_checkpoint_ref": "r10-bootstrap-checkpoint.json",
			"members":               memberStatuses,
		}
		snapshotData, err := json.MarshalIndent(snapshot, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(snapshotPath, snapshotData, 0o644))

		intent := ur3OperationIntent{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  "77777777777777777777777777777777",
			Phase:          "intent",
			Operation:      "block",
			RecoveryPolicy: "roll_forward",
			ShipmentID:     shipmentPreimage.ID,
			Target:         string(ShipmentBlocked),
			Reason:         "journal value must not override snapshot",
			BlockedBy:      "stale-r10-operator",
			SnapshotRef:    snapshotRel,
			Preimage: urLifecyclePreimage{
				Shipment: shipmentPreimage,
				Members:  memberPreimages,
			},
		}
		tornShipment := cloneArtifact(shipmentPreimage)
		tornShipment.Status = models.StatusBlocked
		delete(tornShipment.CustomFields, "branch")
		tornShipment.CustomFields["blocked_reason"] = "R10 torn bootstrap"
		tornShipment.CustomFields["blocked_by"] = "r10-torn-writer"
		tornShipment.CustomFields["resume_checkpoint_ref"] = "wrong-r10-checkpoint.json"
		tornShipment.CustomFields["member_status_snapshot"] = map[string]string{
			memberPreimages[0].ID: string(models.StatusActive),
		}
		tornShipment.UpdatedAt = models.NowUTC()
		tornMember := cloneArtifact(memberPreimages[0])
		tornMember.Status = models.StatusQueued
		tornMember.UpdatedAt = models.NowUTC()
		crash := runUR3CrashSubprocess(t, root, ws, intent, tornShipment, tornMember)

		runUR10RecoverySubprocess(t, root)

		reopened, err := NewWorkspace(context.Background(), root)
		require.NoError(t, err)
		defer func() { require.NoError(t, reopened.Close()) }()
		recoveredShipment := loadURCanonicalArtifact(t, reopened, shipmentPreimage.ID)
		require.Equal(t, models.StatusBlocked, recoveredShipment.Status)
		require.Equal(t, "feat/r10-bootstrap", recoveredShipment.CustomFields["branch"])
		require.Equal(t, "R10 snapshot-authoritative bootstrap", recoveredShipment.CustomFields["blocked_reason"])
		require.Equal(t, "2026-09-22T00:00:00Z", recoveredShipment.CustomFields["blocked_at"])
		require.Equal(t, "r10-bootstrap-operator", recoveredShipment.CustomFields["blocked_by"])
		require.Equal(t, "r10-bootstrap-checkpoint.json", recoveredShipment.CustomFields["resume_checkpoint_ref"])
		require.Equal(t, memberStatuses, statusSnapshotUR(recoveredShipment.CustomFields))
		require.EqualValues(t, 23, recoveredShipment.CustomFields["codec_integer"])

		recoveredMembers := make(map[string]*models.Artifact, len(memberPreimages))
		for _, member := range memberPreimages {
			recovered := loadURCanonicalArtifact(t, reopened, member.ID)
			require.Equal(t, models.StatusQueued, recovered.Status)
			recoveredMembers[member.ID] = cloneArtifact(recovered)
		}
		gotSnapshot, err := os.ReadFile(snapshotPath)
		require.NoError(t, err)
		require.JSONEq(t, string(snapshotData), string(gotSnapshot),
			"bootstrap recovery must preserve its exact machine-readable snapshot")
		terminal := requireUR3IntentResolved(t, crash)
		require.Equal(t, "committed", terminal.Phase)
		requireUR3CorrelatedTerminalEvidence(t, reopened, crash.Journal, terminal)
		canonical := make([]*models.Artifact, 0, len(recoveredMembers)+1)
		canonical = append(canonical, recoveredShipment)
		for _, member := range memberPreimages {
			canonical = append(canonical, recoveredMembers[member.ID])
		}
		requireUR3PostSyncConvergence(t, reopened, canonical...)
	})
}

type ur10Subprocess struct {
	cmd    *exec.Cmd
	output *bytes.Buffer
}

func (child ur10Subprocess) wait() error {
	return child.cmd.Wait()
}

func (child ur10Subprocess) outputString() string {
	return child.output.String()
}

func newUR10Instruction(root, mode string) ur10SubprocessInstruction {
	prefix := filepath.Join(root, "r10-"+mode)
	return ur10SubprocessInstruction{
		Mode:          mode,
		Root:          root,
		ReadyPath:     prefix + "-ready",
		StartPath:     prefix + "-start",
		AttemptedPath: prefix + "-attempted",
		ContendedPath: prefix + "-contended",
		AcquiredPath:  prefix + "-acquired",
		ContinuePath:  prefix + "-continue",
		DonePath:      prefix + "-done",
	}
}

func startUR10Subprocess(t *testing.T, instruction ur10SubprocessInstruction) ur10Subprocess {
	t.Helper()

	instructionPath := filepath.Join(instruction.Root, "r10-"+instruction.Mode+"-instruction.json")
	data, err := json.Marshal(instruction)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(instructionPath, data, 0o644))

	cmd := exec.Command(os.Args[0], "-test.run=^TestShipmentBlockedRecoveryR10SubprocessHelper$", "-test.count=1")
	cmd.Env = append(os.Environ(), ur10SubprocessInstructionEnv+"="+instructionPath)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return ur10Subprocess{cmd: cmd, output: &output}
}

func runUR10RecoverySubprocess(t *testing.T, root string) {
	t.Helper()

	instruction := newUR10Instruction(root, "recovery")
	child := startUR10Subprocess(t, instruction)
	waitUR10Marker(t, instruction.AcquiredPath, "recovery global lock acquired")
	writeUR10Marker(t, instruction.ContinuePath)
	waitUR10Marker(t, instruction.DonePath, "recovery subprocess completed")
	require.NoError(t, child.wait(), child.outputString())
}

func waitUR10Marker(t *testing.T, path, description string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.FailNow(t, "subprocess synchronization marker not observed", "%s: %s", description, path)
}

func writeUR10Marker(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("ready"), 0o644))
}

func requireUR10MarkerAbsent(t *testing.T, path, message string) {
	t.Helper()
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), message)
}

func TestShipmentBlockedRecoveryR10SubprocessHelper(t *testing.T) {
	instructionPath := os.Getenv(ur10SubprocessInstructionEnv)
	if instructionPath == "" {
		t.Skip("subprocess-only R10 helper")
	}

	data, err := os.ReadFile(instructionPath)
	require.NoError(t, err)
	var instruction ur10SubprocessInstruction
	require.NoError(t, json.Unmarshal(data, &instruction))

	switch instruction.Mode {
	case "recovery":
		var acquiredOnce sync.Once
		hook := func(phase string) {
			if phase != "acquired" {
				return
			}
			acquiredOnce.Do(func() {
				writeUR10Marker(t, instruction.AcquiredPath)
				waitUR10Marker(t, instruction.ContinuePath, "parent released recovery barrier")
			})
		}
		ctx := context.WithValue(
			context.Background(),
			shipmentLifecycleGlobalLockHookContextKey{},
			hook,
		)
		ws, openErr := NewWorkspace(ctx, instruction.Root)
		require.NoError(t, openErr)
		require.NoError(t, ws.Close())
	case "competitor":
		ws, openErr := NewWorkspace(context.Background(), instruction.Root)
		require.NoError(t, openErr)
		defer func() { require.NoError(t, ws.Close()) }()
		writeUR10Marker(t, instruction.ReadyPath)
		waitUR10Marker(t, instruction.StartPath, "parent started competing governed operation")

		var attemptedOnce sync.Once
		var contendedOnce sync.Once
		var acquiredOnce sync.Once
		hook := func(phase string) {
			switch phase {
			case "attempt":
				attemptedOnce.Do(func() { writeUR10Marker(t, instruction.AttemptedPath) })
			case "contended":
				contendedOnce.Do(func() { writeUR10Marker(t, instruction.ContendedPath) })
			case "acquired":
				acquiredOnce.Do(func() { writeUR10Marker(t, instruction.AcquiredPath) })
			}
		}
		ctx := context.WithValue(
			context.Background(),
			shipmentLifecycleGlobalLockHookContextKey{},
			hook,
		)
		require.NoError(t, AddItemToShipment(ctx, ws, instruction.ShipmentID, instruction.ItemID))
	default:
		require.FailNow(t, "unsupported R10 subprocess mode", "%q", instruction.Mode)
	}
	writeUR10Marker(t, instruction.DonePath)
}
