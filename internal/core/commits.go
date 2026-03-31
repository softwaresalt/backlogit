package core

import (
	"context"
	"database/sql"
)

// CommitLinkInfo contains the metadata linking an artifact to a git commit.
type CommitLinkInfo struct {
	ItemID    string `json:"item_id"`
	CommitSHA string `json:"commit_sha"`
	Message   string `json:"message"`
	Author    string `json:"author"`
}

// LinkCommit associates a git commit SHA with an artifact, storing the
// relationship in both the Markdown frontmatter and SQLite index.
//
// Worker: Parse the existing .md frontmatter. Set or append the commit SHA to
// the `commit` field. Write back the file. Update the DB record. Append a
// "commit_linked" event to events.jsonl.
func LinkCommit(ctx context.Context, db *sql.DB, ws *Workspace, itemID, commitSHA, message string) error {
	panic("not implemented: Worker: Link commit SHA to artifact frontmatter and DB, emit event")
}

// GetCommitLinks retrieves all commit associations for a given artifact.
//
// Worker: Query the commit_links table (or items.commit field) for the given
// item ID and return all linked commits.
func GetCommitLinks(ctx context.Context, db *sql.DB, itemID string) ([]CommitLinkInfo, error) {
	panic("not implemented: Worker: Retrieve all commit links for the given artifact from DB")
}

// AutoLinkCommits scans recent git log messages for artifact ID references
// (e.g., "T001" or "BUG003") and creates links automatically.
//
// Worker: Run `git log --oneline -N` (where N is configurable). Parse each
// message for artifact ID patterns matching the configured prefixes. Call
// LinkCommit for each match.
func AutoLinkCommits(ctx context.Context, db *sql.DB, ws *Workspace, depth int) ([]CommitLinkInfo, error) {
	panic("not implemented: Worker: Scan recent git log for artifact ID references and auto-link matching commits")
}
