package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

// DoctorTargetKind classifies the outcome of a single-file doctor validation so
// the CLI (U2) can map each outcome to a stable, versioned process exit code.
type DoctorTargetKind string

const (
	// DoctorTargetPass indicates the target file validated cleanly.
	DoctorTargetPass DoctorTargetKind = "pass"
	// DoctorTargetValidation indicates the file parsed but failed header-def
	// required-field validation.
	DoctorTargetValidation DoctorTargetKind = "validation"
	// DoctorTargetScope indicates the target path resolved outside the
	// workspace storage root (.backlogit) — a scope-confinement violation.
	DoctorTargetScope DoctorTargetKind = "scope"
	// DoctorTargetIO indicates a system/config fault prevented completing
	// validation: an unreadable or undecodable target file, a lock-sidecar IO
	// failure, or an absent workspace header-def schema (nil HeaderDef). It maps
	// to exit 3 and is distinct from a user-correctable required-field validation
	// failure (DoctorTargetValidation, exit 1) — the target artifact may be
	// well-formed; the fault is that validation could not be performed.
	DoctorTargetIO DoctorTargetKind = "io"
	// DoctorTargetBusy indicates the per-task advisory lock was already held by
	// a concurrent mutation/validation (set by U5). Never blocks.
	DoctorTargetBusy DoctorTargetKind = "busy"
	// DoctorTargetTimeout indicates the caller's deadline (U2's 5s bound) fired
	// before validation completed. DoctorTarget itself never returns this — the
	// CLI constructs it on ctx.Done — but it is part of the versioned contract.
	DoctorTargetTimeout DoctorTargetKind = "timeout"
)

// DoctorTargetResult is the concrete result of validating exactly one
// .backlogit artifact file against the header-def schema. It carries a
// classified Kind so the CLI can map it to the versioned exit-code contract and
// emit a versioned target-mode JSON schema.
type DoctorTargetResult struct {
	// Mode is a stable discriminator distinguishing this payload from the
	// full-scan DoctorReport shape.
	Mode string `json:"mode"`
	// SchemaVersion pins the target-mode JSON schema for downstream parsers.
	SchemaVersion string `json:"schema_version"`
	// Path echoes the caller-supplied target path.
	Path string `json:"path"`
	// ArtifactID is the resolved artifact ID when the file decoded.
	ArtifactID string `json:"artifact_id,omitempty"`
	// ArtifactType is the resolved artifact type when the file decoded.
	ArtifactType string `json:"artifact_type,omitempty"`
	// OK is true only when Kind == DoctorTargetPass.
	OK bool `json:"ok"`
	// Kind classifies the outcome for the exit-code mapping.
	Kind DoctorTargetKind `json:"kind"`
	// FieldErrors lists missing/invalid header-def fields on a validation fail.
	FieldErrors []string `json:"field_errors,omitempty"`
	// Message is a human-readable summary of a non-pass outcome.
	Message string `json:"message,omitempty"`
}

// doctorTargetSchemaVersion pins the versioned target-mode JSON schema. Bump on
// any breaking change to DoctorTargetResult field names/shape.
const doctorTargetSchemaVersion = "1.0.0"

// newDoctorTargetResult builds a result stamped with the mode discriminator and
// schema version so every return path is consistently versioned.
func newDoctorTargetResult(path string, kind DoctorTargetKind) *DoctorTargetResult {
	return &DoctorTargetResult{
		Mode:          "target",
		SchemaVersion: doctorTargetSchemaVersion,
		Path:          path,
		Kind:          kind,
		OK:            kind == DoctorTargetPass,
	}
}

// NewDoctorTargetResult builds a versioned target-mode result for the given
// path and kind. Exported so the CLI can construct the timeout result (kind
// DoctorTargetTimeout) on ctx.Done without duplicating the schema-version stamp.
func NewDoctorTargetResult(path string, kind DoctorTargetKind) *DoctorTargetResult {
	return newDoctorTargetResult(path, kind)
}

