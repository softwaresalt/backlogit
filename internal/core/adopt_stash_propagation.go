package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// adoptArchiveTxn tracks the archive-file mutations made while propagating an
// adopt rename so they can be restored byte-for-byte when the adopt fails.
type adoptArchiveTxn struct {
	unlock    func() error
	snapshots []adoptFileSnapshot
}

type adoptFileSnapshot struct {
	path    string
	raw     []byte
	existed bool
}

// Close releases the stash lock. It is safe on a nil receiver.
func (t *adoptArchiveTxn) Close() {
	if t == nil || t.unlock == nil {
		return
	}
	if err := t.unlock(); err != nil {
		slog.Warn("adopt item: release stash lock failed", "error", err)
	}
	t.unlock = nil
}

// Rollback restores every snapshotted file. It is safe on a nil receiver.
func (t *adoptArchiveTxn) Rollback() {
	if t == nil {
		return
	}
	for _, s := range t.snapshots {
		if !s.existed {
			if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
				slog.Warn("adopt item: rollback remove archive file failed", "path", s.path, "error", err)
			}
			continue
		}
		if err := writeFileAtomicBytes(s.path, s.raw); err != nil {
			slog.Warn("adopt item: rollback restore archive file failed", "path", s.path, "error", err)
		}
	}
}

func writeFileAtomicBytes(path string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".adopt-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmp := f.Name()
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close %s: %w", tmp, err)
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

// refuseSymlink returns an error when path exists and is a symlink.
func refuseSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("lstat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%q is a symlink; refusing to rewrite through it", path)
	}
	return nil
}

// propagateAdoptToStashArchive rewrites harvested_artifact_id in
// archive/stash.jsonl from oldID to newID and appends a successor correction
// for every stash whose latest provenance correction names oldID. It acquires
// the stash lock (the last lock in the canonical order: artifact mutation ->
// item log -> stash) and returns a transaction handle; the caller must Close it
// and call Rollback if the adopt later fails. A nil handle means there was
// nothing to propagate.
func propagateAdoptToStashArchive(ws *Workspace, oldID, newID string) (txn *adoptArchiveTxn, err error) {
	archiveDir := filepath.Join(workspaceStorageRoot(ws), "archive")
	if info, statErr := os.Lstat(archiveDir); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, nil
		}
		return nil, fmt.Errorf("lstat archive dir: %w", statErr)
	} else if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("archive directory %q is a symlink; refusing to rewrite through it", archiveDir)
	}
	stashPath := filepath.Join(archiveDir, stashArchiveFileName)
	correctionsPath := filepath.Join(archiveDir, "provenance_corrections.jsonl")
	for _, p := range []string{stashPath, correctionsPath} {
		if err := refuseSymlink(p); err != nil {
			return nil, err
		}
	}

	unlock, lockErr := lockStashFile(stashPath)
	if lockErr != nil {
		return nil, fmt.Errorf("acquire stash lock: %w", lockErr)
	}
	// Use a local: the named result is cleared by `return nil, err`, which
	// would make the deferred cleanup a no-op.
	local := &adoptArchiveTxn{unlock: unlock}
	txn = local
	defer func() {
		if err != nil {
			local.Rollback()
			local.Close()
			txn = nil
		}
	}()

	for _, p := range []string{stashPath, correctionsPath} {
		raw, readErr := os.ReadFile(p)
		switch {
		case readErr == nil:
			txn.snapshots = append(txn.snapshots, adoptFileSnapshot{path: p, raw: raw, existed: true})
		case os.IsNotExist(readErr):
			txn.snapshots = append(txn.snapshots, adoptFileSnapshot{path: p})
		default:
			return nil, fmt.Errorf("snapshot %s: %w", p, readErr)
		}
	}

	if rewritten, changed, rwErr := rewriteHarvestedArtifactID(txn.snapshots[0].raw, oldID, newID); rwErr != nil {
		return nil, rwErr
	} else if changed {
		if err := writeFileAtomicBytes(stashPath, rewritten); err != nil {
			return nil, err
		}
	}

	successors, scanErr := successorCorrections(txn.snapshots[1].raw, oldID, newID)
	if scanErr != nil {
		return nil, scanErr
	}
	for _, c := range successors {
		if appendErr := appendToProvenanceCorrections(correctionsPath, c); appendErr != nil {
			return nil, fmt.Errorf("append adopt provenance correction: %w", appendErr)
		}
	}
	return txn, nil
}

// rewriteHarvestedArtifactID rewrites matching lines of a stash archive and
// leaves every other line byte-identical.
func rewriteHarvestedArtifactID(raw []byte, oldID, newID string) ([]byte, bool, error) {
	if len(raw) == 0 || !bytes.Contains(raw, []byte(oldID)) {
		return raw, false, nil
	}
	lines := bytes.Split(raw, []byte("\n"))
	changed := false
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 || !bytes.Contains(line, []byte(oldID)) {
			continue
		}
		var rec map[string]json.RawMessage
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, false, fmt.Errorf("parse stash archive line %d: %w", i+1, err)
		}
		var harvested string
		if v, ok := rec["harvested_artifact_id"]; !ok || json.Unmarshal(v, &harvested) != nil || harvested != oldID {
			continue
		}
		encoded, err := json.Marshal(newID)
		if err != nil {
			return nil, false, fmt.Errorf("marshal new id: %w", err)
		}
		rec["harvested_artifact_id"] = encoded
		out, err := json.Marshal(rec)
		if err != nil {
			return nil, false, fmt.Errorf("marshal stash archive line %d: %w", i+1, err)
		}
		lines[i] = out
		changed = true
	}
	if !changed {
		return raw, false, nil
	}
	return bytes.Join(lines, []byte("\n")), true, nil
}

// successorCorrections returns a successor ProvenanceCorrection for each stash
// whose latest correction record points at oldID, in first-seen stash order.
func successorCorrections(raw []byte, oldID, newID string) ([]ProvenanceCorrection, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	latest := map[string]ProvenanceCorrection{}
	var order []string
	for i, line := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var c ProvenanceCorrection
		if err := json.Unmarshal(line, &c); err != nil {
			return nil, fmt.Errorf("parse provenance corrections line %d: %w", i+1, err)
		}
		if c.StashID == "" {
			continue
		}
		if _, seen := latest[c.StashID]; !seen {
			order = append(order, c.StashID)
		}
		latest[c.StashID] = c
	}
	var out []ProvenanceCorrection
	now := time.Now().UTC().Format(time.RFC3339)
	for _, id := range order {
		c := latest[id]
		if c.CanonicalDeliveryArtifactID != oldID {
			continue
		}
		c.CanonicalDeliveryArtifactID = newID
		c.Reason = fmt.Sprintf("adopt rename %s -> %s", oldID, newID)
		c.Actor = "backlogit"
		c.CorrectedAt = now
		if c.EventType == "" {
			c.EventType = "stash_provenance_corrected"
		}
		out = append(out, c)
	}
	return out, nil
}
