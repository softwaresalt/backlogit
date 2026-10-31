package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// reconstituteJournalState is the durable preimage a reconstitute journal
// carries so rollback can restore the workspace after a failure or crash.
type reconstituteJournalState struct {
	Images        []reconstituteFileImage  `json:"images"`
	KnownMarkdown []string                 `json:"known_markdown"`
	KnownLogs     []string                 `json:"known_logs"`
	CommitLinks   []reconstituteCommitLink `json:"commit_links"`
	CommitItemIDs []string                 `json:"commit_item_ids"`
	EventIDs      []string                 `json:"event_ids"`
	LockIDs       []string                 `json:"lock_ids"`
}

// reconstituteFileImage is a byte-exact preimage of a file; Path is relative
// to the storage root and slash-separated.
type reconstituteFileImage struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content []byte `json:"content,omitempty"`
}

type reconstituteCommitLink struct {
	ItemID    string `json:"item_id"`
	CommitSHA string `json:"commit_sha"`
	Message   string `json:"message"`
	Author    string `json:"author"`
}

func (st *reconstitutePlanState) affectedIDs() []string {
	ids := map[string]struct{}{}
	add := func(id string) {
		if id != "" && !strings.Contains(id, ":") {
			ids[id] = struct{}{}
		}
	}
	for id := range st.sources {
		add(id)
	}
	for _, target := range st.targets {
		add(target.key)
		if target.existingFeature != nil {
			add(target.featureKey)
		}
		for _, item := range target.items {
			add(item)
		}
		if create := target.spec.Feature.Create; create != nil {
			add(strings.TrimSpace(create.RelatedTo))
		}
	}
	for key := range st.edgeAdds {
		add(key)
	}
	for key := range st.edgeRemoves {
		add(key)
	}
	for _, rewire := range st.rewireItems {
		add(rewire.item)
		for _, dep := range rewire.blocksOn {
			add(dep)
		}
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func executeReconstitution(ctx context.Context, ws *Workspace, opts ReconstituteOptions) (_ *ReconstituteShipmentResult, err error) {
	// Fail fast on an invalid plan before taking any lock; the plan is
	// re-validated under the locks below.
	peek, err := planReconstitution(ctx, ws, opts)
	if err != nil {
		return nil, err
	}
	unlockStash, err := lockActiveStash(ws)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: acquire stash lock: %w", err)
	}
	defer func() {
		if unlockErr := unlockStash(); unlockErr != nil {
			slog.WarnContext(ctx, "release stash lock after reconstitute", "error", unlockErr)
		}
	}()
	ctx = withStashLockHeld(ctx)

	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: lock shipment lifecycle: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after reconstitute", "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	sourceIDs := make([]string, 0, len(peek.sources))
	for id := range peek.sources {
		sourceIDs = append(sourceIDs, id)
	}
	sort.Strings(sourceIDs)
	for _, id := range sourceIDs {
		unlock, lockErr := lockShipmentMembership(ctx, ws, id)
		if lockErr != nil {
			return nil, fmt.Errorf("reconstitute: lock shipment %s membership: %w", id, lockErr)
		}
		defer func(id string) {
			if unlockErr := unlock(); unlockErr != nil {
				slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", id, "error", unlockErr)
			}
		}(id)
	}
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, peek.affectedIDs())
	if err != nil {
		return nil, fmt.Errorf("reconstitute: lock affected artifacts: %w", err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release reconstitute artifact locks", "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	st, err := planReconstitution(ctx, ws, opts)
	if err != nil {
		return nil, err
	}
	baseline, err := reconstituteErrorFindings(ws)
	if err != nil {
		return nil, err
	}
	correlationID, err := newShipmentManifestCorrelationID()
	if err != nil {
		return nil, err
	}
	state, err := captureReconstituteState(ctx, ws, st)
	if err != nil {
		return nil, err
	}
	firstSource := sourceIDs[0]
	journal := shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  correlationID,
		Phase:          shipmentOperationPhaseIntent,
		Operation:      shipmentOperationReconstitute,
		RecoveryPolicy: shipmentOperationRecoveryPolicyRollback,
		ShipmentID:     firstSource,
		Target:         shipmentOperationReconstitute,
		Reason:         strings.TrimSpace(opts.Reason),
		Preimage: shipmentLifecyclePreimage{
			Shipment: cloneArtifact(st.sources[firstSource]),
			Members:  []*models.Artifact{},
		},
		Reconstitute: state,
	}
	journalName := shipmentLifecycleJournalName(correlationID)
	journalPath, err := writeShipmentOperationJournal(ws, journalName, journal)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: persist intent: %w", err)
	}

	opCtx := hooks.WithBufferedPostHooks(withShipmentOperation(ctx, correlationID))
	result, applyErr := applyReconstitution(opCtx, ws, st, correlationID, baseline)
	if applyErr != nil {
		hooks.DiscardBufferedPostHooks(opCtx)
		if blerrors.IsWriteIndeterminate(applyErr) {
			// Outcome unknown: keep the intent journal so the lifecycle
			// barrier decides between rollback and finalize.
			return nil, fmt.Errorf("reconstitute: %w", applyErr)
		}
		if rollbackErr := rollbackReconstitute(opCtx, ws, journal); rollbackErr != nil {
			return nil, fmt.Errorf("reconstitute rolled back with errors (cause: %v): %w", applyErr,
				errors.Join(rollbackErr, blerrors.ErrWriteIndeterminate))
		}
		cleanupShipmentOperationJournal(ctx, ws, journalPath)
		return nil, fmt.Errorf("reconstitute: %w", applyErr)
	}

	committed := journal
	committed.Phase = shipmentOperationPhaseCommitted
	if _, writeErr := writeShipmentOperationJournal(ws, journalName, committed); writeErr != nil {
		hooks.DiscardBufferedPostHooks(opCtx)
		return nil, fmt.Errorf("reconstitute: persist committed journal: %w",
			errors.Join(writeErr, blerrors.ErrWriteIndeterminate))
	}
	cleanupShipmentOperationJournal(ctx, ws, journalPath)
	hooks.FlushBufferedPostHooks(opCtx, ws.HookRunner)
	return result, nil
}

