package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// reconstituteIDPattern is the grammar every artifact ID in a reconstitute
// journal must satisfy: dot/dash-separated alphanumeric segments (for example
// 001.002-T, T001, 001-S). It excludes path separators, drive and stream
// colons, and ".." so an ID can never address a file outside its directory.
var reconstituteIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+(?:[._-][A-Za-z0-9]+)*$`)

func validReconstituteID(id string) bool {
	return id != "" && len(id) <= 255 && reconstituteIDPattern.MatchString(id)
}

// validateReconstituteJournalState rejects a journaled recovery state whose IDs
// or paths could address anything other than workspace artifacts. Recovery
// replays these values as deletes and writes, so a tampered journal must be
// refused before any of them is used.
func validateReconstituteJournalState(state *reconstituteJournalState) error {
	if state == nil {
		return nil
	}
	reject := func(kind, value string) error {
		return fmt.Errorf("reconstitute journal has an invalid %s %q: %w", kind, value, blerrors.ErrValidation)
	}
	for _, group := range []struct {
		kind string
		ids  []string
	}{
		{"created id", state.CreatedIDs},
		{"commit item id", state.CommitItemIDs},
		{"event id", state.EventIDs},
		{"lock id", state.LockIDs},
	} {
		for _, id := range group.ids {
			if !validReconstituteID(id) {
				return reject(group.kind, id)
			}
		}
	}
	for oldID, newID := range state.Adoptions {
		if !validReconstituteID(oldID) {
			return reject("adoption old id", oldID)
		}
		if !validReconstituteID(newID) {
			return reject("adoption new id", newID)
		}
	}
	for _, link := range state.CommitLinks {
		if !validReconstituteID(link.ItemID) {
			return reject("commit link item id", link.ItemID)
		}
	}
	for _, rel := range state.KnownLogs {
		name, ok := strings.CutPrefix(rel, "logs/")
		id, isLog := strings.CutSuffix(name, ".jsonl")
		if !ok || !isLog || !validReconstituteID(id) {
			return reject("known log path", rel)
		}
	}
	for _, image := range state.Images {
		if err := validateReconstituteImagePath(image.Path); err != nil {
			return err
		}
	}
	return nil
}

// validateReconstituteImagePath accepts only the file kinds a reconstitute
// snapshot captures: Markdown artifacts and the two stash archive files, as a
// clean, relative, slash-separated path with no traversal.
func validateReconstituteImagePath(rel string) error {
	bad := func() error {
		return fmt.Errorf("reconstitute journal has an invalid file image path %q: %w", rel, blerrors.ErrValidation)
	}
	if rel == "" || strings.ContainsAny(rel, "\\:\x00") || strings.HasPrefix(rel, "/") ||
		path.Clean(rel) != rel || filepath.IsAbs(rel) {
		return bad()
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return bad()
		}
	}
	switch rel {
	case "archive/" + stashArchiveFileName, "archive/provenance_corrections.jsonl":
		return nil
	}
	first := strings.SplitN(rel, "/", 2)[0]
	if !strings.HasSuffix(strings.ToLower(rel), ".md") || first == "ops" || first == "logs" ||
		strings.HasPrefix(first, ".") {
		return bad()
	}
	return nil
}

// checkRecoveryPath proves target resolves inside storageRoot without passing
// through a symlink or reparse point, so a recovery write or delete cannot be
// redirected outside the workspace. Components that do not exist yet are fine.
func checkRecoveryPath(storageRoot, target string) error {
	if !pathContained(storageRoot, target) {
		return fmt.Errorf("recovery path %q escapes the storage root: %w", target, blerrors.ErrValidation)
	}
	rel, err := filepath.Rel(filepath.Clean(storageRoot), filepath.Clean(target))
	if err != nil {
		return fmt.Errorf("recovery path %q: %w", target, err)
	}
	current := filepath.Clean(storageRoot)
	if rel == "." {
		return nil
	}
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			return nil
		}
		if statErr != nil {
			return fmt.Errorf("inspect recovery path %q: %w", current, statErr)
		}
		redirected, redirectErr := IsSymlinkOrReparsePoint(info, current)
		if redirectErr != nil {
			return fmt.Errorf("inspect recovery path %q: %w", current, redirectErr)
		}
		if redirected {
			return fmt.Errorf("recovery path %q passes through a symlink or reparse point: %w", current, blerrors.ErrValidation)
		}
	}
	return nil
}
