package docline

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/core"
)

// Severity classifies a lint Finding.
type Severity string

const (
	// SeverityError marks a contract violation that fails the authoring gate.
	SeverityError Severity = "error"
	// SeverityWarn marks an advisory finding that does not fail the gate.
	SeverityWarn Severity = "warning"
)

// Action is the migration action computed for a single file.
type Action string

const (
	// ActionInsert adds a frontmatter block to a file that had none.
	ActionInsert Action = "insert"
	// ActionUpdate rewrites an existing frontmatter block.
	ActionUpdate Action = "update"
	// ActionNoop indicates the file already matches its normalized form.
	ActionNoop Action = "noop"
)

// Finding is a single lint result for one file/field.
type Finding struct {
	File     string   // repo-relative POSIX path
	Field    string   // the offending frontmatter field
	Rule     string   // the rule that failed
	Severity Severity // error | warning
	Fix      string   // actionable remediation hint
}

// Change is the planned migration delta for one file. Before/After hold the full
// file bytes; BodyBytesChanged MUST be false for every legitimate change (the
// migration only rewrites the frontmatter block).
type Change struct {
	File             string // repo-relative POSIX path
	Action           Action
	Before           string
	After            string
	BodyBytesChanged bool
}

// MigrationPlan is the dry-run result of PlanMigration: an ordered, deterministic
// set of per-file changes computed without writing anything.
type MigrationPlan struct {
	Changes []Change
}

// Result reports the outcome of ApplyMigration.
type Result struct {
	Applied []string // files written (repo-relative POSIX)
	Skipped []string // files left unchanged (noop)
}

// Options configures the docline application service. Root is the workspace root
// used for path containment; Path optionally narrows the operation to a
// repo-relative sub-path (empty = the full in-scope tree).
type Options struct {
	Root    string
	Path    string
	Profile Profile
	Now     time.Time
}

// LintTree returns per-file validation findings for the in-scope tree without
// mutating any file.
func LintTree(opts Options) ([]Finding, error) {
	files, err := collectInScopeDocs(opts.Root, opts.Path)
	if err != nil {
		return nil, err
	}
	profile := opts.Profile
	if profile == "" {
		profile = ProfileAuthoring
	}

	var findings []Finding
	for _, rel := range files {
		md, err := decodeDoc(opts.Root, rel)
		if err != nil {
			return nil, err
		}
		b := FromMap(frontmatterOrEmpty(md))
		for _, v := range ValidateFields(b, profile) {
			findings = append(findings, Finding{
				File:     rel,
				Field:    v.Field,
				Rule:     v.Rule,
				Severity: SeverityError,
				Fix:      v.Msg,
			})
		}
	}
	return findings, nil
}

// PlanMigration computes a dry-run MigrationPlan without writing anything.
func PlanMigration(opts Options) (MigrationPlan, error) {
	files, err := collectInScopeDocs(opts.Root, opts.Path)
	if err != nil {
		return MigrationPlan{}, err
	}

	changes := make([]Change, 0, len(files))
	for _, rel := range files {
		abs, err := core.SafeResolve(opts.Root, rel)
		if err != nil {
			return MigrationPlan{}, fmt.Errorf("docline.PlanMigration: %s: %w", rel, ErrPathEscapesWorkspace)
		}
		raw, err := os.ReadFile(abs)
		if err != nil {
			return MigrationPlan{}, fmt.Errorf("docline.PlanMigration: read %s: %w", rel, err)
		}
		normalized, err := Normalize(rel, raw, NormalizeOptions{Now: opts.Now})
		if err != nil {
			return MigrationPlan{}, fmt.Errorf("docline.PlanMigration: normalize %s: %w", rel, err)
		}

		c := Change{
			File:             rel,
			Before:           string(raw),
			After:            string(normalized),
			BodyBytesChanged: bodyBytesChanged(raw, normalized),
		}
		switch {
		case bytes.Equal(raw, normalized):
			c.Action = ActionNoop
		case !hadFrontmatter(raw):
			c.Action = ActionInsert
		default:
			c.Action = ActionUpdate
		}
		changes = append(changes, c)
	}
	return MigrationPlan{Changes: changes}, nil
}

// ApplyMigration writes the planned changes atomically (temp + rename) and is
// path-contained: every target is resolved through core.SafeResolve and any
// escape is rejected with ErrPathEscapesWorkspace before a single write. All
// non-noop changes are validated in a preflight pass, so an invalid later
// change cannot leave earlier files partially migrated. As a final TOCTOU guard,
// every target is re-read at apply time and the apply aborts with ErrConcurrentEdit
// (zero writes) if any on-disk bytes diverged from the plan-time Before.
func ApplyMigration(plan MigrationPlan, opts Options) (Result, error) {
	var res Result

	// Preflight: validate every non-noop change before writing anything. A body
	// mutation or path escape on any change aborts the whole apply with zero
	// writes, preserving the all-or-nothing guarantee in the doc comment.
	type pendingWrite struct {
		file   string
		abs    string
		before string
		data   []byte
	}
	var writes []pendingWrite
	for _, c := range plan.Changes {
		if c.Action == ActionNoop {
			res.Skipped = append(res.Skipped, c.File)
			continue
		}
		if c.BodyBytesChanged {
			return res, fmt.Errorf("docline.ApplyMigration: %s: %w", c.File, ErrBodyMutated)
		}
		abs, err := core.SafeResolve(opts.Root, c.File)
		if err != nil {
			return res, fmt.Errorf("docline.ApplyMigration: %s: %w", c.File, ErrPathEscapesWorkspace)
		}
		writes = append(writes, pendingWrite{file: c.File, abs: abs, before: c.Before, data: []byte(c.After)})
	}

	// TOCTOU guard: re-read every target and verify the on-disk bytes still match
	// the plan-time Before before the write loop begins. A file already edited
	// between plan and apply aborts the whole apply with zero writes. This detects
	// edits up to the moment apply starts; it is not a lock, so an edit landing
	// mid-write-loop is not guaranteed to be caught — external locking is required
	// for that. Containment is enforced lexically via core.SafeResolve above; this
	// guard does not add symlink-based realpath containment.
	for _, w := range writes {
		current, err := os.ReadFile(w.abs)
		if err != nil {
			// A target removed between plan and apply is itself a plan/apply
			// divergence; surface it as ErrConcurrentEdit so callers can detect
			// all concurrent-change cases uniformly via errors.Is.
			if errors.Is(err, os.ErrNotExist) {
				return res, fmt.Errorf("docline.ApplyMigration: %s: %w", w.file, ErrConcurrentEdit)
			}
			return res, fmt.Errorf("docline.ApplyMigration: re-read %s: %w", w.file, err)
		}
		if string(current) != w.before {
			return res, fmt.Errorf("docline.ApplyMigration: %s: %w", w.file, ErrConcurrentEdit)
		}
	}

	// All changes validated; perform the writes.
	for _, w := range writes {
		if err := atomicWrite(w.abs, w.data); err != nil {
			return res, fmt.Errorf("docline.ApplyMigration: write %s: %w", w.file, err)
		}
		res.Applied = append(res.Applied, w.file)
	}
	return res, nil
}

