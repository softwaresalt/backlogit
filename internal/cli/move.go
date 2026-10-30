package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// newMoveCommand creates the `backlogit move` command.
func newMoveCommand(cwd *string) *cobra.Command {
	var (
		status      string
		gateBase    string
		forceGates  bool
		forceReason string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "move <id>",
		Short: "Change artifact status",
		Long: `Change an artifact status and relocate its file according to registry.yaml
routing rules.

For task/subtask completions the pre-task-completion gate broker may run
autoharness gate check. On a gate refusal this command exits 6 (blocked),
7 (configuration/setup error), or 8 (retryable: lock contention or timeout).`,
		Example: `  backlogit move 001.001-T --status review
  backlogit move 001-F --status done
  backlogit move 001.001-T --status done --gate-base origin/release`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if status == "" {
				return fmt.Errorf("--status is required")
			}

			// Force is operator-only and requires a justification (ST3.4).
			opts := core.TransitionOptions{GateBase: gateBase}
			if forceGates {
				if forceReason == "" {
					return fmt.Errorf("--force-gates requires --force-reason")
				}
				opts.Force = true
				opts.ForceReason = forceReason
				opts.ForceSource = core.ForceSourceCLI
			}

			id := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer func() {
				if cerr := ws.Close(); cerr != nil {
					err = errors.Join(err, cerr)
				}
			}()

			artifact, outcome, err := core.UpdateArtifactWithGate(ctx, ws, id, map[string]any{"status": status}, opts)
			if err != nil {
				return moveGateError(cmd, id, err, jsonOut)
			}

			// Relocate to the directory mapped by the ACTUAL resulting status. On
			// the err==nil path a passing gate yields the requested status (there
			// is no redirect without an error).
			resultStatus := string(artifact.Status)
			newPath, err := core.RelocateArtifactFile(ctx, ws, artifact.ArtifactType, id, resultStatus)
			if err != nil {
				return fmt.Errorf("relocate artifact: %w", err)
			}
			if err := core.WriteArtifactFile(artifact, newPath); err != nil {
				return err
			}
			if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
				return err
			}

			if jsonOut && outcome != nil {
				payload, mErr := renderGatePassJSON(id, outcome)
				if mErr != nil {
					return mErr
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), payload); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Moved %s → %s\n", id, resultStatus); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "new status (required)")
	cmd.Flags().StringVar(&gateBase, "gate-base", "", "operator-only base ref override for the completion gate (audited)")
	cmd.Flags().BoolVar(&forceGates, "force-gates", false, "operator-only: force completion past the gate (requires --force-reason)")
	cmd.Flags().StringVar(&forceReason, "force-reason", "", "justification recorded in the forced-gate audit event")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the machine-readable gate outcome contract")
	return cmd
}

// moveGateError maps a gate error to the versioned exit code (6/7/8) and, under
// --json, emits the machine payload. Non-gate errors pass through unchanged.
// Also maps 144-F governance refusals to exit code 9 (ExitShipmentGovernance).
func moveGateError(cmd *cobra.Command, id string, err error, jsonOut bool) error {
	// 144-F: check governance refusals before gate-typed checks so the specific
	// exit code (9) surfaces instead of the generic 1.
	if eeGov := shipmentGovernanceExitError(err); eeGov != nil {
		cmd.SilenceErrors = true
		if _, wErr := fmt.Fprintln(cmd.ErrOrStderr(), err.Error()); wErr != nil {
			return errors.Join(eeGov, fmt.Errorf("write output: %w", wErr))
		}
		return eeGov
	}
	ee := GateExitError(err)
	if ee == nil {
		return err
	}
	cmd.SilenceErrors = true
	var be *corerrors.GateBlockedError
	var ge *corerrors.GateError
	if jsonOut {
		switch {
		case errors.As(err, &be):
			if payload, mErr := renderGateBlockedJSON(id, be); mErr == nil {
				if _, wErr := fmt.Fprintln(cmd.OutOrStdout(), payload); wErr != nil {
					return errors.Join(ee, fmt.Errorf("write output: %w", wErr))
				}
			}
		case errors.As(err, &ge):
			if payload, mErr := RenderGateErrorJSON(id, ge); mErr == nil {
				if _, wErr := fmt.Fprintln(cmd.OutOrStdout(), payload); wErr != nil {
					return errors.Join(ee, fmt.Errorf("write output: %w", wErr))
				}
			}
		}
		return ee
	}
	var msg string
	if errors.As(err, &be) {
		msg = gateHumanMessage(id, be)
	} else {
		msg = err.Error()
	}
	if _, wErr := fmt.Fprintln(cmd.ErrOrStderr(), msg); wErr != nil {
		return errors.Join(ee, fmt.Errorf("write output: %w", wErr))
	}
	return ee
}
