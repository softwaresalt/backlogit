package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/soundness"
)

// errSoundnessBlocked is returned by `soundness lint` in enforce mode when the
// plan has blocking findings, so the process exits non-zero.
var errSoundnessBlocked = errors.New("soundness: plan blocked by findings")

// maxSoundnessInputBytes bounds the plan and config files the linter reads.
const maxSoundnessInputBytes = 4 << 20

// newSoundnessCommand builds the `backlogit soundness` parent command, a thin
// layer over the soundness library (internal/soundness).
func newSoundnessCommand(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "soundness",
		Short: "Lint plans and work items for decomposition soundness",
	}
	cmd.AddCommand(newSoundnessLintCommand(cwd))
	return cmd
}

func newSoundnessLintCommand(cwd *string) *cobra.Command {
	var planPath, configPath, mode, format string
	var disable []string
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run the soundness linter over a plan file (report-only by default)",
		Long: `Run the soundness engine over a JSON plan of units (plans, features, tasks,
subtasks) as a standalone check. Wiring this check into shipment assembly is a
follow-up.

The default mode is report-only: findings are printed as warnings and the exit
status stays zero. In enforce mode any error finding is printed and the command
exits non-zero; --mode may tighten a report-only config but never relax an
enforce config. An empty plan is an error in enforce mode. A misconfiguration (unknown mode, unknown rule, invalid budget)
is always an error, never a pass. Plan and config paths are confined to the
workspace root.`,
		Example: `  backlogit soundness lint --plan plan.json
  backlogit soundness lint --plan plan.json --mode enforce --config soundness.conf
  backlogit soundness lint --plan plan.json --disable-rule budget --format json`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveDocsRoot(cwd)
			if err != nil {
				return err
			}
			outFormat, err := resolveFormat(cmd.OutOrStdout(), format)
			if err != nil {
				return err
			}
			if planPath == "" {
				return errors.New("--plan is required")
			}
			var plan soundness.Plan
			if err := decodeWorkspaceJSON(root, planPath, &plan); err != nil {
				return fmt.Errorf("load plan: %w", err)
			}
			cfg := soundness.DefaultConfig()
			if configPath != "" {
				text, err := readWorkspaceFile(root, configPath)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if cfg, err = soundness.ParseConfig(string(text)); err != nil {
					return fmt.Errorf("load config: %w", err)
				}
			}
			cfg.Root = root
			if mode != "" {
				if cfg.Mode == soundness.ModeEnforce && soundness.Mode(mode) != soundness.ModeEnforce {
					return fmt.Errorf("--mode %q would relax the configured enforce mode; the flag may only tighten", mode)
				}
				cfg.Mode = soundness.Mode(mode)
			}
			for _, id := range disable {
				cfg.Rules[strings.TrimSpace(id)] = false
			}

			decision, err := soundness.Assess(plan, cfg)
			if err != nil {
				return fmt.Errorf("soundness lint: %w", err)
			}
			if outFormat == "json" {
				if err := writeDocsJSON(cmd.OutOrStdout(), decision); err != nil {
					return err
				}
			} else {
				printSoundnessText(cmd.OutOrStdout(), decision)
			}
			if decision.Blocked {
				return errSoundnessBlocked
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "workspace-relative JSON plan file (required)")
	cmd.Flags().StringVar(&configPath, "config", "", "workspace-relative text config (budgets, mode, rule flags)")
	cmd.Flags().StringVar(&mode, "mode", "", "gate mode: report-only (default), enforce")
	cmd.Flags().StringSliceVar(&disable, "disable-rule", nil, "rule IDs to disable")
	cmd.Flags().StringVar(&format, "format", "", "output format: text, json (default: text on TTY, json otherwise)")
	return cmd
}

func readWorkspaceFile(root, rel string) ([]byte, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open workspace root: %w", err)
	}
	defer func() { _ = r.Close() }()
	f, err := r.Open(filepath.ToSlash(rel))
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", rel, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxSoundnessInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", rel, err)
	}
	if len(data) > maxSoundnessInputBytes {
		return nil, fmt.Errorf("%q exceeds %d bytes", rel, maxSoundnessInputBytes)
	}
	return data, nil
}

func decodeWorkspaceJSON(root, rel string, v any) error {
	data, err := readWorkspaceFile(root, rel)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode %q: %w", rel, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode %q: unexpected data after the first JSON value", rel)
	}
	return nil
}

func printSoundnessText(w io.Writer, d soundness.GateDecision) {
	for _, f := range d.Findings {
		_, _ = fmt.Fprintf(w, "%s [%s] %s: %s\n", f.Severity, f.Rule, f.UnitID, f.Message)
	}
	verdict := "proceed"
	if d.Blocked {
		verdict = "blocked"
	}
	_, _ = fmt.Fprintf(w, "soundness (%s): %d finding(s), %s\n", d.Mode, len(d.Findings), verdict)
}
