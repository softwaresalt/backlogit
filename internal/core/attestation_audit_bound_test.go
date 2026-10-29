package core_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
)

func TestAuditAttestationRejection_BoundsPersistedDiagnostics(t *testing.T) {
	ws := attAuditWorkspace(t)
	rejection := &core.AttestationVerifyError{Field: "signature", Err: errors.New(strings.Repeat("<", 600000))}

	got := core.AuditAttestationRejection(context.Background(), ws, "150-S", "att", rejection)
	require.Same(t, rejection, got, "the original typed error is returned unchanged")

	logged, err := events.ReadAllEvents(context.Background(), core.WorkspaceLogsRoot(ws.RootPath), "150-S")
	require.NoError(t, err, "the shipment log must stay readable")
	require.Len(t, logged, 1)
	require.Equal(t, core.EventAttestationRejected, logged[0].EventType)
	require.Equal(t, "signature", logged[0].Delta["field"])

	data, err := os.ReadFile(events.LogPathForItem(core.WorkspaceLogsRoot(ws.RootPath), "150-S"))
	require.NoError(t, err)
	require.Less(t, len(data), 64*1024)
}
