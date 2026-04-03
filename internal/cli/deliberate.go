package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/core/templates"
)

func newDeliberateCommand(cwd *string) *cobra.Command {
	var title string
	var problemFrame string
	var options string
	var chosenDirection string
	var openQuestions string
	var notes string

	cmd := &cobra.Command{
		Use:   "deliberate <stash-id>",
		Short: "Create a deliberation artifact linked to a stash entry",
		Long: `Create a first-class deliberation artifact in .backlogit\queue and link it
back to an active stash entry so future planning and implementation can recover
the full collaborative context.`,
		Example: `  backlogit deliberate ABCD1234 --title "Audit dashboard split follow-up"
  backlogit deliberate ABCD1234 --options "- Keep the current feature set narrow\n- Pull the work into the next feature wave"
  backlogit deliberate ABCD1234 --chosen-direction "Split the backlog work and defer reporting polish"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			svc, err := templates.NewService(ctx, filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "templates"))
			if err != nil {
				return fmt.Errorf("load template service: %w", err)
			}

			result, err := templates.CreateDeliberationFromStash(ctx, ws, svc, templates.DeliberationInput{
				StashID:         args[0],
				Title:           title,
				ProblemFrame:    problemFrame,
				Options:         options,
				ChosenDirection: chosenDirection,
				OpenQuestions:   openQuestions,
				Notes:           notes,
			})
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "override the deliberation title (defaults to stash text)")
	cmd.Flags().StringVar(&problemFrame, "problem-frame", "", "problem frame content")
	cmd.Flags().StringVar(&options, "options", "", "option set or alternatives considered")
	cmd.Flags().StringVar(&chosenDirection, "chosen-direction", "", "chosen direction and rationale")
	cmd.Flags().StringVar(&openQuestions, "open-questions", "", "outstanding questions or risks")
	cmd.Flags().StringVar(&notes, "notes", "", "supplementary notes or research")
	return cmd
}