// DoctorTarget validates a single .backlogit artifact file against the
// header-def schema (required-field presence) and returns a typed, classified
// result. It is context-free at the core layer; the 5s timeout is applied by
// the caller (U2).
//
// DoctorTarget owns the per-task advisory lock for the whole read+validate and
// releases it via defer. It is the right entry point for SYNCHRONOUS callers
// (e.g. the MCP handler) whose deferred unlock is guaranteed to run. Callers
// that enforce their own wall-clock timeout in a goroutine (the CLI) MUST NOT
// use DoctorTarget — a timeout could return while this function is still
// running, and if the process then exits via os.Exit the deferred unlock never
// runs, stranding the lock sidecar. Those callers use PrepareDoctorTarget (lock
// owned in a frame whose defer is guaranteed to run) plus the lock-free
// ValidateDoctorTargetResolved.
//
// Scope confinement (invariant #2): the target path is resolved against the
// workspace storage root (WorkspaceStorageRoot) — the same boundary used by the
// artifact search dirs — and any path outside that root is rejected with the
// scope kind. This is the single source of truth for the .backlogit boundary;
// no bespoke prefix string is introduced.
func DoctorTarget(ws *Workspace, filePath string) (*DoctorTargetResult, error) {
	absTarget, unlock, short := PrepareDoctorTarget(ws, filePath)
	if short != nil {
		return short, nil
	}
	defer func() { _ = unlock() }()
	return ValidateDoctorTargetResolved(ws, filePath, absTarget), nil
}

// PrepareDoctorTarget confines filePath to the workspace storage root and
// acquires the per-task advisory lock. On a scope rejection or a lock failure
// (busy/IO) it returns a terminal short result (short != nil) and unlock == nil.
// On success it returns the resolved absolute target plus an unlock func that
// the caller MUST own in a frame whose deferred call is guaranteed to run, so a
// caller-enforced timeout cannot strand the lock sidecar (see DoctorTarget doc).
func PrepareDoctorTarget(ws *Workspace, filePath string) (absTarget string, unlock func() error, short *DoctorTargetResult) {
	resolved, ok, err := confineToStorageRoot(ws, filePath)
	if err != nil {
		return "", nil, newDoctorTargetResult(filePath, DoctorTargetScope)
	}
	if !ok {
		res := newDoctorTargetResult(filePath, DoctorTargetScope)
		res.Message = fmt.Sprintf("path outside workspace storage root: %s", filePath)
		return "", nil, res
	}

	// U5: acquire the per-task advisory lock before any read so a concurrent
	// mutation cannot modify the task while it is under validation. Acquisition
	// is NON-BLOCKING: a held lock yields the busy kind (→ exit 4 in U2), never
	// a mid-write read and never a block.
	u, lockErr := lockTaskFile(resolved)
	if lockErr != nil {
		if errors.Is(lockErr, ErrTaskBusy) {
			res := newDoctorTargetResult(filePath, DoctorTargetBusy)
			res.Message = fmt.Sprintf("task is locked by a concurrent operation: %v", lockErr)
			return "", nil, res
		}
		// A non-contention lock failure (e.g. permission/IO error creating the
		// sidecar) is an IO fault (exit 3), not busy (exit 4): preserve the
		// exit-code contract rather than reporting misleading contention.
		res := newDoctorTargetResult(filePath, DoctorTargetIO)
		res.Message = fmt.Sprintf("acquire task lock: %v", lockErr)
		return "", nil, res
	}
	return resolved, u, nil
}

