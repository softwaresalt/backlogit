package docline

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
// escape is rejected with ErrPathEscapesWorkspace before a single write.
func ApplyMigration(plan MigrationPlan, opts Options) (Result, error) {
	var res Result
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
		if err := atomicWrite(abs, []byte(c.After)); err != nil {
			return res, fmt.Errorf("docline.ApplyMigration: write %s: %w", c.File, err)
		}
		res.Applied = append(res.Applied, c.File)
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
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relPosix := filepath.ToSlash(rel)
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

// atomicWrite writes data to path via a same-directory temp file and rename so a
// partial write can never leave a corrupt document. The temp file is chmod'd to
// match the original file's mode (or 0644 for a new file) so an in-place rewrite
// never silently downgrades permissions from the 0600 CreateTemp default.
func atomicWrite(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".docline-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