func reconstituteErrorFindings(ws *Workspace) (map[string]struct{}, error) {
	refs, err := scanCanonicalArtifacts(ws)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: scan workspace: %w", err)
	}
	ids := make([]string, 0, len(refs))
	for id := range refs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	findings, err := detectShipmentManifestFindings(workspaceStorageRoot(ws), refs, ids)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: inspect manifests: %w", err)
	}
	out := map[string]struct{}{}
	for _, finding := range findings {
		if finding.Severity == DoctorSeverityError {
			out[string(finding.Type)+"|"+finding.ArtifactID+"|"+finding.Description] = struct{}{}
		}
	}
	return out, nil
}

// captureReconstituteState records the preimage needed to undo the operation.
func captureReconstituteState(ctx context.Context, ws *Workspace, st *reconstitutePlanState) (*reconstituteJournalState, error) {
	storageRoot := workspaceStorageRoot(ws)
	affected := st.affectedIDs()
	state := &reconstituteJournalState{
		Images: []reconstituteFileImage{}, KnownMarkdown: []string{}, KnownLogs: []string{},
		CommitLinks: []reconstituteCommitLink{}, CommitItemIDs: []string{},
		EventIDs: affected, LockIDs: affected,
	}
	rel := func(path string) string {
		relative, err := filepath.Rel(storageRoot, path)
		if err != nil {
			return filepath.ToSlash(path)
		}
		return filepath.ToSlash(relative)
	}

	dirs, err := artifactSearchDirs(ws)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: resolve artifact directories: %w", err)
	}
	for _, dir := range dirs {
		if _, statErr := os.Lstat(dir); os.IsNotExist(statErr) {
			continue
		}
		walkErr := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".md" || strings.EqualFold(filepath.Base(path), ".stash.md") {
				return nil
			}
			state.KnownMarkdown = append(state.KnownMarkdown, rel(path))
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			content := string(data)
			for _, id := range affected {
				if strings.Contains(content, id) {
					state.Images = append(state.Images, reconstituteFileImage{Path: rel(path), Exists: true, Content: data})
					break
				}
			}
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("reconstitute: snapshot %s: %w", dir, walkErr)
		}
	}
	for _, name := range []string{stashArchiveFileName, "provenance_corrections.jsonl"} {
		path := filepath.Join(storageRoot, "archive", name)
		data, readErr := os.ReadFile(path)
		switch {
		case readErr == nil:
			state.Images = append(state.Images, reconstituteFileImage{Path: rel(path), Exists: true, Content: data})
		case os.IsNotExist(readErr):
			state.Images = append(state.Images, reconstituteFileImage{Path: rel(path)})
		default:
			return nil, fmt.Errorf("reconstitute: snapshot %s: %w", path, readErr)
		}
	}
	logEntries, err := os.ReadDir(WorkspaceLogsRoot(ws.RootPath))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reconstitute: list logs: %w", err)
	}
	for _, entry := range logEntries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
			state.KnownLogs = append(state.KnownLogs, "logs/"+entry.Name())
		}
	}

	for _, target := range st.targets {
		state.CommitItemIDs = append(state.CommitItemIDs, target.items...)
	}
	for _, item := range state.CommitItemIDs {
		rows, queryErr := ws.DB.QueryContext(ctx,
			`SELECT item_id, commit_sha, message, author FROM commit_links WHERE item_id = ?`, item)
		if queryErr != nil {
			return nil, fmt.Errorf("reconstitute: snapshot commit links for %s: %w", item, queryErr)
		}
		for rows.Next() {
			var link reconstituteCommitLink
			if scanErr := rows.Scan(&link.ItemID, &link.CommitSHA, &link.Message, &link.Author); scanErr != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("reconstitute: scan commit link: %w", scanErr)
			}
			state.CommitLinks = append(state.CommitLinks, link)
		}
		if closeErr := errors.Join(rows.Err(), rows.Close()); closeErr != nil {
			return nil, fmt.Errorf("reconstitute: snapshot commit links for %s: %w", item, closeErr)
		}
	}
	return state, nil
}

