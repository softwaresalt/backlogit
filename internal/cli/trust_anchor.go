package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// newTrustAnchorCommand returns the `backlogit trust-anchor` command group.
// list and show are strictly read-only (168.004-T): they load config.yaml and
// report the state computed by core.ResolveVerificationKey. They never open
// the database or write any file. Mutating subcommands (add, rotate, revoke)
// live in trust_anchor_mutate.go and delegate to the core mutator.
func newTrustAnchorCommand(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust-anchor",
		Short: "Inspect and manage verification-key trust anchors",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured trust anchors with their resolved state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			anchors, err := loadTrustAnchors(cmd.Context(), *cwd)
			if err != nil {
				return err
			}
			return writeTrustAnchorList(cmd.OutOrStdout(), anchors, time.Now(), loadPins(cmd.Context()))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show one trust anchor with its resolved state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			anchors, err := loadTrustAnchors(cmd.Context(), *cwd)
			if err != nil {
				return err
			}
			return writeTrustAnchorShow(cmd.OutOrStdout(), anchors, args[0], time.Now(), loadPins(cmd.Context()))
		},
	})
	addTrustAnchorMutationCommands(cmd, cwd)
	return cmd
}

// loadTrustAnchors reads the workspace config without touching the database.
func loadTrustAnchors(ctx context.Context, cwd string) ([]config.TrustAnchor, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	storageRoot, err := core.ResolveStorageRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	cfg, err := config.Load(ctx, storageRoot)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg.TrustAnchors, nil
}

// trustAnchorValidity maps the resolver outcome for one anchor to a label.
func trustAnchorValidity(anchors []config.TrustAnchor, id string, at time.Time, pins *core.ExternalPinSet) string {
	_, err := core.ResolveVerificationKey(anchors, core.KeySelector{ID: id}, at, pins)
	switch {
	case err == nil:
		return "valid"
	case errors.Is(err, core.ErrTrustPinSetAbsent), errors.Is(err, core.ErrTrustAnchorNotPinned):
		return "untrusted"
	case errors.Is(err, core.ErrTrustAnchorNotYetValid):
		return "not-yet-valid"
	case errors.Is(err, core.ErrTrustAnchorExpired):
		return "expired"
	default:
		return "unresolvable"
	}
}

func writeTrustAnchorList(w io.Writer, anchors []config.TrustAnchor, at time.Time, pins *core.ExternalPinSet) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tROLE\tALGO\tSTATUS\tVALIDITY\tFINGERPRINT"); err != nil {
		return fmt.Errorf("write trust anchor list: %w", err)
	}
	for i := range anchors {
		a := &anchors[i]
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			a.ID, a.Role, a.Algo, a.Status, trustAnchorValidity(anchors, a.ID, at, pins), a.Fingerprint); err != nil {
			return fmt.Errorf("write trust anchor list: %w", err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("write trust anchor list: %w", err)
	}
	return nil
}

func writeTrustAnchorShow(w io.Writer, anchors []config.TrustAnchor, id string, at time.Time, pins *core.ExternalPinSet) error {
	var a *config.TrustAnchor
	for i := range anchors {
		if anchors[i].ID == id {
			a = &anchors[i]
			break
		}
	}
	if a == nil {
		return fmt.Errorf("show trust anchor %q: %w", id, core.ErrTrustAnchorNotFound)
	}
	lines := []string{
		"id: " + a.ID,
		"role: " + a.Role,
		"algo: " + a.Algo,
		"status: " + string(a.Status),
		"fingerprint: " + a.Fingerprint,
		"public_key_ref: " + refSummary(a.PublicKeyRef),
		"not_before: " + timeOrNone(a.NotBefore),
		"not_after: " + timeOrNone(a.NotAfter),
		"validity: " + trustAnchorValidity(anchors, id, at, pins),
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(w, l); err != nil {
			return fmt.Errorf("write trust anchor: %w", err)
		}
	}
	return nil
}

func timeOrNone(t *time.Time) string {
	if t == nil {
		return "none"
	}
	return t.UTC().Format(time.RFC3339)
}

// refSummary avoids dumping inline PEM bodies.
func refSummary(ref string) string {
	if strings.HasPrefix(strings.TrimSpace(ref), "-----BEGIN") {
		return "(inline PEM)"
	}
	return ref
}
