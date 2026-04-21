package core

import (
	"context"
	"database/sql"

	"github.com/softwaresalt/backlogit/internal/models"
)

// crossRefUpdate carries the data needed to atomically rewrite one referencing
// artifact's Markdown file and corresponding DB rows inside the adoption
// transaction. snapshotRaw holds the original file bytes for rollback.
type crossRefUpdate struct {
	artifact    *models.Artifact
	filePath    string
	snapshotRaw []byte
}

// findCrossArtifactReferences walks all artifact Markdown files in the
// workspace, collecting a crossRefUpdate for every artifact whose frontmatter
// references oldID in parent_id, dependencies, or links. The adopted artifact
// itself (identified by oldID and by newID when they differ) is excluded from
// results. Each returned Artifact struct already has oldID replaced by newID so
// callers can pass the slice directly to applyCrossArtifactRewrites.
//
// This function performs no writes and opens no transactions.
func findCrossArtifactReferences(
	_ context.Context,
	_ *Workspace,
	_, _ string,
) ([]crossRefUpdate, error) {
	panic("not implemented: findCrossArtifactReferences")
}

// applyCrossArtifactRewrites applies a batch of cross-artifact reference
// updates inside the provided transaction. For each update it:
//
//  1. Writes the updated Markdown file via atomic tmp+rename.
//  2. Upserts the artifact's items row via UpsertItemTx.
//  3. Deletes then reinserts the artifact's item_deps and item_links rows so
//     they reflect the updated in-memory state.
//
// On any write failure the function restores all previously written files from
// their snapshotRaw bytes before returning the error. The caller remains
// responsible for rolling back or committing the transaction.
func applyCrossArtifactRewrites(
	_ context.Context,
	_ *sql.Tx,
	_ *Workspace,
	_ []crossRefUpdate,
) error {
	panic("not implemented: applyCrossArtifactRewrites")
}