// collectInScopeDocs walks the workspace (optionally narrowed to subPath) and
// returns the sorted repo-relative POSIX paths of in-scope markdown files.
func collectInScopeDocs(root, subPath string) ([]string, error) {
	base, err := core.SafeResolve(root, subPath)
	if err != nil {
		return nil, fmt.Errorf("docline.collectInScopeDocs: %s: %w", subPath, ErrPathEscapesWorkspace)
	}

	var out []string
	walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relPosix := filepath.ToSlash(rel)
		if d.IsDir() {
			if relPosix == "." {
				return nil
			}
			// Skip excluded subtrees (docs/memory/, docs/archive/, .github/)
			// and any top-level directory other than docs/ (cmd/, internal/,
			// schemas/, ...) wholesale, rather than descending and filtering
			// file-by-file. Only docs/** plus a few root-level knowledge files
			// can ever be in scope, and root files are still visited directly.
			if isExcludedDir(relPosix) {
				return filepath.SkipDir
			}
			if !strings.Contains(relPosix, "/") && relPosix != "docs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		if !inScope(relPosix) {
			return nil
		}
		out = append(out, relPosix)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("docline.collectInScopeDocs: walk: %w", walkErr)
	}
	sort.Strings(out)
	return out, nil
}

// decodeDoc reads and decodes a repo-relative doc through SafeResolve.
func decodeDoc(root, rel string) (*Markdown, error) {
	abs, err := core.SafeResolve(root, rel)
	if err != nil {
		return nil, fmt.Errorf("docline.decodeDoc: %s: %w", rel, ErrPathEscapesWorkspace)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("docline.decodeDoc: read %s: %w", rel, err)
	}
	md, err := Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("docline.decodeDoc: decode %s: %w", rel, err)
	}
	return md, nil
}

// frontmatterOrEmpty returns the decoded frontmatter map, never nil.
func frontmatterOrEmpty(md *Markdown) map[string]any {
	if md == nil || md.Frontmatter == nil {
		return map[string]any{}
	}
	return md.Frontmatter
}

// hadFrontmatter reports whether raw began with a frontmatter block.
func hadFrontmatter(raw []byte) bool {
	md, err := Decode(raw)
	if err != nil {
		return false
	}
	return md.HasFrontmatter
}

// bodyBytesChanged reports whether the decoded body bytes differ between before
// and after. A true result is a hard invariant violation for a migration.
func bodyBytesChanged(before, after []byte) bool {
	mb, err1 := Decode(before)
	ma, err2 := Decode(after)
	if err1 != nil || err2 != nil {
		return true
	}
	return !bytes.Equal(mb.Body, ma.Body)
}

// ValidateApplyPath enforces that a migration apply is narrowed to a real
// sub-path of the workspace. It rejects an empty scope and any path that
// resolves to the workspace root itself (e.g. ".", "docs/.."), closing the
// whole-tree apply rail that the per-surface CLI/MCP "path is required" guards
// miss because core.SafeResolve treats the root as a valid in-bounds target.
// Both the CLI and MCP apply boundaries call this so they cannot drift.
func ValidateApplyPath(workspaceRoot, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("docline.ValidateApplyPath: apply requires an explicit sub-path: %w", ErrWholeTreeApply)
	}
	abs, err := core.SafeResolve(workspaceRoot, path)
	if err != nil {
		return fmt.Errorf("docline.ValidateApplyPath: %w", ErrPathEscapesWorkspace)
	}
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("docline.ValidateApplyPath: resolve root: %w", err)
	}
	if abs == filepath.Clean(absRoot) {
		return fmt.Errorf("docline.ValidateApplyPath: path %q resolves to the workspace root: %w", path, ErrWholeTreeApply)
	}
	return nil
}

// atomicWrite writes data to path atomically by delegating to the shared
// internal/atomicfile.WriteFileAtomic primitive (temp + rename, clamped mode
// preservation, Windows rename fallback, sync-free). The destination path is
// already containment-checked by ApplyMigration via core.SafeResolve before this
// is reached, satisfying atomicfile's path-agnostic caller-validates contract.
func atomicWrite(path string, data []byte) error {
	return atomicfile.WriteFileAtomic(path, data)
}