func applyReconstitution(
	ctx context.Context,
	ws *Workspace,
	st *reconstitutePlanState,
	correlationID string,
	baseline map[string]struct{},
) (*ReconstituteShipmentResult, error) {
	result := newReconstituteResult(st)
	result.Status, result.CorrelationID = "reconstituted", correlationID
	for id, manifest := range st.manifests {
		result.BeforeManifests[id] = append([]string{}, manifest...)
	}
	shipmentIDs := map[string]string{}
	featureIDs := map[string]string{}
	for _, target := range st.targets {
		if target.existing != nil {
			shipmentIDs[target.key] = target.key
		}
		if target.existingFeature != nil {
			featureIDs[target.featureKey] = target.featureKey
		}
	}

	for _, target := range st.targets {
		create := target.spec.Feature.Create
		if create == nil {
			continue
		}
		var opts []Option
		if create.Priority != "" {
			opts = append(opts, WithPriority(create.Priority))
		}
		if create.Description != "" {
			opts = append(opts, WithDescription(create.Description))
		}
		feature, err := CreateArtifact(ctx, ws, create.Title, "feature", opts...)
		if err != nil {
			return nil, fmt.Errorf("create covering feature %q: %w", create.Title, err)
		}
		featureIDs[target.featureKey] = feature.ID
		result.CreatedFeatures = append(result.CreatedFeatures, ReconstituteCreatedArtifact{
			ID: feature.ID, Label: target.spec.Label, Title: create.Title,
		})
		if related := strings.TrimSpace(create.RelatedTo); related != "" {
			if err := AddArtifactLink(ctx, ws, feature.ID, related, "related_to"); err != nil {
				return nil, fmt.Errorf("link feature %s to %s: %w", feature.ID, related, err)
			}
		}
	}

	finalID := func(id string) string {
		if newID, ok := result.AdoptedIDMap[id]; ok {
			return newID
		}
		return id
	}
	destination := map[string]*reconstituteTarget{}
	for _, target := range st.targets {
		for _, item := range target.items {
			destination[item] = target
		}
		for _, item := range target.adopt {
			adopted, err := AdoptItemWithOptions(ctx, ws, item, featureIDs[target.featureKey],
				AdoptItemOptions{KeepID: target.spec.KeepID})
			if err != nil {
				return nil, fmt.Errorf("adopt item %s under %s: %w", item, featureIDs[target.featureKey], err)
			}
			if adopted.NewID != "" && adopted.NewID != item {
				result.AdoptedIDMap[item] = adopted.NewID
			}
		}
	}

	moved := make([]string, 0, len(st.moves))
	for item := range st.moves {
		moved = append(moved, item)
	}
	sort.Strings(moved)
	for _, item := range moved {
		move := st.moves[item]
		member, err := findArtifact(ctx, ws, finalID(item))
		if err != nil {
			return nil, fmt.Errorf("load moved item %s: %w", finalID(item), err)
		}
		var dest *models.Artifact
		if target := destination[item]; target != nil {
			dest = target.existing
			if dest == nil {
				dest = &models.Artifact{ID: target.key, ArtifactType: "shipment", Status: models.StatusQueued}
			}
		}
		updated, changed, _ := planMemberDetach(member, move.sourceID, dest)
		if !changed {
			continue
		}
		previous := member.Status
		if err := persistArtifact(ctx, ws, updated, shouldRelocateOnStatusChange(previous, updated.Status)); err != nil {
			return nil, fmt.Errorf("persist moved item %s: %w", updated.ID, err)
		}
		if previous != updated.Status {
			if err := ws.appendShipmentEvent(ctx, updated.ID, "status_changed", map[string]any{
				"from": string(previous), "to": string(updated.Status),
				"reason": "shipment reconstitution", "correlation_id": correlationID, "operation": shipmentOperationReconstitute,
			}); err != nil {
				return nil, fmt.Errorf("append item %s status event: %w", updated.ID, err)
			}
		}
	}

	for _, target := range st.targets {
		if target.existing != nil {
			continue
		}
		shipment, err := CreateShipment(ctx, ws, target.title, []string{})
		if err != nil {
			return nil, fmt.Errorf("create shipment %q: %w", target.title, err)
		}
		shipmentIDs[target.key] = shipment.ID
		result.CreatedShipments = append(result.CreatedShipments, ReconstituteCreatedArtifact{
			ID: shipment.ID, Label: target.spec.Label, Title: target.title,
		})
	}

	for _, target := range st.targets {
		id := shipmentIDs[target.key]
		featureID := featureIDs[target.featureKey]
		arts := map[string]*models.Artifact{}
		feature, err := findArtifact(ctx, ws, featureID)
		if err != nil {
			return nil, fmt.Errorf("load covering feature %s: %w", featureID, err)
		}
		arts[featureID] = feature
		itemIDs := make([]string, 0, len(target.items))
		for _, item := range target.items {
			art, err := findArtifact(ctx, ws, finalID(item))
			if err != nil {
				return nil, fmt.Errorf("load item %s: %w", finalID(item), err)
			}
			arts[art.ID] = art
			itemIDs = append(itemIDs, art.ID)
		}
		order, err := orderReconstituteManifest(featureID, itemIDs, arts, target.autoOrder())
		if err != nil {
			return nil, err
		}
		current, err := findArtifact(ctx, ws, id)
		if err != nil {
			return nil, fmt.Errorf("load shipment %s: %w", id, err)
		}
		updated := cloneArtifact(current)
		normalizeShipmentArtifact(updated)
		updated.CustomFields["items"] = order
		if containsString(st.dagRoots, target.key) && !containsString(updated.Labels, reconstituteDagRootLabel) {
			updated.Labels = append(updated.Labels, reconstituteDagRootLabel)
		}
		updated.UpdatedAt = models.NowUTC()
		hookCtx := hooks.HookContext{
			ItemID: id, ArtifactType: "shipment", Actor: "backlogit", Workspace: ws.RootPath,
			NewValues: map[string]any{"items": order},
		}
		if ws.HookRunner != nil {
			if err := ws.HookRunner.FirePre(ctx, hooks.HookUpdateArtifact, hookCtx); err != nil {
				return nil, fmt.Errorf("pre-update hook: %w", err)
			}
		}
		if err := persistArtifact(ctx, ws, updated, false); err != nil {
			return nil, fmt.Errorf("persist shipment %s manifest: %w", id, err)
		}
		if ws.HookRunner != nil {
			ws.HookRunner.FirePost(ctx, hooks.HookUpdateArtifact, hookCtx)
		}
		result.TargetShipments = append(result.TargetShipments, id)
		result.AfterManifests[id] = order
		if target.spec.Label != "" {
			result.LabelMap[target.spec.Label] = id
		}
	}

	idOf := func(key string) string {
		if id, ok := shipmentIDs[key]; ok {
			return id
		}
		return key
	}
	if err := applyReconstituteEdges(ctx, ws, st, idOf, finalID); err != nil {
		return nil, err
	}
	result.DependencyChanges = st.dependencyChanges(idOf, result.AdoptedIDMap)

	eventShipments := append([]string{}, result.SourceShipments...)
	for _, id := range result.TargetShipments {
		eventShipments = appendUniqueString(eventShipments, id)
	}
	sort.Strings(eventShipments)
	for _, id := range eventShipments {
		delta := map[string]any{
			"shipment_id":        id,
			"correlation_id":     correlationID,
			"operation":          shipmentOperationReconstitute,
			"phase":              shipmentOperationPhaseCommitted,
			"plan_digest":        st.digest,
			"reason":             result.Reason,
			"force":              st.opts.Force,
			"source_shipments":   result.SourceShipments,
			"target_shipments":   result.TargetShipments,
			"created_features":   result.CreatedFeatures,
			"created_shipments":  result.CreatedShipments,
			"adopted_id_map":     result.AdoptedIDMap,
			"dependency_changes": result.DependencyChanges,
		}
		if err := ws.appendShipmentEvent(ctx, id, EventShipmentReconstituted, delta); err != nil {
			return nil, fmt.Errorf("append %s event for shipment %s: %w", EventShipmentReconstituted, id, err)
		}
	}

	after, err := reconstituteErrorFindings(ws)
	if err != nil {
		return nil, err
	}
	for finding := range after {
		if _, preexisting := baseline[finding]; !preexisting {
			return nil, reconstituteValidationError("post-apply manifest verification failed: %s", finding)
		}
	}
	return result, nil
}

