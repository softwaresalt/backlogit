package templates

import (
	"context"
	"fmt"
	"strings"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/models"
)

// DeliberationInput captures the inputs required to create a deliberation from a stash entry.
type DeliberationInput struct {
	StashID         string `json:"stash_id"`
	Title           string `json:"title,omitempty"`
	ProblemFrame    string `json:"problem_frame,omitempty"`
	Options         string `json:"options,omitempty"`
	ChosenDirection string `json:"chosen_direction,omitempty"`
	OpenQuestions   string `json:"open_questions,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

// DeliberationResult describes a created deliberation artifact and its linked stash entry.
type DeliberationResult struct {
	Entry    core.StashEntryView `json:"entry"`
	Artifact *models.Artifact    `json:"artifact"`
}

// CreateDeliberationFromStash creates a first-class deliberation artifact and links it to a stash entry.
func CreateDeliberationFromStash(ctx context.Context, ws *core.Workspace, svc *Service, input DeliberationInput) (*DeliberationResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if svc == nil {
		return nil, fmt.Errorf("template service is required")
	}
	entry, err := core.GetStashEntry(ctx, ws, input.StashID)
	if err != nil {
		return nil, err
	}
	if entry.DeliberationID != "" {
		return nil, fmt.Errorf("stash entry %s is already linked to deliberation %s", entry.ID, entry.DeliberationID)
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = entry.Text
	}
	problemFrame := strings.TrimSpace(input.ProblemFrame)
	if problemFrame == "" {
		problemFrame = fmt.Sprintf(
			"Stash entry `%s` captured a `%s` at `%s` priority: %s",
			entry.ID,
			entry.Kind,
			entry.Priority,
			entry.Text,
		)
	}

	sections := map[string]string{
		"problem-frame": problemFrame,
	}
	if value := strings.TrimSpace(input.Options); value != "" {
		sections["options"] = value
	}
	if value := strings.TrimSpace(input.ChosenDirection); value != "" {
		sections["chosen-direction"] = value
	}
	if value := strings.TrimSpace(input.OpenQuestions); value != "" {
		sections["open-questions"] = value
	}
	if value := strings.TrimSpace(input.Notes); value != "" {
		sections["notes"] = value
	}

	fields := map[string]any{
		"linked_stash_id":       entry.ID,
		"linked_stash_priority": entry.Priority,
		"linked_stash_kind":     entry.Kind,
		"linked_stash_text":     entry.Text,
	}
	artifact, err := svc.Create(
		ctx,
		ws,
		title,
		"deliberation",
		sections,
		core.WithPriority(entry.Priority),
		core.WithFields(fields),
	)
	if err != nil {
		return nil, err
	}

	linkedEntry, err := core.LinkDeliberationToStashEntry(ctx, ws, entry.ID, artifact.ID)
	if err != nil {
		return nil, err
	}

	return &DeliberationResult{
		Entry:    *linkedEntry,
		Artifact: artifact,
	}, nil
}
