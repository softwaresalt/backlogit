package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// doctorTargetTimeout is the authoritative in-process deadline for a single
// doctor --target validation. autoharness's subprocess timeout_seconds: 5
// (design §5) is the authoritative OUTER bound; this matches it so the CLI is
// deterministic on its own.
const doctorTargetTimeout = 5 * time.Second

// doctorTargetFunc is the LOCK-FREE validation seam so the timeout/select path
// can be exercised non-vacuously in tests with a slow stub. The lock is owned by
// runDoctorTargetMode's synchronous frame (see PrepareDoctorTarget), never by
// this function, so an abandoned timeout run strands nothing.
type doctorTargetFunc func(ws *core.Workspace, target, absTarget string) *core.DoctorTargetResult

func newDoctorCommand(cwd *string) *cobra.Command {
	var (
		checkOrphans      bool
		checkDuplicates   bool
		checkArchivedFrom bool
		checkGateEvidence bool
		fixOrphans        bool
		fixArchivedFrom   bool
		fixMalformed      bool
		outputFormatFlag  string
		targetFlag        string
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check workspace integrity",
		Long: `Scan the .backlogit workspace for structural issues such as
orphaned artifacts (child types with no parent) and duplicate IDs
across queue and archive directories.

Use --fix-orphans to archive orphaned artifacts automatically.
Use --fix-archived-from to repair legacy self-referential archived_from
records (rewrites them to their canonical queue restore path). Use
--fix-malformed to clear malformed archived_from records that have no restore
target. Both are destructive, CLI-only migrations: not exposed on the MCP doctor tool.

Target mode (--target {file}) validates ONE .backlogit artifact file against
the header-def schema, with a 5s deadline, and returns a versioned, gate-stable
exit code:

  0  pass
  1  validation fail (required field errors)
  2  timeout (validation exceeded the 5s deadline)
  3  scope or IO error (path outside .backlogit storage root, unreadable/undecodable)
  4  busy (the task's advisory lock is held by a concurrent operation)

Target mode confines the path to the .backlogit storage root and never reads
outside it. With --format json it emits a versioned target-mode schema
(mode:"target"). autoharness's subprocess timeout_seconds: 5 is the
authoritative outer bound. On repeated gate failure, transition the task with
the existing 'backlogit move {id} --status blocked' (and '--status queued' to
resume) — no new command; retry policy is owned by the caller.`,
		Example: `  backlogit doctor
  backlogit doctor --check-orphans=false
  backlogit doctor --fix-orphans
  backlogit doctor --fix-archived-from
  backlogit doctor --format json
  backlogit doctor --target .backlogit/queue/001.001-T.md
  backlogit doctor --target .backlogit/queue/001.001-T.md --format json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if outputFormatFlag != "text" && outputFormatFlag != "json" {
				return fmt.Errorf("unsupported format %q (expected text or json)", outputFormatFlag)
			}

			ctx := context.Background()

			// Target mode: single-file gate with a versioned exit-code contract.
			if cmd.Flags().Changed("target") {
				// Gate outcomes (validation fail, busy, timeout) are normal
				// results, not usage errors; silence Cobra's stderr error print
				// and carry the code via ExitError so main can os.Exit(code).
				cmd.SilenceErrors = true
				ws, err := core.NewWorkspace(ctx, *cwd)
				if err != nil {
					return &ExitError{Code: 3, Msg: fmt.Sprintf("open workspace: %v", err)}
				}
				defer ws.Close()

				code, res := runDoctorTargetMode(ctx, ws, targetFlag,
					outputFormatFlag, doctorTargetTimeout, core.ValidateDoctorTargetResolved, cmd.OutOrStdout())
				if code == 0 {
					return nil
				}
				return &ExitError{Code: code, Msg: fmt.Sprintf("doctor target: %s (%s)", res.Kind, res.Message)}
			}

			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			report, err := core.Doctor(ctx, ws, &core.DoctorOptions{
				CheckOrphans:      checkOrphans,
				CheckDuplicates:   checkDuplicates,
				CheckArchivedFrom: checkArchivedFrom,
				CheckGateEvidence: checkGateEvidence,
				FixOrphans:        fixOrphans,
				FixArchivedFrom:   fixArchivedFrom,
				FixMalformed:      fixMalformed,
			})
			if err != nil {
				return fmt.Errorf("doctor: %w", err)
			}

			w := cmd.OutOrStdout()
			if outputFormatFlag == "json" {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			// Text output.
			if len(report.Findings) == 0 && len(report.FixActions) == 0 {
				fmt.Fprintln(w, "No issues found.")
				return nil
			}
			for _, f := range report.Findings {
				fmt.Fprintf(w, "[%s] %s: %s\n", f.Type, f.ArtifactID, f.Description)
			}
			for _, a := range report.FixActions {
				fmt.Fprintf(w, "[fix:%s] %s: %s\n", a.Type, a.ArtifactID, a.Detail)
			}
			fmt.Fprintf(w, "\n%d issue(s) found, %d fix(es) applied.\n", len(report.Findings), len(report.FixActions))
			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOrphans, "check-orphans", true, "check for orphaned child artifacts")
	cmd.Flags().BoolVar(&checkDuplicates, "check-duplicates", true, "check for duplicate IDs across directories")
	cmd.Flags().BoolVar(&checkArchivedFrom, "check-archived-from", true, "check archive records for self-referential/malformed archived_from fields")
	cmd.Flags().BoolVar(&checkGateEvidence, "check-gate-evidence", false, "advisory: warn when a terminal task/subtask lacks pre-task-completion gate evidence (exit code unaffected)")
	cmd.Flags().BoolVar(&fixOrphans, "fix-orphans", false, "archive orphaned artifacts instead of just reporting them")
	cmd.Flags().BoolVar(&fixArchivedFrom, "fix-archived-from", false, "repair legacy self-referential archived_from records (destructive, CLI-only)")
	cmd.Flags().BoolVar(&fixMalformed, "fix-malformed", false, "clear malformed archived_from records with no restore target (destructive, CLI-only; requires --check-archived-from)")
	cmd.Flags().StringVar(&outputFormatFlag, "format", "text", "output format: text or json")
	cmd.Flags().StringVar(&targetFlag, "target", "", "validate a single .backlogit artifact file against header-def (versioned exit-code gate)")

	return cmd
}

// runDoctorTargetMode owns the doctor --target lock lifecycle in a SYNCHRONOUS
// frame whose deferred unlock is guaranteed to run before the command returns
// (and thus before main's os.Exit). It confines + locks the target via
// core.PrepareDoctorTarget; a scope/busy/IO short-circuit is handled directly.
// Only the lock-free read+validate runs under the goroutine-enforced timeout, so
// a timed-out (abandoned) validation never strands the lock sidecar.
func runDoctorTargetMode(
	ctx context.Context,
	ws *core.Workspace,
	target, format string,
	timeout time.Duration,
	validate doctorTargetFunc,
	w io.Writer,
) (int, *core.DoctorTargetResult) {
	absTarget, unlock, short := core.PrepareDoctorTarget(ws, target)
	if short != nil {
		writeDoctorTargetOutput(w, short, format)
		return doctorTargetExitCode(short), short
	}
	// Deferred here — in the command's synchronous call frame — so the sidecar
	// is released even if the validation below times out. This is the whole
	// point of splitting Prepare (locked) from Validate (lock-free).
	defer func() { _ = unlock() }()

	return runDoctorTargetWithTimeout(ctx, ws, target, absTarget, format, timeout, validate, w)
}

// runDoctorTargetWithTimeout runs validate(ws, target, absTarget) under a real
// deadline enforced via a goroutine + select on ctx.Done(). A bare
// context.WithTimeout cannot interrupt synchronous I/O, so the work runs in a
// goroutine and the select races the deadline. The result channel is BUFFERED
// (cap 1) so that on timeout the still-running goroutine can send and exit
// rather than leaking blocked on the channel. validate MUST be lock-free (the
// lock is owned by runDoctorTargetMode) so an abandoned run strands nothing. It
// writes the versioned output and returns the mapped exit code plus the result
// used for the outcome summary.
func runDoctorTargetWithTimeout(
	ctx context.Context,
	ws *core.Workspace,
	target, absTarget, format string,
	timeout time.Duration,
	validate doctorTargetFunc,
	w io.Writer,
) (int, *core.DoctorTargetResult) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resCh := make(chan *core.DoctorTargetResult, 1)
	go func() {
		res := validate(ws, target, absTarget)
		if res == nil {
			// ValidateDoctorTargetResolved classifies every real outcome into
			// res; a nil result is unexpected → bucket as scope/IO (exit 3).
			fallback := core.NewDoctorTargetResult(target, core.DoctorTargetIO)
			fallback.Message = "validation returned no result"
			res = fallback
		}
		resCh <- res
	}()

	select {
	case <-tctx.Done():
		if errors.Is(tctx.Err(), context.DeadlineExceeded) {
			res := core.NewDoctorTargetResult(target, core.DoctorTargetTimeout)
			res.Message = fmt.Sprintf("validation exceeded the %s deadline", timeout)
			writeDoctorTargetOutput(w, res, format)
			return 2, res
		}
		// Parent context cancelled for another reason → treat as IO/scope.
		res := core.NewDoctorTargetResult(target, core.DoctorTargetIO)
		res.Message = tctx.Err().Error()
		writeDoctorTargetOutput(w, res, format)
		return 3, res
	case res := <-resCh:
		writeDoctorTargetOutput(w, res, format)
		return doctorTargetExitCode(res), res
	}
}

// doctorTargetExitCode maps a validation outcome to the versioned exit-code
// contract. Changing this table breaks the cross-repo gate contract.
func doctorTargetExitCode(res *core.DoctorTargetResult) int {
	switch res.Kind {
	case core.DoctorTargetPass:
		return 0
	case core.DoctorTargetValidation:
		return 1
	case core.DoctorTargetTimeout:
		return 2
	case core.DoctorTargetScope, core.DoctorTargetIO:
		return 3
	case core.DoctorTargetBusy:
		return 4
	default:
		return 3
	}
}

// writeDoctorTargetOutput emits the target-mode result as versioned JSON or a
// concise text line.
func writeDoctorTargetOutput(w io.Writer, res *core.DoctorTargetResult, format string) {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return
	}
	if res.OK {
		fmt.Fprintf(w, "PASS %s (%s)\n", res.Path, res.ArtifactID)
		return
	}
	fmt.Fprintf(w, "%s %s: %s\n", string(res.Kind), res.Path, res.Message)
	for _, fe := range res.FieldErrors {
		fmt.Fprintf(w, "  - %s\n", fe)
	}
}
