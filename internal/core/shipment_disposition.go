package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ShipmentDispositionOptions carries the operator-supplied inputs for a
// governed queued-shipment disposition. Every field except SupersededBy is
// required, and Confirm must be true.
type ShipmentDispositionOptions struct {
	Reason           string
	SupersededBy     string
	By               string
	AuthorizationRef string
	Confirm          bool
}

// disposeAfterGovernedWriteHook is a test seam. It is nil in production. It
// runs after the single governed write and before the journal is marked
// committed.
var disposeAfterGovernedWriteHook func() error

// authorizesDispositionKey is the deliberation custom field that names the
// shipment a deliberation authorizes disposing.
const authorizesDispositionKey = "authorizes_disposition"

// DisposeQueuedShipment moves a queued shipment to abandoned under a decided
// deliberation authorization. Inputs and authorization are validated first,
// then the lifecycle locks are taken and the target state is re-verified.
func DisposeQueuedShipment(ctx context.Context, ws *Workspace, id string, opts ShipmentDispositionOptions) (*models.Artifact, error) {
	id = strings.TrimSpace(id)
	opts.Reason = strings.TrimSpace(opts.Reason)
	opts.By = strings.TrimSpace(opts.By)
	opts.AuthorizationRef = strings.TrimSpace(opts.AuthorizationRef)
	opts.SupersededBy = strings.TrimSpace(opts.SupersededBy)

	if err := validateDispositionInputs(id, opts); err != nil {
		return nil, err
	}
	if err := validateDispositionRequest(ctx, ws, id, opts); err != nil {
		return nil, err
	}

	globalUnlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment lifecycle: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock", "shipment_id", id, "error", unlockErr)
		}
	}()
	ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockContextKey{}, struct{}{})
	if err := recoverPendingShipmentOperations(ctx, ws); err != nil {
		return nil, fmt.Errorf("recover pending shipment operations before dispose: %w", err)
	}

	membershipUnlock, err := lockShipmentMembership(ctx, ws, id)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s membership: %w", id, err)
	}
	defer func() {
		if unlockErr := membershipUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", id, "error", unlockErr)
		}
	}()

	current, err := loadDispositionTarget(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	memberIDs := NormalizeShipmentItems(current)
	lockIDs := uniqueNonEmptyStrings(append([]string{id}, memberIDs...))
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, lockIDs)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s disposition aggregate: %w", id, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment disposition aggregate locks", "shipment_id", id, "error", unlockErr)
		}
	}()

	// Re-read and re-validate everything under the locks: the pre-lock reads
	// above are advisory only.
	if err := validateDispositionRequest(lockedCtx, ws, id, opts); err != nil {
		return nil, err
	}
	current, err = loadDispositionTarget(lockedCtx, ws, id)
	if err != nil {
		return nil, err
	}
	if !slices.Equal(NormalizeShipmentItems(current), memberIDs) {
		return nil, fmt.Errorf("shipment %s changed while acquiring disposition locks: %w", id, blerrors.ErrShipmentConflict)
	}
	if err := requireNoLiveShipmentDependents(lockedCtx, ws, id); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("dispose queued shipment %s: %w", id, blerrors.ErrNotImplemented)
}

// validateDispositionInputs checks the caller-supplied options before any
// workspace read.
func validateDispositionInputs(id string, opts ShipmentDispositionOptions) error {
	if !opts.Confirm {
		return fmt.Errorf("dispose shipment %s: %w", id, blerrors.ErrConfirmationRequired)
	}
	if id == "" {
		return fmt.Errorf("dispose shipment: shipment id is required: %w", blerrors.ErrValidation)
	}
	if opts.Reason == "" {
		return fmt.Errorf("dispose shipment %s: reason is required: %w", id, blerrors.ErrValidation)
	}
	if opts.By == "" {
		return fmt.Errorf("dispose shipment %s: by is required: %w", id, blerrors.ErrValidation)
	}
	if opts.AuthorizationRef == "" {
		return fmt.Errorf("dispose shipment %s: authorization_ref is required: %w", id, blerrors.ErrValidation)
	}
	return nil
}

// validateDispositionRequest validates the successor reference and the
// authorization deliberation against the current Markdown.
func validateDispositionRequest(ctx context.Context, ws *Workspace, id string, opts ShipmentDispositionOptions) error {
	if opts.SupersededBy != "" {
		if opts.SupersededBy == id {
			return fmt.Errorf("dispose shipment %s: superseded_by must not name the shipment itself: %w", id, blerrors.ErrValidation)
		}
		successor, err := findArtifact(ctx, ws, opts.SupersededBy)
		switch {
		case errors.Is(err, blerrors.ErrNotFound):
			return fmt.Errorf("dispose shipment %s: superseded_by %s does not exist: %w", id, opts.SupersededBy, blerrors.ErrValidation)
		case err != nil:
			return fmt.Errorf("dispose shipment %s: load superseded_by %s: %w", id, opts.SupersededBy, err)
		case successor.ArtifactType != "shipment":
			return fmt.Errorf("dispose shipment %s: superseded_by %s is not a shipment: %w", id, opts.SupersededBy, blerrors.ErrValidation)
		}
	}
	return validateDispositionAuthorization(ctx, ws, id, opts.AuthorizationRef)
}

