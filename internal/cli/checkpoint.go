package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	cmd.AddCommand(newCheckpointListCmd(cwd))
	cmd.AddCommand(newCheckpointGetCmd(cwd))
	cmd.AddCommand(newCheckpointResolveCmd(cwd))
	cmd.AddCommand(newCheckpointCleanupCmd(cwd))
	return cmd
}

func checkpointDir(cwd string) string {
	return cwd + "/.backlogit/checkpoints"
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
				if s.ValidationErr != "" {
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
				wsPath := *cwd + "/.backlogit"
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
