package events

import (
	"context"
	"fmt"
)

// RewriteCheckpointFile is the only sanctioned in-place rewrite path for a
// stored checkpoint (147-F / U11). It is declared here as the seam every
// in-place rewrite must pass through — QuarantineCheckpoint's verbatim move
// and CleanupCheckpoints' rename are explicitly excluded, since neither
// parses or re-marshals.
//
// This declaration carries no working behaviour: landing the real
// read/ParseCheckpoint/ValidateCheckpoint/CheckConformingTopLevelNamespace/
// mutate/marshal/atomic-replace flow here, ahead of a failing test for that
// behaviour, would be exactly the Constitution Principle II carve-out the
// cycle-31 test lifecycle withdraws. The contract harness (147.035-T / U12)
// lands in a later wave against this declaration and observes it fail; the
// implementation (147.036-T / U13) then replaces this body. There is no
// caller of this seam until 147.037-T / U14 migrates ResolveCheckpoint onto
// it, so nothing observable on a live path changes in this unit.
func RewriteCheckpointFile(
	_ context.Context,
	_, _ string,
	_ func(*CheckpointV1) error,
) error {
	return fmt.Errorf("RewriteCheckpointFile: not yet implemented (147.035-T / U12 lands the contract harness; 147.036-T / U13 implements it)")
}
