package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func forceFlatScopeStatus(t *testing.T, ws *Workspace, id string, status models.ArtifactStatus) {
	t.Helper()
	artifact := cloneArtifact(loadURCanonicalArtifact(t, ws, id))
	artifact.Status = status
	artifact.UpdatedAt = models.NowUTC()
	forceURArtifactFixture(t, ws, artifact)
}

func flatScopeStatus(t *testing.T, ws *Workspace, id string) models.ArtifactStatus {
	t.Helper()
	return loadURCanonicalArtifact(t, ws, id).Status
}

func flatScopeEventCount(t *testing.T, ws *Workspace, id string) int {
	t.Helper()
	all, err := events.ReadAllEvents(context.Background(), WorkspaceLogsRoot(ws.RootPath), id)
	require.NoError(t, err)
	return len(all)
}

func flatScopeStatusReasonsSince(t *testing.T, ws *Workspace, id string, offset int) []string {
	t.Helper()
	all, err := events.ReadAllEvents(context.Background(), WorkspaceLogsRoot(ws.RootPath), id)
	require.NoError(t, err)
	require.LessOrEqual(t, offset, len(all))
	reasons := make([]string, 0)
	for _, event := range all[offset:] {
		if event.EventType != "status_changed" {
			continue
		}
		if reason, ok := event.Delta["reason"].(string); ok {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

func flatScopeLockProbe(t *testing.T, ws *Workspace, id string) bool {
	t.Helper()
	unlock, err := lockArtifactMutation(context.Background(), ws, id)
	if err != nil {
		return true
	}
	require.NoError(t, unlock())
	return false
}
