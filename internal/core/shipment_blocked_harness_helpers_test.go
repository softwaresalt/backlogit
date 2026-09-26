package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

type urBlockedFixture struct {
	shipment *models.Artifact
	members  []*models.Artifact
}

func newURBlockedActiveFixture(t *testing.T, ws *Workspace) urBlockedFixture {
	t.Helper()

	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "UR blocked lifecycle feature", "feature")
	require.NoError(t, err)
	first, err := CreateArtifact(ctx, ws, "UR blocked lifecycle first member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	second, err := CreateArtifact(ctx, ws, "UR blocked lifecycle second member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "UR blocked lifecycle shipment", []string{first.ID, second.ID})
	require.NoError(t, err)
	claimed, err := ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Keep the pre-block member set deliberately heterogeneous. A homogeneous
	// all-active fixture can pass even when an implementation stores one status
	// for every member instead of a complete per-member preimage.
	second = cloneArtifact(loadURCanonicalArtifact(t, ws, second.ID))
	second.Status = models.StatusReview
	second.UpdatedAt = models.NowUTC()
	forceURArtifactFixture(t, ws, second)

	return urBlockedFixture{
		shipment: claimed,
		members:  []*models.Artifact{first, second},
	}
}

func forceURArtifactFixture(t *testing.T, ws *Workspace, artifact *models.Artifact) {
	t.Helper()

	ctx := context.Background()
	path, err := FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	content := models.SerializeFrontmatter(artifact.ToFrontmatterMap(), artifact.Description)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, artifact))
}

func loadURCanonicalArtifact(t *testing.T, ws *Workspace, id string) *models.Artifact {
	t.Helper()

	artifact, err := findArtifact(context.Background(), ws, id)
	require.NoError(t, err)
	return artifact
}

func statusSnapshotUR(value any) map[string]string {
	got := make(map[string]string)
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			id, _ := typed["id"].(string)
			if id == "" {
				id, _ = typed["item_id"].(string)
			}
			status, _ := typed["status"].(string)
			if id != "" && status != "" {
				got[id] = status
			}
			for key, nested := range typed {
				if nestedStatus, ok := nested.(string); ok && isURStatus(nestedStatus) {
					got[key] = nestedStatus
				}
				visit(nested)
			}
		case map[string]string:
			for key, nested := range typed {
				if isURStatus(nested) {
					got[key] = nested
				}
			}
		case map[string]models.ArtifactStatus:
			for key, nested := range typed {
				got[key] = string(nested)
			}
		case []any:
			for _, nested := range typed {
				visit(nested)
			}
		case []*models.Artifact:
			for _, artifact := range typed {
				if artifact != nil {
					got[artifact.ID] = string(artifact.Status)
				}
			}
		}
	}
	visit(value)
	return got
}

func isURStatus(value string) bool {
	switch models.ArtifactStatus(value) {
	case models.StatusQueued, models.StatusActive, models.StatusBlocked, models.StatusReview,
		models.StatusDone, models.StatusAccepted, models.StatusRejected, models.StatusArchived,
		models.StatusShipped, models.StatusAbandoned:
		return true
	default:
		return false
	}
}

func eventPhaseUR(event events.Event) string {
	if phase, ok := event.Delta["phase"].(string); ok {
		return strings.ToLower(phase)
	}
	eventType := strings.ToLower(event.EventType)
	switch {
	case strings.Contains(eventType, "intent"):
		return "intent"
	case strings.Contains(eventType, "commit"):
		return "committed"
	case strings.Contains(eventType, "compensat"), strings.Contains(eventType, "rollback"):
		return "compensated"
	default:
		return ""
	}
}