// validateDispositionAuthorization requires ref to resolve to a decided
// deliberation (live done, or archived with Markdown archived_status done)
// whose authorizes_disposition field names the target shipment.
func validateDispositionAuthorization(ctx context.Context, ws *Workspace, id, ref string) error {
	if deliberationIDPattern.FindString(ref) != ref {
		return fmt.Errorf("dispose shipment %s: authorization_ref %q is not a deliberation ID: %w", id, ref, blerrors.ErrValidation)
	}
	path, err := FindArtifactPath(ctx, ws, ref)
	if errors.Is(err, blerrors.ErrNotFound) {
		return fmt.Errorf("dispose shipment %s: authorization_ref %s does not resolve: %w", id, ref, blerrors.ErrValidation)
	}
	if err != nil {
		return fmt.Errorf("dispose shipment %s: resolve authorization_ref %s: %w", id, ref, err)
	}
	deliberation, _, err := parseFile(path)
	if err != nil {
		return fmt.Errorf("dispose shipment %s: authorization_ref %s is unreadable (%v): %w", id, ref, err, blerrors.ErrValidation)
	}
	if deliberation.ArtifactType != "deliberation" {
		return fmt.Errorf("dispose shipment %s: authorization_ref %s is a %s, not a deliberation: %w",
			id, ref, deliberation.ArtifactType, blerrors.ErrValidation)
	}
	decided := deliberation.Status == models.StatusDone ||
		(deliberation.Status == models.StatusArchived && deliberation.ArchivedStatus == string(models.StatusDone))
	if !decided {
		return fmt.Errorf("dispose shipment %s: authorization_ref %s is not decided (status %s): %w",
			id, ref, deliberation.Status, blerrors.ErrValidation)
	}
	authorized, _ := deliberation.CustomFields[authorizesDispositionKey].(string)
	if strings.TrimSpace(authorized) != id {
		return fmt.Errorf("dispose shipment %s: authorization_ref %s does not authorize this shipment: %w", id, ref, blerrors.ErrValidation)
	}
	return nil
}

// loadDispositionTarget loads the target from Markdown and refuses anything
// that is not a queued shipment.
func loadDispositionTarget(ctx context.Context, ws *Workspace, id string) (*models.Artifact, error) {
	current, err := findArtifact(ctx, ws, id)
	if errors.Is(err, blerrors.ErrNotFound) {
		return nil, fmt.Errorf("dispose shipment %s: %w: %w", id, blerrors.ErrShipmentNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("load shipment %s for disposition: %w", id, err)
	}
	if current.ArtifactType != "shipment" {
		return nil, fmt.Errorf("dispose shipment %s: artifact is a %s, not a shipment: %w",
			id, current.ArtifactType, blerrors.ErrShipmentDispositionRefused)
	}
	normalizeShipmentArtifact(current)
	if current.Status != models.StatusQueued {
		return nil, fmt.Errorf("dispose shipment %s: status is %s, only queued shipments can be disposed: %w",
			id, current.Status, blerrors.ErrShipmentDispositionRefused)
	}
	return current, nil
}

// requireNoLiveShipmentDependents refuses while a non-terminal shipment still
// holds a blocks edge onto the target. Candidates come from the dependency
// index and each edge is confirmed against the dependent's Markdown.
func requireNoLiveShipmentDependents(ctx context.Context, ws *Workspace, id string) error {
	edges, err := bldb.GetDependents(ctx, ws.DB, id)
	if err != nil {
		return fmt.Errorf("dispose shipment %s: load dependents: %w", id, err)
	}
	live := make([]string, 0, len(edges))
	for _, edge := range edges {
		if depType := strings.ToLower(strings.TrimSpace(edge.DepType)); depType != "" && depType != "blocks" {
			continue
		}
		dependent, loadErr := findArtifact(ctx, ws, edge.ItemID)
		if errors.Is(loadErr, blerrors.ErrNotFound) {
			continue
		}
		if loadErr != nil {
			return fmt.Errorf("dispose shipment %s: load dependent %s: %w", id, edge.ItemID, loadErr)
		}
		if dependent.ArtifactType != "shipment" || !shipmentHoldsBlocksEdge(dependent, id) {
			continue
		}
		switch dependent.Status {
		case models.StatusShipped, models.StatusAbandoned, models.StatusArchived:
			continue
		}
		live = append(live, dependent.ID)
	}
	if len(live) == 0 {
		return nil
	}
	sort.Strings(live)
	return fmt.Errorf("dispose shipment %s: live dependent shipments %s hold a blocks edge onto it: %w",
		id, strings.Join(live, ", "), blerrors.ErrShipmentDispositionRefused)
}

// shipmentHoldsBlocksEdge reports whether dependent lists targetID as a
// blocks prerequisite; an empty edge type defaults to blocks.
func shipmentHoldsBlocksEdge(dependent *models.Artifact, targetID string) bool {
	for _, edge := range dependent.Dependencies {
		depType := strings.ToLower(strings.TrimSpace(edge.Type))
		if edge.ID == targetID && (depType == "" || depType == "blocks") {
			return true
		}
	}
	return false
}