func applyReconstituteEdges(
	ctx context.Context,
	ws *Workspace,
	st *reconstitutePlanState,
	idOf func(string) string,
	finalID func(string) string,
) error {
	keys := map[string]struct{}{}
	for key := range st.edgeAdds {
		keys[key] = struct{}{}
	}
	for key := range st.edgeRemoves {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	for _, key := range ordered {
		for _, remove := range st.edgeRemoves[key] {
			if err := RemoveDependency(ctx, ws, idOf(key), idOf(remove)); err != nil {
				return fmt.Errorf("remove dependency %s->%s: %w", idOf(key), idOf(remove), err)
			}
		}
	}
	for _, key := range ordered {
		for _, add := range st.edgeAdds[key] {
			if err := AddDependency(ctx, ws, idOf(key), idOf(add), "blocks"); err != nil {
				return fmt.Errorf("add dependency %s->%s: %w", idOf(key), idOf(add), err)
			}
		}
	}
	for _, rewire := range st.rewireItems {
		subject := finalID(rewire.item)
		current, err := findArtifact(ctx, ws, subject)
		if err != nil {
			return fmt.Errorf("load rewire item %s: %w", subject, err)
		}
		desired := reconstituteMapIDs(rewire.blocksOn, map[string]string{})
		for index, dep := range desired {
			desired[index] = finalID(dep)
		}
		for _, dep := range current.Dependencies {
			if isBlocksDependency(dep) && !containsString(desired, dep.ID) {
				if err := RemoveDependency(ctx, ws, subject, dep.ID); err != nil {
					return fmt.Errorf("remove dependency %s->%s: %w", subject, dep.ID, err)
				}
			}
		}
		for _, dep := range desired {
			if err := AddDependency(ctx, ws, subject, dep, "blocks"); err != nil {
				return fmt.Errorf("add dependency %s->%s: %w", subject, dep, err)
			}
		}
	}
	return nil
}

func reconcileReconstituteIntent(
	ctx context.Context,
	ws *Workspace,
	journalPath string,
	journal shipmentLifecycleJournal,
) (*models.Artifact, error) {
	if err := validateShipmentLifecycleJournalRecord(journal, journal.CorrelationID); err != nil {
		return nil, fmt.Errorf("shipment lifecycle journal %s is incomplete: %w", journalPath, err)
	}
	if journal.Phase != shipmentOperationPhaseCommitted {
		if err := rollbackReconstitute(withShipmentOperation(ctx, journal.CorrelationID), ws, journal); err != nil {
			return nil, fmt.Errorf("roll back reconstitute journal %s: %w", filepath.Base(journalPath), err)
		}
		journal.Phase = "compensated"
	}
	if _, err := writeShipmentOperationJournal(ws, filepath.Base(journalPath), journal); err != nil {
		return nil, fmt.Errorf("persist shipment %s reconstitute recovery journal: %w", journal.ShipmentID, err)
	}
	cleanupShipmentOperationJournal(ctx, ws, journalPath)
	current, err := findArtifact(ctx, ws, journal.ShipmentID)
	if err != nil {
		return nil, fmt.Errorf("load shipment %s recovered state: %w", journal.ShipmentID, err)
	}
	return current, nil
}

// rollbackReconstitute restores the preimage recorded in journal. It is
// idempotent and shared by in-process failure handling and crash recovery.
func rollbackReconstitute(ctx context.Context, ws *Workspace, journal shipmentLifecycleJournal) error {
	ctx = withShipmentOperation(context.WithoutCancel(ctx), journal.CorrelationID)
	state := journal.Reconstitute
	if state == nil {
		return rollbackReconstitutePreimage(ctx, ws, journal)
	}
	if !stashLockHeld(ctx) {
		unlock, err := tryLockActiveStash(ws)
		if err != nil {
			return fmt.Errorf("lock active stash for rollback: %w", err)
		}
		defer func() { _ = unlock() }()
	}

	var errs []error
	storageRoot := workspaceStorageRoot(ws)
	logsRoot := WorkspaceLogsRoot(ws.RootPath)
	abs := func(rel string) (string, error) {
		path := filepath.Join(storageRoot, filepath.FromSlash(rel))
		if !pathContained(storageRoot, path) {
			return "", fmt.Errorf("journal path %q escapes the storage root: %w", rel, blerrors.ErrValidation)
		}
		return path, nil
	}

	knownLogs := map[string]struct{}{}
	for _, rel := range state.KnownLogs {
		knownLogs[rel] = struct{}{}
	}
	pairs, unknownLogs, err := discoverReconstituteAdoptions(ctx, ws, journal.CorrelationID, logsRoot, knownLogs)
	if err != nil {
		errs = append(errs, err)
	}
	oldIDs := make([]string, 0, len(pairs))
	for oldID := range pairs {
		oldIDs = append(oldIDs, oldID)
	}
	sort.Strings(oldIDs)
	for _, oldID := range oldIDs {
		if err := restoreReconstitutedItemLogs(ctx, ws, logsRoot, oldID, pairs[oldID]); err != nil {
			errs = append(errs, err)
		}
	}
	if err := removeShipmentOperationEvents(ctx, ws, state.EventIDs, journal.CorrelationID); err != nil {
		errs = append(errs, fmt.Errorf("scrub reconstitute events: %w", err))
	}
	for _, rel := range unknownLogs {
		path, err := abs(rel)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove created log %s: %w", rel, err))
		}
	}

	known := map[string]struct{}{}
	for _, rel := range state.KnownMarkdown {
		known[rel] = struct{}{}
	}
	if dirs, dirErr := artifactSearchDirs(ws); dirErr != nil {
		errs = append(errs, dirErr)
	} else {
		for _, dir := range dirs {
			if _, statErr := os.Lstat(dir); os.IsNotExist(statErr) {
				continue
			}
			walkErr := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() || filepath.Ext(path) != ".md" || strings.EqualFold(filepath.Base(path), ".stash.md") {
					return nil
				}
				rel, relErr := filepath.Rel(storageRoot, path)
				if relErr != nil {
					return relErr
				}
				if _, ok := known[filepath.ToSlash(rel)]; !ok {
					if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
						return fmt.Errorf("remove created artifact %s: %w", rel, rmErr)
					}
				}
				return nil
			})
			if walkErr != nil {
				errs = append(errs, walkErr)
			}
		}
	}
	for _, image := range state.Images {
		path, err := abs(image.Path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := restoreSnapshot(fileSnapshot{Path: path, Exists: image.Exists, Content: image.Content}); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if _, err := bldb.Rehydrate(ctx, storageRoot, ws.DB); err != nil {
		return fmt.Errorf("rehydrate index after reconstitute rollback: %w", err)
	}
	return restoreReconstituteCommitLinks(ctx, ws, state, pairs)
}

func restoreReconstituteCommitLinks(
	ctx context.Context,
	ws *Workspace,
	state *reconstituteJournalState,
	pairs map[string]string,
) error {
	ids := append([]string{}, state.CommitItemIDs...)
	for _, newID := range pairs {
		ids = append(ids, newID)
	}
	for _, id := range ids {
		if _, err := ws.DB.ExecContext(ctx, `DELETE FROM commit_links WHERE item_id = ?`, id); err != nil {
			return fmt.Errorf("clear commit links for %s: %w", id, err)
		}
	}
	for _, link := range state.CommitLinks {
		if _, err := ws.DB.ExecContext(ctx,
			`INSERT OR REPLACE INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`,
			link.ItemID, link.CommitSHA, link.Message, link.Author); err != nil {
			return fmt.Errorf("restore commit link %s/%s: %w", link.ItemID, link.CommitSHA, err)
		}
	}
	return nil
}

// discoverReconstituteAdoptions finds the old->new ID pairs this operation
// created from its tagged adopted events, scanning only logs that did not
// exist before the operation. It also returns those unknown log paths.
func discoverReconstituteAdoptions(
	ctx context.Context,
	ws *Workspace,
	correlationID, logsRoot string,
	knownLogs map[string]struct{},
) (map[string]string, []string, error) {
	pairs := map[string]string{}
	entries, err := os.ReadDir(logsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return pairs, nil, nil
		}
		return nil, nil, fmt.Errorf("list logs: %w", err)
	}
	var unknown []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		rel := "logs/" + entry.Name()
		if _, ok := knownLogs[rel]; ok {
			continue
		}
		unknown = append(unknown, rel)
		itemID := strings.TrimSuffix(entry.Name(), ".jsonl")
		logged, readErr := events.ReadAllEvents(ctx, logsRoot, itemID)
		if readErr != nil {
			return nil, nil, fmt.Errorf("read log %s: %w", itemID, readErr)
		}
		for _, event := range logged {
			if event.EventType != "adopted" || !isShipmentOperationEvent(event, correlationID) {
				continue
			}
			oldID, _ := event.Delta["old_id"].(string)
			newID, _ := event.Delta["new_id"].(string)
			if oldID != "" && newID != "" && oldID != newID {
				pairs[oldID] = newID
			}
		}
	}
	return pairs, unknown, nil
}

