package core

import (
	"context"
	"fmt"
	"time"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/gateproof"
	"github.com/softwaresalt/backlogit/internal/models"
)

// computeManifestDigest returns internal/canonical.Hash over the shipment's
// ORDERED manifest membership, its derived covering feature (empty string
// when none), and the resolved shipment head — the authoritative projection
// (106-F F1/U7). Reordering members, dropping a member, or swapping the
// covering feature each change the canonical bytes and therefore the digest,
// so any of those manifest mutations invalidate a previously bound proof.
func computeManifestDigest(ctx context.Context, ws *Workspace, shipment *models.Artifact, shipmentHead string) (string, error) {
	items := NormalizeShipmentItems(shipment)
	itemsAny := make([]any, len(items))
	for i, id := range items {
		itemsAny[i] = id
	}

	coveringID := ""
	if covering, ok := DeriveCoveringFeature(ctx, ws, shipment); ok {
		coveringID = covering.ID
	}

	payload := map[string]any{
		"items":            itemsAny,
		"covering_feature": coveringID,
		"shipment_head":    shipmentHead,
	}
	digest, err := canonical.Hash(payload)
	if err != nil {
		return "", fmt.Errorf("hash shipment manifest: %w", err)
	}
	return digest, nil
}

// augmentShipmentDeltaWithFormalProof is the shipment-purpose counterpart to
// augmentDeltaWithFormalProof (U4): it signs a purpose=shipment envelope
// carrying the required manifest_digest (U7), so a shipment-level pass event
// can be formally bound to the exact manifest membership, covering feature,
// and shipment head it was recorded against. It is a no-op — delta returned
// unchanged — when formal admission is neither enabled nor required.
func (ws *Workspace) augmentShipmentDeltaWithFormalProof(ctx context.Context, shipment *models.Artifact, shipmentID, shipmentHead string, delta map[string]any) error {
	if !ws.formalGateEnforced() {
		return nil
	}
	var formalCfg config.FormalGateConfig
	if ws.Config != nil && ws.Config.FormalGate != nil {
		formalCfg = *ws.Config.FormalGate
	}

	key, keyErr := config.ResolveFormalGateKey()
	if keyErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, keyErr)
	}

	digest, digestErr := computeManifestDigest(ctx, ws, shipment, shipmentHead)
	if digestErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, digestErr)
	}

	counter, unlock, counterErr := nextGateEvidenceCounter(ctx, ws, shipmentID)
	if counterErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, counterErr)
	}
	defer unlock()

	ran, _ := delta["ran"].(bool)
	timestampUTC := time.Now().UTC().Format(time.RFC3339)
	env := gateproof.Envelope{
		Magic:          gateproof.Magic,
		Purpose:        gateproof.PurposeShipment,
		Schema:         gateproof.Schema,
		Alg:            gateproof.AlgHMACSHA256,
		KeyID:          formalCfg.KeyID,
		WorkspaceID:    workspaceIdentity(ws.RootPath),
		ItemID:         shipmentID,
		EventType:      EventGatePassed,
		Ran:            ran,
		Actor:          "backlogit",
		TimestampUTC:   timestampUTC,
		HeadSHA:        shipmentHead,
		ReportDigest:   "", // shipment-level evidence has no per-task formal report to bind.
		Counter:        counter,
		ManifestDigest: digest,
	}

	proof, signErr := gateproof.Sign(env, key)
	if signErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, signErr)
	}

	delta["proof"] = proof
	delta["key_id"] = formalCfg.KeyID
	delta["proof_schema"] = gateproof.Schema
	delta["counter"] = counter
	delta["timestamp_utc"] = timestampUTC
	delta["manifest_digest"] = digest
	return nil
}

// verifyShipmentManifestBinding recomputes the manifest digest from CURRENT
// live state and re-verifies it against the proof just signed and appended,
// refusing on either ErrProofInvalid (definitively wrong — e.g. the manifest
// changed between signing and this check) or ErrProofUnverifiable (could not
// be evaluated at all). This is additive defense-in-depth alongside the
// existing head_sha ancestry and head-drift guards (unchanged): even within a
// single locked completion, a self-consistency re-check catches a
// computation bug or an unexpected intervening mutation rather than trusting
// the just-signed proof blindly. It is a no-op when formal admission is not
// enforced.
func (ws *Workspace) verifyShipmentManifestBinding(ctx context.Context, shipment *models.Artifact, shipmentID, shipmentHead string, delta map[string]any) error {
	if !ws.formalGateEnforced() {
		return nil
	}
	key, keyErr := config.ResolveFormalGateKey()
	if keyErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrFormalGateRequired, keyErr)
	}

	proof, _ := delta["proof"].(string)
	if proof == "" {
		return fmt.Errorf("%w: shipment evidence carries no proof to verify", bkerrors.ErrProofUnverifiable)
	}
	keyID, _ := delta["key_id"].(string)
	schemaVal, _ := delta["proof_schema"].(int)
	counterVal, _ := delta["counter"].(int64)
	timestampUTC, _ := delta["timestamp_utc"].(string)
	recordedDigest, _ := delta["manifest_digest"].(string)
	ran, _ := delta["ran"].(bool)

	freshDigest, digestErr := computeManifestDigest(ctx, ws, shipment, shipmentHead)
	if digestErr != nil {
		return fmt.Errorf("%w: %v", bkerrors.ErrProofUnverifiable, digestErr)
	}
	if freshDigest != recordedDigest {
		return fmt.Errorf("%w: manifest changed since evidence was signed (recorded %q, fresh %q)",
			bkerrors.ErrProofInvalid, recordedDigest, freshDigest)
	}

	env := gateproof.Envelope{
		Magic:          gateproof.Magic,
		Purpose:        gateproof.PurposeShipment,
		Schema:         schemaVal,
		Alg:            gateproof.AlgHMACSHA256,
		KeyID:          keyID,
		WorkspaceID:    workspaceIdentity(ws.RootPath),
		ItemID:         shipmentID,
		EventType:      EventGatePassed,
		Ran:            ran,
		Actor:          "backlogit",
		TimestampUTC:   timestampUTC,
		HeadSHA:        shipmentHead,
		ReportDigest:   "",
		Counter:        counterVal,
		ManifestDigest: recordedDigest,
	}
	if verifyErr := gateproof.Verify(env, proof, key); verifyErr != nil {
		return verifyErr
	}
	return nil
}