func eventCorrelationUR(event events.Event) string {
	for _, key := range []string{"correlation_id", "operation_id", "intent_id"} {
		if value, ok := event.Delta[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func exactEventCorrelationUR(event events.Event) string {
	correlationID, _ := event.Delta["correlation_id"].(string)
	return correlationID
}

func eventLifecycleOperationUR(event events.Event) string {
	for _, key := range []string{"operation", "lifecycle_operation", "action"} {
		value, _ := event.Delta[key].(string)
		switch strings.ToLower(value) {
		case "block", "block_shipment", "shipment_block":
			return "block"
		case "unblock", "unblock_shipment", "shipment_unblock":
			return "unblock"
		}
	}

	eventType := strings.ToLower(event.EventType)
	switch {
	case strings.Contains(eventType, "unblock"):
		return "unblock"
	case strings.Contains(eventType, "block"):
		return "block"
	}

	from, _ := event.Delta["from"].(string)
	to, _ := event.Delta["to"].(string)
	if to == string(ShipmentBlocked) {
		return "block"
	}
	if from == string(ShipmentBlocked) {
		return "unblock"
	}
	return ""
}

func eventMutationOperationUR(event events.Event) string {
	for _, key := range []string{"operation", "mutation", "action"} {
		value, _ := event.Delta[key].(string)
		switch strings.ToLower(value) {
		case "update", "update_artifact", "artifact_update":
			return "update"
		}
	}
	eventType := strings.ToLower(event.EventType)
	if strings.Contains(eventType, "update") {
		return "update"
	}
	return ""
}

func eventTargetUR(event events.Event) string {
	for _, key := range []string{"target", "to", "status"} {
		if value, ok := event.Delta[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func readUREvents(t *testing.T, ws *Workspace, itemID string) []events.Event {
	t.Helper()

	itemEvents, err := events.ReadAllEvents(context.Background(), WorkspaceLogsRoot(ws.RootPath), itemID)
	require.NoError(t, err)
	return itemEvents
}

func eventCarriesChangedFieldUR(event events.Event, field string, want any) bool {
	if changed, ok := event.Delta["changes"].(map[string]any); ok && assert.ObjectsAreEqual(want, changed[field]) {
		return true
	}
	if assert.ObjectsAreEqual(want, event.Delta[field]) {
		return true
	}
	gotField, _ := event.Delta["field"].(string)
	if gotField != field {
		return false
	}
	for _, key := range []string{"new_value", "to", "value"} {
		if assert.ObjectsAreEqual(want, event.Delta[key]) {
			return true
		}
	}
	return false
}

func requireNewCorrelatedMutationUR(
	t *testing.T,
	ws *Workspace,
	itemID string,
	baseline int,
	operation string,
	field string,
	want any,
) {
	t.Helper()

	allEvents := readUREvents(t, ws, itemID)
	require.GreaterOrEqual(t, len(allEvents), baseline)
	newEvents := allEvents[baseline:]
	for intentIndex, intent := range newEvents {
		if eventPhaseUR(intent) != "intent" || eventMutationOperationUR(intent) != operation {
			continue
		}
		correlationID := eventCorrelationUR(intent)
		require.NotEmpty(t, correlationID, "intent correlation id")
		for commitIndex := intentIndex + 1; commitIndex < len(newEvents); commitIndex++ {
			committed := newEvents[commitIndex]
			if eventPhaseUR(committed) != "committed" ||
				eventCorrelationUR(committed) != correlationID ||
				eventMutationOperationUR(committed) != operation {
				continue
			}
			for evidenceIndex := intentIndex; evidenceIndex <= commitIndex; evidenceIndex++ {
				evidence := newEvents[evidenceIndex]
				if eventCorrelationUR(evidence) == correlationID &&
					eventMutationOperationUR(evidence) == operation &&
					eventCarriesChangedFieldUR(evidence, field, want) {
					return
				}
			}
		}
	}
	require.FailNow(t, "new correlated mutation evidence missing",
		"item %s must append an exact %s/%s=%v intent and ordered commit under one non-empty correlation id, with field evidence on either endpoint or a correlated intermediate event",
		itemID, operation, field, want)
}

func requireCorrelatedLifecycleIntentCommitUR(
	t *testing.T,
	ws *Workspace,
	itemID string,
	operation string,
	target ShipmentStatus,
	journalPath string,
) []events.Event {
	t.Helper()

	journalCorrelationID := readURLifecycleJournalCorrelation(t, journalPath)
	itemEvents := readUREvents(t, ws, itemID)
	intentIndex, intent, commitIndex, committed, found := findCorrelatedLifecyclePairUR(
		itemEvents,
		operation,
		target,
		journalCorrelationID,
	)
	require.True(t, found, "exact lifecycle intent/commit pair missing: %s to %s on %s", operation, target, itemID)
	correlationID := exactEventCorrelationUR(intent)
	require.NotEmpty(t, correlationID, "%s intent correlation id", operation)
	require.Equal(t, journalCorrelationID, correlationID,
		"%s event intent must use the durable operation journal correlation", operation)
	require.Equal(t, correlationID, exactEventCorrelationUR(committed), "%s commit correlation id", operation)
	require.Equal(t, operation, eventLifecycleOperationUR(intent), "%s intent operation", operation)
	require.Equal(t, operation, eventLifecycleOperationUR(committed), "%s commit operation", operation)
	require.Equal(t, string(target), eventTargetUR(intent), "%s intent target", operation)
	require.Equal(t, string(target), eventTargetUR(committed), "%s commit target", operation)
	require.Less(t, intentIndex, commitIndex, "%s intent must precede its exact commit", operation)

	statusIndex := -1
	for index := intentIndex + 1; index < commitIndex; index++ {
		evidence := itemEvents[index]
		if evidence.EventType != "shipment_status_changed" ||
			exactEventCorrelationUR(evidence) != correlationID ||
			eventLifecycleOperationUR(evidence) != operation ||
			eventTargetUR(evidence) != string(target) {
			continue
		}
		statusIndex = index
		break
	}
	require.Greater(t, statusIndex, intentIndex,
		"exact status evidence must follow the %s intent under correlation %s", operation, correlationID)
	require.Less(t, statusIndex, commitIndex,
		"exact status evidence must precede the %s commit under correlation %s", operation, correlationID)
	return itemEvents
}

func findCorrelatedLifecyclePairUR(
	itemEvents []events.Event,
	operation string,
	target ShipmentStatus,
	correlationID string,
) (int, events.Event, int, events.Event, bool) {
	for intentIndex, intent := range itemEvents {
		if eventPhaseUR(intent) != "intent" ||
			eventLifecycleOperationUR(intent) != operation ||
			eventTargetUR(intent) != string(target) ||
			exactEventCorrelationUR(intent) != correlationID {
			continue
		}
		for commitIndex := intentIndex + 1; commitIndex < len(itemEvents); commitIndex++ {
			committed := itemEvents[commitIndex]
			if eventPhaseUR(committed) != "committed" ||
				exactEventCorrelationUR(committed) != correlationID ||
				eventLifecycleOperationUR(committed) != operation ||
				eventTargetUR(committed) != string(target) {
				continue
			}
			return intentIndex, intent, commitIndex, committed, true
		}
	}
	return -1, events.Event{}, -1, events.Event{}, false
}

func readURLifecycleJournalCorrelation(t *testing.T, journalPath string) string {
	t.Helper()

	require.NotEmpty(t, journalPath, "durable operation journal path")
	data, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	var state struct {
		CorrelationID string `json:"correlation_id"`
	}
	require.NoError(t, json.Unmarshal(data, &state))
	require.NotEmpty(t, state.CorrelationID, "durable operation journal correlation id")
	return state.CorrelationID
}

func artifactCodecViewUR(t *testing.T, artifact *models.Artifact) map[string]any {
	t.Helper()

	require.NotNil(t, artifact)
	data, err := json.Marshal(artifact)
	require.NoError(t, err)
	var normalized map[string]any
	require.NoError(t, json.Unmarshal(data, &normalized))
	return normalized
}

func assertURArtifactEqual(t *testing.T, want, got *models.Artifact) {
	t.Helper()

	assert.Equal(t, artifactCodecViewUR(t, want), artifactCodecViewUR(t, got),
		"complete normalized artifact must match exactly")
}

type urAggregateSnapshot struct {
	ArtifactIDs        []string
	Artifacts          map[string]*models.Artifact
	CanonicalFiles     map[string]string
	ArtifactLocations  map[string][]string
	DatabaseItems      map[string]map[string]any
	DatabaseProjection map[string][]string
	EventLogs          map[string]string
	OperationJournals  map[string]string
}

func snapshotURAggregate(t *testing.T, ws *Workspace, shipmentID string) urAggregateSnapshot {
	t.Helper()

	shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
	ids := append([]string{shipmentID}, NormalizeShipmentItems(shipment)...)
	return snapshotURGovernedState(t, ws, ids)
}

func snapshotURWorkspace(t *testing.T, ws *Workspace) urAggregateSnapshot {
	t.Helper()

	return snapshotURGovernedState(t, ws, nil)
}

func snapshotURGovernedState(t *testing.T, ws *Workspace, ids []string) urAggregateSnapshot {
	t.Helper()

	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	artifacts := make(map[string]*models.Artifact, len(ids))
	for _, id := range ids {
		artifacts[id] = cloneArtifact(loadURCanonicalArtifact(t, ws, id))
	}

	canonicalFiles, locations := snapshotURCanonicalFiles(t, ws)
	databaseItems := snapshotURDatabaseItems(t, ws)
	return urAggregateSnapshot{
		ArtifactIDs:        ids,
		Artifacts:          artifacts,
		CanonicalFiles:     canonicalFiles,
		ArtifactLocations:  locations,
		DatabaseItems:      databaseItems,
		DatabaseProjection: snapshotURDatabaseProjection(t, ws),
		EventLogs:          snapshotURFileTree(t, workspaceStorageRoot(ws), WorkspaceLogsRoot(ws.RootPath), nil),
		OperationJournals:  snapshotURFileTree(t, workspaceStorageRoot(ws), shipmentOpsRoot(ws.RootPath), nil),
	}
}

func requireURAggregateUnchanged(t *testing.T, ws *Workspace, before urAggregateSnapshot) {
	t.Helper()

	after := snapshotURGovernedState(t, ws, before.ArtifactIDs)
	for id, want := range before.Artifacts {
		got := after.Artifacts[id]
		require.Equal(t, artifactCodecViewUR(t, want), artifactCodecViewUR(t, got),
			"refusal must preserve the complete normalized artifact %s", id)
	}
	require.Equal(t, before.CanonicalFiles, after.CanonicalFiles,
		"refusal must preserve canonical artifact inventory and raw content")
	require.Equal(t, before.ArtifactLocations, after.ArtifactLocations,
		"refusal must preserve every canonical artifact location")
	require.Equal(t, before.DatabaseItems, after.DatabaseItems,
		"refusal must preserve the complete SQLite item projection")
	require.Equal(t, before.DatabaseProjection, after.DatabaseProjection,
		"refusal must preserve governed SQLite relationship and event projections")
	require.Equal(t, before.EventLogs, after.EventLogs,
		"refusal must not append, remove, or rewrite event logs")
	require.Equal(t, before.OperationJournals, after.OperationJournals,
		"refusal must not create, remove, or rewrite operation journals")
}

func snapshotURCanonicalFiles(t *testing.T, ws *Workspace) (map[string]string, map[string][]string) {
	t.Helper()

	storageRoot := workspaceStorageRoot(ws)
	files := snapshotURFileTree(t, storageRoot, storageRoot, func(path string) bool {
		return filepath.Ext(path) == ".md"
	})
	locations := make(map[string][]string)
	for path := range files {
		artifact, _, parseErr := parseFile(filepath.Join(storageRoot, filepath.FromSlash(path)))
		if parseErr == nil && artifact.ID != "" {
			locations[artifact.ID] = append(locations[artifact.ID], path)
		}
	}
	for id := range locations {
		sort.Strings(locations[id])
	}
	return files, locations
}

func snapshotURDatabaseItems(t *testing.T, ws *Workspace) map[string]map[string]any {
	t.Helper()

	items, err := bldb.QueryItems(context.Background(), ws.DB, bldb.QueryFilters{IncludeArchived: true})
	require.NoError(t, err)
	projection := make(map[string]map[string]any, len(items))
	for _, item := range items {
		projection[item.ID] = artifactCodecViewUR(t, item)
	}
	return projection
}

func snapshotURDatabaseProjection(t *testing.T, ws *Workspace) map[string][]string {
	t.Helper()

	tables := []struct {
		name  string
		query string
	}{
		{name: "items", query: `SELECT * FROM items`},
		{name: "item_deps", query: `SELECT * FROM item_deps`},
		{name: "item_links", query: `SELECT * FROM item_links`},
		{name: "commit_links", query: `SELECT * FROM commit_links`},
		{name: "stash_links", query: `SELECT * FROM stash_links`},
		{name: "item_logs", query: `SELECT * FROM item_logs`},
		{name: "item_log_entries", query: `SELECT * FROM item_log_entries`},
	}
	projection := make(map[string][]string, len(tables))
	for _, table := range tables {
		projection[table.name] = snapshotURDatabaseTable(t, ws, table.query)
	}
	return projection
}

func snapshotURDatabaseTable(t *testing.T, ws *Workspace, query string) []string {
	t.Helper()

	rows, err := ws.DB.QueryContext(context.Background(), query)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	columns, err := rows.Columns()
	require.NoError(t, err)
	records := make([]string, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		require.NoError(t, rows.Scan(destinations...))
		for index, value := range values {
			if bytes, ok := value.([]byte); ok {
				values[index] = string(bytes)
			}
		}
		record, err := json.Marshal(values)
		require.NoError(t, err)
		records = append(records, string(record))
	}
	require.NoError(t, rows.Err())
	sort.Strings(records)
	return records
}

func snapshotURFileTree(
	t *testing.T,
	relativeRoot string,
	treeRoot string,
	include func(string) bool,
) map[string]string {
	t.Helper()

	files := make(map[string]string)
	if _, err := os.Stat(treeRoot); os.IsNotExist(err) {
		return files
	} else {
		require.NoError(t, err)
	}
	require.NoError(t, filepath.WalkDir(treeRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || include != nil && !include(path) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(relativeRoot, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relativePath)] = string(content)
		return nil
	}))
	return files
}

type urLifecyclePreimage struct {
	Shipment *models.Artifact   `json:"shipment"`
	Members  []*models.Artifact `json:"members"`
}

func requireDurableLifecycleIntentPreimageUR(
	t *testing.T,
	ws *Workspace,
	operation string,
	shipment *models.Artifact,
	members []*models.Artifact,
) string {
	t.Helper()

	entries, err := os.ReadDir(shipmentOpsRoot(ws.RootPath))
	require.NoError(t, err, "durable operation directory must exist before the first artifact mutation")
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(shipmentOpsRoot(ws.RootPath), entry.Name())
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		var state struct {
			CorrelationID string              `json:"correlation_id"`
			Phase         string              `json:"phase"`
			Operation     string              `json:"operation"`
			ShipmentID    string              `json:"shipment_id"`
			Preimage      urLifecyclePreimage `json:"preimage"`
		}
		if json.Unmarshal(data, &state) != nil ||
			state.Phase != "intent" ||
			state.Operation != operation ||
			state.ShipmentID != shipment.ID {
			continue
		}
		require.NotEmpty(t, state.CorrelationID, "durable lifecycle intent correlation id")
		assertURArtifactEqual(t, shipment, state.Preimage.Shipment)
		require.Len(t, state.Preimage.Members, len(members), "intent must contain every member preimage")
		gotByID := make(map[string]*models.Artifact, len(state.Preimage.Members))
		for _, member := range state.Preimage.Members {
			require.NotNil(t, member)
			gotByID[member.ID] = member
		}
		for _, member := range members {
			got, found := gotByID[member.ID]
			require.True(t, found, "intent preimage missing member %s", member.ID)
			assertURArtifactEqual(t, member, got)
		}
		return path
	}
	require.FailNow(t, "durable lifecycle intent missing before first mutation",
		"no %s intent with a complete preimage was found for %s", operation, shipment.ID)
	return ""
}
