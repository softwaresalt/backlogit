package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
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
	return cmd
}

func checkpointDir(cwd string) string {
	return filepath.Join(cwd, ".backlogit", "checkpoints")
}

// newCheckpointCreateCmd returns the `backlogit checkpoint create` subcommand.
// It mirrors the MCP backlogit_create_checkpoint tool: a V1 state dump is
// validated and written through the shared events.CreateCheckpoint pipeline, and
// the written path is returned as JSON ({"path": ...}).
func newCheckpointCreateCmd(cwd *string) *cobra.Command {
	var stateDump string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a session state checkpoint",
		Long: `Create a session state checkpoint from a JSON state dump.

The state dump is written to the workspace checkpoints directory. When the dump
declares schema_version=1, it is validated as a V1 checkpoint and missing
created_at, updated_at, and status fields are auto-populated. The written path
is returned as JSON.`,
		Example: `  backlogit checkpoint create --state-dump '{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info("checkpoint command invoked", "operation", "checkpoint-create")

			if stateDump == "" {
				return fmt.Errorf("state-dump is required")
			}

			dir := checkpointDir(*cwd)
			path, err := events.CreateCheckpoint(ctx, dir, stateDump)
			if err != nil {
				return fmt.Errorf("create checkpoint: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{"path": path})
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

			dir := checkpointDir(*cwd)
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

			quarantined := 0
			for _, s := range summaries {
				if s.Quarantined {
					quarantined++
				}
			}

			result := map[string]any{
				"checkpoints": summaries,
				"total":       len(summaries),
				"quarantined": quarantined,
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
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

			dir := checkpointDir(*cwd)
			cp, err := events.GetCheckpoint(ctx, dir, filename)
			if err != nil {
				return fmt.Errorf("get checkpoint: %w", err)
			}

			result := map[string]any{
				"checkpoint": cp,
				"filename":   filename,
				"valid":      true,
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
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

			dir := checkpointDir(*cwd)
			if err := events.ResolveCheckpoint(ctx, dir, filename); err != nil {
				return fmt.Errorf("resolve checkpoint: %w", err)
			}

			result := map[string]any{
				"ok":          true,
				"filename":    filename,
				"status":      "resolved",
				"resolved_at": time.Now().UTC(),
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
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

			dir := checkpointDir(*cwd)

			// Load retention from config if not overridden.
			if retentionDays == 0 {
				wsPath := filepath.Join(*cwd, ".backlogit")
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

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "override retention days (defaults to config)")
	return cmd
}