// restoreReconstitutedItemLogs moves an adopted item's log back to its old ID
// under both item-log locks. Operation-tagged events are scrubbed afterwards,
// which preserves independent events (comments, commit tracking) that landed in
// the same log.
func restoreReconstitutedItemLogs(ctx context.Context, ws *Workspace, logsRoot, oldID, newID string) error {
	lockedCtx, release, err := lockAdoptionEventLogs(ctx, ws, oldID, newID)
	if err != nil {
		return fmt.Errorf("lock item logs %s/%s: %w", oldID, newID, err)
	}
	defer release()
	_ = lockedCtx
	newPath := events.LogPathForItem(logsRoot, newID)
	oldPath := events.LogPathForItem(logsRoot, oldID)
	if _, statErr := os.Stat(newPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return fmt.Errorf("inspect log %s: %w", newID, statErr)
	}
	if _, statErr := os.Stat(oldPath); statErr == nil {
		return nil
	}
	if err := os.Rename(newPath, oldPath); err != nil {
		return fmt.Errorf("restore log %s -> %s: %w", newID, oldID, err)
	}
	return nil
}

// rollbackReconstitutePreimage restores a journal that carries only shipment
// preimages (no reconstitute state): members, related shipments, then the
// primary shipment, and scrubs the operation's events.
func rollbackReconstitutePreimage(ctx context.Context, ws *Workspace, journal shipmentLifecycleJournal) error {
	governedCtx := context.WithValue(ctx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
		correlationID:                 journal.CorrelationID,
		operation:                     shipmentOperationReconstitute + "_recovery",
		allowGovernedShipmentMutation: true,
	})
	artifactIDs := []string{journal.ShipmentID}
	for _, member := range journal.Preimage.Members {
		if err := persistArtifact(ctx, ws, cloneArtifact(member), true); err != nil {
			return fmt.Errorf("restore member %s preimage: %w", member.ID, err)
		}
		artifactIDs = append(artifactIDs, member.ID)
	}
	for _, related := range journal.Preimage.Related {
		if err := persistArtifact(governedCtx, ws, cloneArtifact(related), false); err != nil {
			return fmt.Errorf("restore related shipment %s preimage: %w", related.ID, err)
		}
		artifactIDs = append(artifactIDs, related.ID)
	}
	if err := persistArtifact(governedCtx, ws, cloneArtifact(journal.Preimage.Shipment), false); err != nil {
		return fmt.Errorf("restore shipment %s preimage: %w", journal.ShipmentID, err)
	}
	if err := removeShipmentOperationEvents(ctx, ws, artifactIDs, journal.CorrelationID); err != nil {
		return fmt.Errorf("scrub reconstitute events: %w", err)
	}
	return nil
}
