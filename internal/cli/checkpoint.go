package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/jsonutil"
)

// NewCheckpointCmd returns the checkpoint command group.
func NewCheckpointCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Manage session state checkpoints",
		Long: `Manage agent session state checkpoints for disaster recovery.

Checkpoints are written by agent sessions to enable recovery from unexpected
termination. Use these commands to list, inspect, resolve, and clean up
checkpoint files.`,
	}
	cmd.AddCommand(newCheckpointCreateCmd(cwd))
	cmd.AddCommand(newCheckpointListCmd(cwd))
	cmd.AddCommand(newCheckpointGetCmd(cwd))
	cmd.AddCommand(newCheckpointResolveCmd(cwd))
	cmd.AddCommand(newCheckpointCleanupCmd(cwd))
	cmd.AddCommand(newCheckpointAbandonCmd(cwd))
	cmd.AddCommand(newCheckpointQuarantineCmd(cwd))
	return cmd
}

func checkpointDir(cwd string) (string, error) {
	storageRoot, err := core.ResolveStorageRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve workspace storage root: %w", err)
	}
	return filepath.Join(storageRoot, "checkpoints"), nil
}

// newCheckpointCreateCmd returns the `backlogit checkpoint create` subcommand.
// It mirrors the MCP backlogit_create_checkpoint tool: a V1 state dump is
// validated and written through the shared events.CreateCheckpoint pipeline, and
// the written path is returned as JSON ({"path": ...}).
func newCheckpointCreateCmd(cwd *string) *cobra.Command {
	var stateDump string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a session state checkpoint (open context, closed schema)",
		Long: `Create a session state checkpoint from a JSON state dump.

The state dump is written to the workspace checkpoints directory. When the dump
declares schema_version=1, it is validated as a V1 checkpoint and missing
created_at, updated_at, and status fields are auto-populated. A dump without
schema_version=1 (legacy) is written verbatim with no schema validation.

For a schema_version=1 dump, the top level and the nested progress object are
a CLOSED schema namespace: any key outside the modeled set (schema_version,
agent, session_id, phase, status, created_at, updated_at, context, progress,
and resume_hint at the top level; tasks_completed, tasks_remaining,
files_modified, and decisions inside progress) is an unknown field and the
create is rejected, naming every offending key path. The four disposition
fields (disposition, disposition_reason, disposition_operator,
disposition_at) are part of the schema but are RESERVED and administrative:
they are set only by "checkpoint abandon", never at create, and supplying
one here is rejected as an unknown field too. status:"abandoned" is ALSO
rejected even with no disposition fields present, because "checkpoint
abandon" is the only governed path to that state; status:"active" and
status:"resolved" remain accepted.

The context object is the OPEN counterpart: shipment_id, feature_id,
task_ids, and branch are modeled, but any other key you supply there
survives the create round-trip unchanged. The written path and context_keys
(the exact list of context key names persisted to disk) are returned as
JSON.`,
		Example: `  backlogit checkpoint create --state-dump '{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","pr_number":372}}'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info("checkpoint command invoked", "operation", "checkpoint-create")

			if stateDump == "" {
				return fmt.Errorf("state-dump is required")
			}

			dir, err := checkpointDir(*cwd)
			if err != nil {
				return fmt.Errorf("resolve checkpoint dir: %w", err)
			}
			result, err := events.CreateCheckpoint(ctx, dir, stateDump)
			if err != nil {
				return fmt.Errorf("create checkpoint: %w", err)
			}

			enc := jsonutil.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"path": result.Path, "context_keys": result.ContextKeys})
		},
	}
	cmd.Flags().StringVar(&stateDump, "state-dump", "", "JSON checkpoint state dump")
	_ = cmd.MarkFlagRequired("state-dump")
	return cmd
}

func newCheckpointListCmd(cwd *string) *cobra.Command {
	var agent, status, shipmentID, featureID string
	var maxAgeHours float64

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List session state checkpoints",
		Example: `  backlogit checkpoint list --agent ship --status active`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info("checkpoint command invoked", "operation", "checkpoint-list")

			dir, err := checkpointDir(*cwd)
			if err != nil {
				return fmt.Errorf("resolve checkpoint dir: %w", err)
			}
			filter := events.CheckpointFilter{
				Agent:      agent,
				Status:     status,
				ShipmentID: shipmentID,
				FeatureID:  featureID,
			}
			if maxAgeHours > 0 {
				filter.MaxAge = time.Duration(maxAgeHours * float64(time.Hour))
			}

			summaries, err := events.ListCheckpoints(ctx, dir, filter)
			if err != nil {
				return fmt.Errorf("list checkpoints: %w", err)
			}

			// "quarantined" is fixed at 0: 136-F/U9 made listing strictly
			// read-only, so a mere list call never physically quarantines
			// anything (the field this key used to reflect,
			// CheckpointSummary.Quarantined, is deprecated and permanently
			// false on this path). needsQuarantine is the accurate,
			// actionable signal for malformed checkpoints awaiting an
			// explicit checkpoint quarantine call.
			needsQuarantine := 0
			for _, s := range summaries {
				if s.NeedsQuarantine {
					needsQuarantine++
				}
			}

			result := map[string]any{
				"checkpoints":      summaries,
				"total":            len(summaries),
				"quarantined":      0,
				"needs_quarantine": needsQuarantine,
			}

			enc := jsonutil.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "", "filter by agent (ship, stage)")
	cmd.Flags().StringVar(&status, "status", "", "filter by status (active, resolved)")
	cmd.Flags().StringVar(&shipmentID, "shipment-id", "", "filter by shipment ID")
	cmd.Flags().StringVar(&featureID, "feature-id", "", "filter by feature ID")
	cmd.Flags().Float64Var(&maxAgeHours, "max-age-hours", 0, "maximum age in hours")
	return cmd
}

func newCheckpointGetCmd(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:     "get <filename>",
		Short:   "Get and validate a specific checkpoint",
		Example: `  backlogit checkpoint get checkpoint-20260423-100000.json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			filename := args[0]
			slog.Info("checkpoint command invoked", "operation", "checkpoint-get", "filename", filename)

			dir, err := checkpointDir(*cwd)
			if err != nil {
				return fmt.Errorf("resolve checkpoint dir: %w", err)
			}
			// 147-F / U8c: project the conformance verdict from
			// events.GetCheckpointResult rather than the shipped
			// events.GetCheckpoint. Schema-invalid documents keep their
			// existing refusal — GetCheckpointResult returns
			// ErrCheckpointInvalid unwrapped, so this command still exits
			// non-zero on it exactly as before.
			result, err := events.GetCheckpointResult(ctx, dir, filename)
			if err != nil {
				return fmt.Errorf("get checkpoint: %w", err)
			}

			payload := map[string]any{
				"checkpoint":            result.Checkpoint,
				"filename":              filename,
				"valid":                 result.Valid,
				"conforming":            result.Conforming,
				"needs_quarantine":      result.NeedsQuarantine,
				"remediation_intent":    result.RemediationIntent,
				"non_conforming_fields": result.NonConformingFields,
			}

			enc := jsonutil.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		},
	}
}

