package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// reconstituteJournalState is the durable preimage a reconstitute journal
// carries so rollback can restore the workspace after a failure or crash.
type reconstituteJournalState struct {
	Images       []reconstituteFileImage `json:"images"`
	KnownLogs    []string                `json:"known_logs"`
	CreatedIDs   []string                `json:"created_ids,omitempty"`
	ScopedImages bool                    `json:"scoped_images,omitempty"`
	// Adoptions maps each adopted item's old ID to its allocated new ID. It is
	// journaled before the rename so every rollback attempt can find the pair.
	Adoptions     map[string]string        `json:"adoptions,omitempty"`
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

func reconstituteImageOwners(state *reconstituteJournalState) ([]string, error) {
	var ids []string
	for _, image := range state.Images {
		if !strings.HasSuffix(strings.ToLower(image.Path), ".md") {
			continue
		}
		if id := reconstituteImageOwner(image); id != "" {
			ids = append(ids, id)
		}
	}
	return uniqueNonEmptyStrings(ids), nil
}

func reconstituteImageOwner(image reconstituteFileImage) string {
	if !image.Exists {
		return ""
	}
	fm, body, err := models.ParseFrontmatter(string(image.Content))
	if err != nil {
		return ""
	}
	art, err := models.ArtifactFromFrontmatter(fm, body)
	if err != nil || !validReconstituteID(art.ID) {
		return ""
	}
	return art.ID
}

// reconstituteScopedImageChangedOutsideOp checks fields the operation cannot
// change and, for reference-only owners, verifies the entire rewound artifact.
func reconstituteScopedImageChangedOutsideOp(
	preimage, current []byte, owner string, eventIDs []string, pairs map[string]string,
) (bool, error) {
	preFM, preBody, err := models.ParseFrontmatter(string(preimage))
	if err != nil {
		return true, nil
	}
	preArt, err := models.ArtifactFromFrontmatter(preFM, preBody)
	if err != nil || preArt == nil || !validReconstituteID(preArt.ID) {
		return true, nil
	}
	curFM, curBody, err := models.ParseFrontmatter(string(current))
	if err != nil {
		return true, nil
	}
	curArt, err := models.ArtifactFromFrontmatter(curFM, curBody)
	if err != nil || curArt == nil || curArt.ID != preArt.ID {
		return true, nil
	}
	if preArt.Title != curArt.Title || preArt.ArtifactType != curArt.ArtifactType ||
		preArt.Description != curArt.Description || preArt.Sprint != curArt.Sprint ||
		preArt.Priority != curArt.Priority || preArt.AssignedTo != curArt.AssignedTo ||
		preArt.Owner != curArt.Owner || preArt.Commit != curArt.Commit ||
		!preArt.CreatedAt.Equal(curArt.CreatedAt) || !slices.Equal(preArt.References, curArt.References) {
		return true, nil
	}
	mutableField := func(key string) bool {
		switch key {
		case "items", "origin_feature", "blocked_reason", "claim_marker", schedulerBaselineClaimKey:
			return true
		default:
			return false
		}
	}
	for key, value := range preArt.CustomFields {
		if !mutableField(key) {
			other, ok := curArt.CustomFields[key]
			if !ok || !reflect.DeepEqual(value, other) {
				return true, nil
			}
		}
	}
	for key, value := range curArt.CustomFields {
		if !mutableField(key) {
			other, ok := preArt.CustomFields[key]
			if !ok || !reflect.DeepEqual(value, other) {
				return true, nil
			}
		}
	}
	reverse := make(map[string]string, len(pairs))
	for oldID, newID := range pairs {
		reverse[newID] = oldID
	}
	rewind := func(id string) string {
		if oldID, ok := reverse[id]; ok {
			return oldID
		}
		return id
	}
	links := slices.Clone(curArt.Links)
	for i := range links {
		links[i].TargetID = rewind(links[i].TargetID)
	}
	if !slices.Equal(preArt.Links, links) {
		return true, nil
	}
	if len(eventIDs) > 0 && !containsString(eventIDs, owner) {
		rewound := cloneArtifact(curArt)
		rewound.ParentID = rewind(rewound.ParentID)
		for i := range rewound.Dependencies {
			rewound.Dependencies[i].ID = rewind(rewound.Dependencies[i].ID)
		}
		rewound.Links = links
		if rewound.ArtifactType == "shipment" {
			if _, ok := rewound.CustomFields["items"]; ok {
				items := NormalizeShipmentItems(rewound)
				for i := range items {
					items[i] = rewind(items[i])
				}
				rewound.CustomFields["items"] = items
			}
		}
		rewound.UpdatedAt = preArt.UpdatedAt
		matches, matchErr := recoveryArtifactMatchesAny(rewound, []recoveryArtifactCandidate{{artifact: preArt}})
		if matchErr != nil {
			return false, fmt.Errorf("compare rewound artifact: %w", matchErr)
		}
		if !matches {
			return true, nil
		}
	}
	return false, nil
}

// lockReconstituteOwners never waits for B while member log locks C are held.
func lockReconstituteOwners(ctx context.Context, ws *Workspace, ids []string) (context.Context, func() error, error) {
	ids = uniqueNonEmptyStrings(ids)
	sort.Strings(ids)
	var unlocks []func() error
	release := func() error {
		var errs []error
		for i := len(unlocks) - 1; i >= 0; i-- {
			errs = append(errs, unlocks[i]())
		}
		return errors.Join(errs...)
	}
	var heldIDs []string
	for _, id := range ids {
		unlock, err := lockArtifactMutation(ctx, ws, id)
		if err != nil {
			return ctx, nil, errors.Join(fmt.Errorf("lock artifact %s: %w", id, err), release())
		}
		unlocks = append(unlocks, unlock)
		heldIDs = append(heldIDs, artifactMutationLockID(id))
	}
	ctx = withArtifactMutationLocks(ctx, ids)
	ctx = withShipmentLifecycleHeldLocks(ctx, ws, heldIDs...)
	return ctx, release, nil
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
	for _, manifest := range st.manifests {
		for _, id := range manifest {
			add(id)
		}
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

// reconstituteDirectOwners identifies existing artifacts directly mutated by
// the operation, excluding synthetic plan keys and reference-only owners.
func reconstituteDirectOwners(st *reconstitutePlanState) []string {
	var owners []string
	for id := range st.sources {
		owners = append(owners, id)
	}
	for id := range st.moves {
		owners = append(owners, id)
	}
	for id := range st.edgeAdds {
		owners = append(owners, id)
	}
	for id := range st.edgeRemoves {
		owners = append(owners, id)
	}
	for _, rewire := range st.rewireItems {
		owners = append(owners, rewire.item)
	}
	for _, target := range st.targets {
		if target.existing != nil {
			owners = append(owners, target.key)
		}
		owners = append(owners, target.adopt...)
	}
	filtered := owners[:0]
	for _, id := range owners {
		if !strings.Contains(id, ":") {
			filtered = append(filtered, id)
		}
	}
	owners = uniqueNonEmptyStrings(filtered)
	sort.Strings(owners)
	return owners
}

// reconstituteSnapshotOwners includes only direct mutation targets and
// structured references that adoption can rewrite.
func reconstituteSnapshotOwners(ctx context.Context, ws *Workspace, st *reconstitutePlanState) ([]string, error) {
	owners := reconstituteDirectOwners(st)
	for _, target := range st.targets {
		if target.spec.KeepID {
			continue
		}
		for _, id := range target.adopt {
			refs, err := findCrossArtifactReferences(ctx, ws, id, "\x00")
			if err != nil {
				return nil, fmt.Errorf("reconstitute: discover references to %s: %w", id, err)
			}
			for _, ref := range refs {
				owners = append(owners, ref.artifact.ID)
			}
		}
	}
	filtered := owners[:0]
	for _, id := range owners {
		if !strings.Contains(id, ":") {
			filtered = append(filtered, id)
		}
	}
	owners = uniqueNonEmptyStrings(filtered)
	sort.Strings(owners)
	return owners, nil
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
	// Membership locks stabilize the source manifests before acquiring their
	// log locks. C must precede B and survive the final eligibility check.
	var memberIDs []string
	for _, id := range sourceIDs {
		shipment, readErr := readShipmentFromDiskReadOnly(ws, id)
		if readErr != nil {
			return nil, readErr
		}
		memberIDs = append(memberIDs, NormalizeShipmentItems(shipment)...)
	}
	lockedCtx, logUnlock, err := lockItemLogsSorted(ctx, ws, memberIDs)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: lock member logs: %w", err)
	}
	defer logUnlock()
	ctx = lockedCtx

	owners, err := reconstituteSnapshotOwners(ctx, ws, peek)
	if err != nil {
		return nil, err
	}
	owners = uniqueNonEmptyStrings(append(owners, peek.affectedIDs()...))
	sort.Strings(owners)
	lockedCtx, artifactUnlock, err := lockReconstituteOwners(ctx, ws, owners)
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
	lockedOwners := make(map[string]struct{}, len(owners))
	for _, id := range owners {
		lockedOwners[id] = struct{}{}
	}
	for _, id := range state.LockIDs {
		if _, ok := lockedOwners[id]; !ok {
			return nil, reconstituteValidationError("snapshot owner %s changed during lock acquisition", id)
		}
	}
	imageOwners, err := reconstituteImageOwners(state)
	if err != nil {
		return nil, err
	}
	snapshotOwners := make(map[string]struct{}, len(imageOwners))
	for _, id := range imageOwners {
		snapshotOwners[id] = struct{}{}
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
	var renamedUnlocks []func() error
	defer func() {
		for i := len(renamedUnlocks) - 1; i >= 0; i-- {
			if unlockErr := renamedUnlocks[i](); unlockErr != nil {
				slog.WarnContext(ctx, "release renamed artifact lock", "error", unlockErr)
			}
		}
	}()
	result, applyErr := applyReconstitution(opCtx, ws, st, correlationID, baseline, snapshotOwners, func(id string, feature bool) error {
		state.CreatedIDs = append(state.CreatedIDs, id)
		if _, err := writeShipmentOperationJournal(ws, journalName, journal); err != nil {
			return fmt.Errorf("reconstitute: journal created artifact %s: %w", id, errors.Join(err, blerrors.ErrWriteIndeterminate))
		}
		if feature {
			unlock, lockErr := lockArtifactMutation(opCtx, ws, id)
			if lockErr != nil {
				return fmt.Errorf("reconstitute: lock created feature %s: %w", id,
					errors.Join(lockErr, blerrors.ErrWriteIndeterminate))
			}
			renamedUnlocks = append(renamedUnlocks, unlock)
			lockedOwners[id] = struct{}{}
			snapshotOwners[id] = struct{}{}
			opCtx = withArtifactMutationLocks(opCtx, []string{id})
			opCtx = withShipmentLifecycleHeldLocks(opCtx, ws, artifactMutationLockID(id))
		}
		return nil
	}, func(adoptCtx context.Context, oldID, newID string) error {
		unlock, lockErr := lockArtifactMutation(adoptCtx, ws, newID)
		if lockErr != nil {
			return fmt.Errorf("reconstitute: lock renamed artifact %s: %w", newID, lockErr)
		}
		renamedUnlocks = append(renamedUnlocks, unlock)
		if state.Adoptions == nil {
			state.Adoptions = map[string]string{}
		}
		state.Adoptions[oldID] = newID
		if _, err := writeShipmentOperationJournal(ws, journalName, journal); err != nil {
			return fmt.Errorf("reconstitute: journal adoption %s->%s: %w", oldID, newID, errors.Join(err, blerrors.ErrWriteIndeterminate))
		}
		return nil
	})
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
	owners, err := reconstituteSnapshotOwners(ctx, ws, st)
	if err != nil {
		return nil, err
	}
	ownerSet := make(map[string]struct{}, len(owners))
	for _, id := range owners {
		ownerSet[id] = struct{}{}
	}
	state := &reconstituteJournalState{
		Images: []reconstituteFileImage{}, KnownLogs: []string{},
		ScopedImages: true,
		CommitLinks:  []reconstituteCommitLink{}, CommitItemIDs: []string{},
		EventIDs: reconstituteDirectOwners(st), LockIDs: uniqueNonEmptyStrings(append(append([]string{}, affected...), owners...)),
	}
	sort.Strings(state.LockIDs)
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
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			fm, body, parseErr := models.ParseFrontmatter(string(data))
			if parseErr != nil {
				return nil
			}
			art, parseErr := models.ArtifactFromFrontmatter(fm, body)
			if parseErr == nil {
				if _, ok := ownerSet[art.ID]; ok {
					state.Images = append(state.Images, reconstituteFileImage{Path: rel(path), Exists: true, Content: data})
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
	lockedOwners map[string]struct{},
	recordCreated func(id string, feature bool) error,
	recordAdoption func(ctx context.Context, oldID, newID string) error,
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
		if err := recordCreated(feature.ID, true); err != nil {
			return nil, err
		}
		ctx = withArtifactMutationLocks(ctx, []string{feature.ID})
		ctx = withShipmentLifecycleHeldLocks(ctx, ws, artifactMutationLockID(feature.ID))
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
				AdoptItemOptions{KeepID: target.spec.KeepID, BeforeRename: recordAdoption, allowedCrossRefIDs: lockedOwners})
			if err != nil {
				return nil, fmt.Errorf("adopt item %s under %s: %w", item, featureIDs[target.featureKey], err)
			}
			if adopted.NewID != "" && adopted.NewID != item {
				result.AdoptedIDMap[item] = adopted.NewID
				if _, ok := lockedOwners[item]; ok {
					lockedOwners[adopted.NewID] = struct{}{}
					ctx = withArtifactMutationLocks(ctx, []string{adopted.NewID})
					ctx = withShipmentLifecycleHeldLocks(ctx, ws, artifactMutationLockID(adopted.NewID))
				}
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
		if err := recordCreated(shipment.ID, false); err != nil {
			return nil, err
		}
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
		applyReconstituteRewiresToGraph(arts, st.rewireItems, finalID)
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

	if err := validateReconstituteJournalState(state); err != nil {
		return err
	}

	var errs []error
	storageRoot := workspaceStorageRoot(ws)
	logsRoot := WorkspaceLogsRoot(ws.RootPath)
	knownLogs := map[string]struct{}{}
	for _, rel := range state.KnownLogs {
		knownLogs[rel] = struct{}{}
	}
	discovered, err := discoverReconstituteAdoptions(ctx, ws, journal.CorrelationID, logsRoot, knownLogs)
	if err != nil {
		errs = append(errs, err)
	}
	// The journaled map survives a rollback that stops midway; discovery only
	// adds pairs a crashed operation did not get to journal.
	pairs := map[string]string{}
	for oldID, newID := range discovered {
		if validReconstituteID(oldID) && validReconstituteID(newID) {
			pairs[oldID] = newID
		}
	}
	for oldID, newID := range state.Adoptions {
		pairs[oldID] = newID
	}
	// Legacy snapshots included unrelated Markdown, so changed images cannot
	// safely be distinguished from edits made after the operation.
	for _, image := range state.Images {
		if !strings.HasSuffix(strings.ToLower(image.Path), ".md") {
			continue
		}
		path := filepath.Join(storageRoot, filepath.FromSlash(image.Path))
		if err := checkRecoveryPath(storageRoot, path); err != nil {
			return fmt.Errorf("journal path %q: %w", image.Path, err)
		}
		current, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			owner := reconstituteImageOwner(image)
			if owner == "" && image.Exists {
				return reconstituteValidationError("unidentifiable snapshot %s changed", image.Path)
			}
			if !state.ScopedImages && !containsString(state.LockIDs, owner) && image.Exists {
				return reconstituteValidationError("legacy snapshot %s changed", image.Path)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("read snapshot destination %s: %w", image.Path, err)
		}
		owner := reconstituteImageOwner(image)
		if owner == "" {
			if !image.Exists || !bytes.Equal(current, image.Content) {
				return reconstituteValidationError("unidentifiable snapshot %s changed", image.Path)
			}
			continue
		}
		if !state.ScopedImages && !containsString(state.LockIDs, owner) && (!image.Exists || !bytes.Equal(current, image.Content)) {
			return reconstituteValidationError("legacy snapshot %s changed", image.Path)
		}
		if !bytes.Equal(current, image.Content) {
			if reconstituteImageOwner(reconstituteFileImage{Exists: true, Content: current}) != owner {
				return reconstituteValidationError("snapshot destination %s has a different owner", image.Path)
			}
			if image.Exists {
				changedOutside, cmpErr := reconstituteScopedImageChangedOutsideOp(image.Content, current, owner, state.EventIDs, pairs)
				if cmpErr != nil {
					return fmt.Errorf("compare snapshot destination %s: %w", image.Path, cmpErr)
				}
				if changedOutside {
					return reconstituteValidationError("snapshot %s changed outside reconstitution", image.Path)
				}
			}
		}
	}
	abs := func(rel string) (string, error) {
		path := filepath.Join(storageRoot, filepath.FromSlash(rel))
		if err := checkRecoveryPath(storageRoot, path); err != nil {
			return "", fmt.Errorf("journal path %q: %w", rel, err)
		}
		return path, nil
	}

	oldIDs := make([]string, 0, len(pairs))
	for oldID := range pairs {
		oldIDs = append(oldIDs, oldID)
	}
	sort.Strings(oldIDs)
	restoreFailed := map[string]bool{}
	for _, oldID := range oldIDs {
		if err := restoreReconstitutedItemLogs(ctx, ws, logsRoot, oldID, pairs[oldID]); err != nil {
			errs = append(errs, err)
			restoreFailed[oldID] = true
		}
	}
	if err := removeShipmentOperationEvents(ctx, ws, state.EventIDs, journal.CorrelationID); err != nil {
		errs = append(errs, fmt.Errorf("scrub reconstitute events: %w", err))
	}
	// Remove only what this operation provably created: the journaled created
	// artifacts and the new-ID artifacts/logs of adoptions. A pair whose log
	// restoration failed keeps its new-ID log so no history is lost.
	ours := append([]string{}, state.CreatedIDs...)
	for _, oldID := range oldIDs {
		if !restoreFailed[oldID] {
			ours = append(ours, pairs[oldID])
		}
	}
	for _, id := range ours {
		if err := removeReconstituteCreated(ctx, ws, logsRoot, id); err != nil {
			errs = append(errs, err)
		}
	}
	for _, image := range state.Images {
		if strings.HasSuffix(strings.ToLower(image.Path), ".md") && reconstituteImageOwner(image) == "" {
			continue
		}
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

	if err := preserveReconstituteDBLinks(ctx, ws, pairs); err != nil {
		return err
	}
	if _, err := bldb.Rehydrate(ctx, storageRoot, ws.DB); err != nil {
		return fmt.Errorf("rehydrate index after reconstitute rollback: %w", err)
	}
	return restoreReconstituteCommitLinks(ctx, ws, state, pairs)
}

// preserveReconstituteDBLinks keeps database-only item links across the
// rollback rehydrate, which clears item_links. Links the adoption re-keyed to
// the new ID are first moved back to the old ID, then every remaining link is
// persisted to its source's Markdown frontmatter, as every other rehydrating
// caller does.
func preserveReconstituteDBLinks(ctx context.Context, ws *Workspace, pairs map[string]string) error {
	if len(pairs) > 0 {
		tx, err := ws.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin link restore after reconstitute rollback: %w", err)
		}
		defer tx.Rollback() //nolint:errcheck
		for oldID, newID := range pairs {
			if err := bldb.RewriteLinkEdges(ctx, tx, newID, oldID); err != nil {
				return fmt.Errorf("restore links for %s: %w", oldID, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit link restore after reconstitute rollback: %w", err)
		}
	}
	if _, err := MigrateDBOnlyLinksBeforeRehydrate(ctx, ws); err != nil {
		return fmt.Errorf("preserve database-only links before rollback rehydrate: %w", err)
	}
	return nil
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
// created from its tagged adopted events, reading only logs that did not
// exist before the operation. It never deletes anything.
func discoverReconstituteAdoptions(
	ctx context.Context,
	ws *Workspace,
	correlationID, logsRoot string,
	knownLogs map[string]struct{},
) (map[string]string, error) {
	pairs := map[string]string{}
	entries, err := os.ReadDir(logsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return pairs, nil
		}
		return nil, fmt.Errorf("list logs: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		rel := "logs/" + entry.Name()
		if _, ok := knownLogs[rel]; ok {
			continue
		}
		itemID := strings.TrimSuffix(entry.Name(), ".jsonl")
		logged, readErr := events.ReadAllEvents(ctx, logsRoot, itemID)
		if readErr != nil {
			return nil, fmt.Errorf("read log %s: %w", itemID, readErr)
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
	return pairs, nil
}

// removeReconstituteCreated deletes the artifact file and item log of an ID this
// operation created or adopted into. A missing file is not an error.
func removeReconstituteCreated(ctx context.Context, ws *Workspace, logsRoot, id string) error {
	if !validReconstituteID(id) {
		return fmt.Errorf("created id %q is invalid: %w", id, blerrors.ErrValidation)
	}
	storageRoot := workspaceStorageRoot(ws)
	var errs []error
	path, err := FindArtifactPath(ctx, ws, id)
	switch {
	case err == nil:
		if guardErr := checkRecoveryPath(storageRoot, path); guardErr != nil {
			errs = append(errs, fmt.Errorf("remove created artifact %s: %w", id, guardErr))
		} else if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			errs = append(errs, fmt.Errorf("remove created artifact %s: %w", id, rmErr))
		}
	case !errors.Is(err, blerrors.ErrNotFound):
		errs = append(errs, fmt.Errorf("locate created artifact %s: %w", id, err))
	}
	logPath := events.LogPathForItem(logsRoot, id)
	if guardErr := checkRecoveryPath(storageRoot, logPath); guardErr != nil {
		errs = append(errs, fmt.Errorf("remove created log %s: %w", id, guardErr))
	} else if rmErr := os.Remove(logPath); rmErr != nil && !os.IsNotExist(rmErr) {
		errs = append(errs, fmt.Errorf("remove created log %s: %w", id, rmErr))
	}
	return errors.Join(errs...)
}

// restoreReconstitutedItemLogs folds an adopted item's new-ID log back into its
// old-ID log under both item-log locks. When an independent writer recreated
// the old-ID log in the meantime, both logs are merged (every distinct event
// from either is kept, ordered by timestamp); operation-tagged events are
// scrubbed afterwards. The new-ID log is removed only after the merged log is
// durably written, so a failure never loses history and a retry is idempotent.
func restoreReconstitutedItemLogs(ctx context.Context, ws *Workspace, logsRoot, oldID, newID string) error {
	lockedCtx, release, err := lockAdoptionEventLogs(ctx, ws, oldID, newID)
	if err != nil {
		return fmt.Errorf("lock item logs %s/%s: %w", oldID, newID, err)
	}
	defer release()
	_ = lockedCtx
	storageRoot := workspaceStorageRoot(ws)
	newPath := events.LogPathForItem(logsRoot, newID)
	oldPath := events.LogPathForItem(logsRoot, oldID)
	for _, path := range []string{newPath, oldPath} {
		if err := checkRecoveryPath(storageRoot, path); err != nil {
			return fmt.Errorf("restore log %s -> %s: %w", newID, oldID, err)
		}
	}
	newContent, readErr := os.ReadFile(newPath)
	if os.IsNotExist(readErr) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("read log %s: %w", newID, readErr)
	}
	oldContent, readErr := os.ReadFile(oldPath)
	switch {
	case os.IsNotExist(readErr):
		if err := os.Rename(newPath, oldPath); err != nil {
			return fmt.Errorf("restore log %s -> %s: %w", newID, oldID, err)
		}
		return nil
	case readErr != nil:
		return fmt.Errorf("read log %s: %w", oldID, readErr)
	}
	merged, err := mergeItemLogContents(oldID, oldContent, newContent)
	if err != nil {
		return fmt.Errorf("merge logs %s <- %s: %w", oldID, newID, err)
	}
	if err := writeStringAtomically(oldPath, string(merged)); err != nil {
		return fmt.Errorf("write merged log %s: %w", oldID, err)
	}
	if err := os.Remove(newPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove merged log %s: %w", newID, err)
	}
	return nil
}

// mergeItemLogContents unions the event lines of two logs, dropping exact
// duplicate lines and ordering by event timestamp (stable, so equal timestamps
// keep first-then-second order).
func mergeItemLogContents(itemID string, first, second []byte) ([]byte, error) {
	type line struct {
		raw string
		at  time.Time
	}
	var lines []line
	seen := map[string]struct{}{}
	for _, content := range [][]byte{first, second} {
		for _, rawLine := range bytes.SplitAfter(content, []byte{'\n'}) {
			text := strings.TrimRight(string(rawLine), "\r\n")
			if strings.TrimSpace(text) == "" {
				continue
			}
			if _, dup := seen[text]; dup {
				continue
			}
			seen[text] = struct{}{}
			event, ok, err := events.ParseEventLine(text, itemID)
			if err != nil {
				return nil, fmt.Errorf("parse event line: %w", err)
			}
			entry := line{raw: text}
			if ok {
				entry.at = event.Timestamp
			}
			lines = append(lines, entry)
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].at.Before(lines[j].at) })
	var out bytes.Buffer
	for _, entry := range lines {
		out.WriteString(entry.raw)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
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