// ValidateDoctorTargetResolved runs the lock-free read + decode + header-def
// validation against a PRE-CONFINED absolute target (as returned by
// PrepareDoctorTarget). It holds no lock, so it is safe to run inside a
// caller-enforced timeout goroutine: an abandoned run strands nothing.
func ValidateDoctorTargetResolved(ws *Workspace, filePath, absTarget string) *DoctorTargetResult {
	data, readErr := os.ReadFile(absTarget)
	if readErr != nil {
		res := newDoctorTargetResult(filePath, DoctorTargetIO)
		res.Message = fmt.Sprintf("read target file: %v", readErr)
		return res
	}

	md, decErr := mdfront.Decode(data)
	if decErr != nil {
		res := newDoctorTargetResult(filePath, DoctorTargetIO)
		res.Message = fmt.Sprintf("decode frontmatter: %v", decErr)
		return res
	}

	artifact, artErr := models.ArtifactFromFrontmatter(md.Frontmatter, string(md.Body))
	if artErr != nil {
		res := newDoctorTargetResult(filePath, DoctorTargetValidation)
		res.Message = fmt.Sprintf("build artifact: %v", artErr)
		return res
	}

	res := newDoctorTargetResult(filePath, DoctorTargetPass)
	res.ArtifactID = artifact.ID
	res.ArtifactType = artifact.ArtifactType

	if ws.HeaderDef == nil {
		// Fail closed when the workspace header-def schema is absent: a nil
		// HeaderDef is a system/config precondition fault, not a clean pass.
		// Returning kind=pass here would make a *skipped* required-field
		// validation indistinguishable from a real pass (a fail-open defect).
		// Map it to io/exit 3 with a distinct diagnostic rather than
		// validation/exit 1 (which would falsely blame the artifact) — the
		// target may be well-formed; we simply cannot load the schema to check
		// it.
		res.Kind = DoctorTargetIO
		res.OK = false
		res.Message = "header definition not loaded; cannot perform required-field validation"
		return res
	}
	if vErr := ValidateArtifactFields(artifact, ws.HeaderDef); vErr != nil {
		res.Kind = DoctorTargetValidation
		res.OK = false
		res.Message = vErr.Error()
		res.FieldErrors = parseMissingFields(vErr.Error())
	}

	return res
}

// confineToStorageRoot resolves filePath (relative paths are interpreted
// against the workspace root) and reports whether it lives within the
// .backlogit storage root. It returns the cleaned absolute target path.
func confineToStorageRoot(ws *Workspace, filePath string) (absTarget string, inScope bool, err error) {
	absStorage, err := filepath.Abs(WorkspaceStorageRoot(ws.RootPath))
	if err != nil {
		return "", false, fmt.Errorf("resolve storage root: %w", err)
	}
	absStorage = filepath.Clean(absStorage)

	target := filePath
	if !filepath.IsAbs(target) {
		target = filepath.Join(ws.RootPath, filePath)
	}
	absTarget, err = filepath.Abs(target)
	if err != nil {
		return "", false, fmt.Errorf("resolve target path: %w", err)
	}
	absTarget = filepath.Clean(absTarget)

	if absTarget == absStorage {
		// The storage root directory itself is not a validatable artifact file.
		return absTarget, false, nil
	}
	if !strings.HasPrefix(absTarget, absStorage+string(filepath.Separator)) {
		return absTarget, false, nil
	}

	// Defense-in-depth against symlink escapes: lexical prefix matching alone can
	// be bypassed if a component under the storage root is a symlink pointing
	// outside it, because os.ReadFile follows symlinks. Resolve realpaths on both
	// sides and require containment — mirroring repairArchivedFrom's EvalSymlinks
	// + pathContained guard (doctor.go).
	realRoot, rootErr := filepath.EvalSymlinks(absStorage)
	if rootErr != nil {
		realRoot = absStorage
	}
	realTarget, evalErr := filepath.EvalSymlinks(absTarget)
	if evalErr != nil {
		// The leaf may legitimately not exist yet (a missing file is an IO, not a
		// scope, failure downstream). Resolve the parent directory and re-attach
		// the base so an intermediate symlinked directory is still caught; if even
		// the parent is unresolvable, accept the lexical result.
		realParent, perr := filepath.EvalSymlinks(filepath.Dir(absTarget))
		if perr != nil {
			return absTarget, true, nil
		}
		realTarget = filepath.Join(realParent, filepath.Base(absTarget))
	}
	if !pathContained(realRoot, realTarget) {
		return absTarget, false, nil
	}
	return absTarget, true, nil
}

// parseMissingFields extracts the field names from a ValidateArtifactFields
// error of the form "missing required fields: a, b, c".
func parseMissingFields(msg string) []string {
	const marker = "missing required fields: "
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return nil
	}
	list := msg[idx+len(marker):]
	parts := strings.Split(list, ",")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		if f := strings.TrimSpace(p); f != "" {
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}