// checkpointDispositionRefusalMessage builds an actionable operator message
// for a resolve/abandon refusal (147-F / U8). It states the required
// disposition verb read from a RemediationIntent — quarantine is the only
// verb either refusal ever names, since a malformed-or-invalid target
// (ErrCheckpointUseQuarantine) and a valid-but-non-conforming target
// (*CheckpointNonConformingError) both route to the same remedy — rather
// than leaving the verb as incidental sentinel-message prose. For a
// non-conforming refusal it also names the offending top-level keys in
// quoted, bounded form via FieldPathsForDisplay() (147.031-T / U1c). It
// prints no paste-runnable remediation command; that bound, approval-gated
// block is owned by 147.039-T / U16. err is wrapped, not replaced, so
// errors.Is/errors.As still traverse to the original sentinel or typed error.
func checkpointDispositionRefusalMessage(op, filename string, err error) error {
	var nonConforming *blerrors.CheckpointNonConformingError
	if errors.As(err, &nonConforming) {
		intent := events.RemediationIntent{
			Verb:             "quarantine",
			TargetFilename:   filename,
			RequiresApproval: true,
			ApprovalClass:    "A4c",
			Reason:           "non_conforming",
		}
		return fmt.Errorf("%s: checkpoint %s carries unmodeled key(s) %s; required verb: %s: %w",
			op, filename, nonConforming.FieldPathsForDisplay(), intent.Verb, err)
	}
	if errors.Is(err, blerrors.ErrCheckpointUseQuarantine) {
		intent := events.RemediationIntent{
			Verb:             "quarantine",
			TargetFilename:   filename,
			RequiresApproval: true,
			ApprovalClass:    "A4c",
			Reason:           "unparseable_or_invalid",
		}
		return fmt.Errorf("%s: checkpoint %s is malformed; required verb: %s: %w", op, filename, intent.Verb, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// RenderCheckpointRemediationBlock is the CLI-boundary remediation command
// renderer (147-F / 147.039-T / U16). It is the ONLY surface allowed to
// render an operator-runnable disposition command, because the CLI is the
// only layer that knows the resolved workspace. It consumes a
// RemediationIntent and writes a bound, approval-gated block to w wherever a
// refusal or a needs_quarantine summary is printed. A nil intent (the
// conforming path) renders nothing.
//
// TODO(147.039-T / U16): implement the real block. Stub renders nothing so
// callers compile; both TestU16_ scenarios are red until this lands.
func RenderCheckpointRemediationBlock(w io.Writer, intent *events.RemediationIntent, workspaceRoot string) {
	_ = w
	_ = intent
	_ = workspaceRoot
}

func newCheckpointResolveCmd(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:     "resolve <filename>",
		Short:   "Mark a checkpoint as resolved",
		Example: `  backlogit checkpoint resolve checkpoint-20260423-100000.json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			filename := args[0]
			slog.Info("checkpoint command invoked", "operation", "checkpoint-resolve", "filename", filename)

			dir, err := checkpointDir(*cwd)
			if err != nil {
				return fmt.Errorf("resolve checkpoint dir: %w", err)
			}
			if err := events.ResolveCheckpoint(ctx, dir, filename); err != nil {
				return checkpointDispositionRefusalMessage("resolve checkpoint", filename, err)
			}

			result := map[string]any{
				"ok":          true,
				"filename":    filename,
				"status":      "resolved",
				"resolved_at": time.Now().UTC(),
			}

			enc := jsonutil.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
}

func newCheckpointCleanupCmd(cwd *string) *cobra.Command {
	var retentionDays int

	cmd := &cobra.Command{
		Use:     "cleanup",
		Short:   "Archive resolved and stale checkpoints",
		Example: `  backlogit checkpoint cleanup --retention-days 7`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info("checkpoint command invoked", "operation", "checkpoint-cleanup")

			dir, err := checkpointDir(*cwd)
			if err != nil {
				return fmt.Errorf("resolve checkpoint dir: %w", err)
			}

			// Load retention from config if not overridden.
			if retentionDays == 0 {
				wsPath := filepath.Dir(dir)
				cfg, err := config.Load(ctx, wsPath)
				if err == nil && cfg.CheckpointRetention.RetentionDays > 0 {
					retentionDays = cfg.CheckpointRetention.RetentionDays
				} else {
					retentionDays = 7
				}
			}

			result, err := events.CleanupCheckpoints(ctx, dir, retentionDays)
			if err != nil {
				return fmt.Errorf("cleanup checkpoints: %w", err)
			}

			enc := jsonutil.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "override retention days (defaults to config)")
	return cmd
}

// resolveCheckpointOperator resolves the operator identity for a checkpoint
// disposition action (abandon or quarantine) in priority order: the
// --operator flag, then the BACKLOGIT_OPERATOR environment variable, then the
// current OS user. It NEVER defaults to a fixed identity such as "backlogit"
// — if none of these sources resolve a non-empty value, it returns
// ErrCheckpointOperatorRequired so the caller can supply one explicitly.
func resolveCheckpointOperator(flagValue string) (string, error) {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("BACKLOGIT_OPERATOR")); v != "" {
		return v, nil
	}
	if u, err := user.Current(); err == nil {
		if v := strings.TrimSpace(u.Username); v != "" {
			return v, nil
		}
	}
	return "", blerrors.ErrCheckpointOperatorRequired
}

// newCheckpointAbandonCmd returns the `backlogit checkpoint abandon` subcommand.
// It mirrors the MCP backlogit_abandon_checkpoint tool: AbandonCheckpoint
// operates only on a parseable, schema-valid checkpoint target — a malformed
// target must be quarantined instead (see `checkpoint quarantine`).
func newCheckpointAbandonCmd(cwd *string) *cobra.Command {
	var reason, operatorFlag string

	cmd := &cobra.Command{
		Use:   "abandon <filename>",
		Short: "Administratively abandon a valid checkpoint",
		Long: `Administratively abandon a session state checkpoint.

Abandon operates ONLY on a parseable, schema-valid checkpoint. If the target
file is malformed (unparseable or schema-invalid), this command refuses and
directs you to "checkpoint quarantine" instead — abandon and quarantine are
disjoint verbs by design.`,
		Example: `  backlogit checkpoint abandon checkpoint-20260423-100000.json --reason "superseded by newer session"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			filename := args[0]
			slog.Info("checkpoint command invoked", "operation", "checkpoint-abandon", "filename", filename)

			if strings.TrimSpace(reason) == "" {
				return blerrors.ErrCheckpointReasonRequired
			}
			operator, err := resolveCheckpointOperator(operatorFlag)
			if err != nil {
				return err
			}

			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			logsDir := core.WorkspaceLogsRoot(ws.RootPath)
			ew := core.NewWorkspaceEventWriter(ws, logsDir)

			if err := core.AbandonCheckpoint(ctx, ws, ew, filename, reason, operator); err != nil {
				return checkpointDispositionRefusalMessage("abandon checkpoint", filename, err)
			}

			enc := jsonutil.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{
				"filename":    filename,
				"disposition": events.DispositionAbandoned,
				"reason":      reason,
				"operator":    operator,
			})
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "reason for the disposition (required)")
	cmd.Flags().StringVar(&operatorFlag, "operator", "", "operator identity (defaults to BACKLOGIT_OPERATOR env var, then the OS user)")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

// newCheckpointQuarantineCmd returns the `backlogit checkpoint quarantine`
// subcommand. It mirrors the MCP backlogit_quarantine_checkpoint tool:
// QuarantineCheckpoint operates only on a malformed (unparseable or
// schema-invalid) checkpoint target — a valid target must be abandoned
// instead (see `checkpoint abandon`).
func newCheckpointQuarantineCmd(cwd *string) *cobra.Command {
	var reason, operatorFlag string

	cmd := &cobra.Command{
		Use:   "quarantine <filename>",
		Short: "Quarantine a malformed checkpoint",
		Long: `Quarantine a malformed session state checkpoint.

Quarantine operates ONLY on a malformed (unparseable or schema-invalid)
checkpoint. If the target file parses and validates cleanly, this command
refuses and directs you to "checkpoint abandon" instead — abandon and
quarantine are disjoint verbs by design. The checkpoint's bytes are moved
verbatim (byte-identical) into the workspace archive/checkpoints directory.`,
		Example: `  backlogit checkpoint quarantine checkpoint-20260423-100000.json --reason "corrupt JSON"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			filename := args[0]
			slog.Info("checkpoint command invoked", "operation", "checkpoint-quarantine", "filename", filename)

			if strings.TrimSpace(reason) == "" {
				return blerrors.ErrCheckpointReasonRequired
			}
			operator, err := resolveCheckpointOperator(operatorFlag)
			if err != nil {
				return err
			}

			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			logsDir := core.WorkspaceLogsRoot(ws.RootPath)
			ew := core.NewWorkspaceEventWriter(ws, logsDir)

			if err := core.QuarantineCheckpoint(ctx, ws, ew, filename, reason, operator); err != nil {
				return fmt.Errorf("quarantine checkpoint: %w", err)
			}

			enc := jsonutil.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{
				"filename":    filename,
				"disposition": events.DispositionQuarantined,
				"reason":      reason,
				"operator":    operator,
			})
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "reason for the disposition (required)")
	cmd.Flags().StringVar(&operatorFlag, "operator", "", "operator identity (defaults to BACKLOGIT_OPERATOR env var, then the OS user)")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}
