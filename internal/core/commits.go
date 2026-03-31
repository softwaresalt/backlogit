package core

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// CommitLinkInfo contains the metadata linking an artifact to a git commit.
type CommitLinkInfo struct {
	ItemID    string `json:"item_id"`
	CommitSHA string `json:"commit_sha"`
	Message   string `json:"message"`
	Author    string `json:"author"`
}

// LinkCommit associates a git commit SHA with an artifact in the SQLite index.
func LinkCommit(ctx context.Context, db *sql.DB, ws *Workspace, itemID, commitSHA, message string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, '')`,
		itemID, commitSHA, message,
	)
	if err != nil {
		return fmt.Errorf("link commit: %w", err)
	}
	return nil
}

// GetCommitLinks retrieves all commit associations for a given artifact.
func GetCommitLinks(ctx context.Context, db *sql.DB, itemID string) ([]CommitLinkInfo, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT item_id, commit_sha, message, author FROM commit_links WHERE item_id = ?`,
		itemID,
	)
	if err != nil {
		return nil, fmt.Errorf("get commit links: %w", err)
	}
	defer rows.Close()

	var links []CommitLinkInfo
	for rows.Next() {
		var l CommitLinkInfo
		if err := rows.Scan(&l.ItemID, &l.CommitSHA, &l.Message, &l.Author); err != nil {
			return nil, fmt.Errorf("scan commit link: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// artifactIDPattern matches artifact IDs like T001, BUG003, US042.
var artifactIDPattern = regexp.MustCompile(`\b([A-Z]{1,6}\d{3,})\b`)

// AutoLinkCommits scans recent git log messages for artifact ID references and
// creates links automatically. depth=0 returns immediately with no links.
func AutoLinkCommits(ctx context.Context, db *sql.DB, ws *Workspace, depth int) ([]CommitLinkInfo, error) {
	if depth <= 0 {
		return nil, nil
	}

	out, err := exec.CommandContext(ctx, "git", "-C", ws.RootPath,
		"log", "--pretty=format:%H\t%ae\t%s", fmt.Sprintf("-n%d", depth)).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var linked []CommitLinkInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		sha, author, msg := parts[0], parts[1], parts[2]
		for _, match := range artifactIDPattern.FindAllString(msg, -1) {
			if err := LinkCommit(ctx, db, ws, match, sha, msg); err != nil {
				continue
			}
			linked = append(linked, CommitLinkInfo{
				ItemID: match, CommitSHA: sha, Message: msg, Author: author,
			})
		}
	}
	return linked, nil
}
