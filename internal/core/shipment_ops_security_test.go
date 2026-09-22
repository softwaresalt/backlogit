package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

const testShipmentOperationJournalName = "shipment-operation-0123456789abcdef0123456789abcdef.json"

func TestNewWorkspace_RejectsUnsafeShipmentOperationEntries(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, string)
	}{
		{
			name: "invalid lifecycle journal filename",
			seed: func(t *testing.T, opsRoot string) {
				t.Helper()
				require.NoError(t, os.WriteFile(
					filepath.Join(opsRoot, "shipment-operation-not-a-correlation.json"),
					[]byte(`{}`),
					0o644,
				))
			},
		},
		{
			name: "lifecycle journal is not a regular file",
			seed: func(t *testing.T, opsRoot string) {
				t.Helper()
				require.NoError(t, os.Mkdir(filepath.Join(opsRoot, testShipmentOperationJournalName), 0o755))
			},
		},
		{
			name: "unknown journal class",
			seed: func(t *testing.T, opsRoot string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(opsRoot, "unknown.json"), []byte(`{}`), 0o644))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newShipmentOpsSecurityWorkspace(t)
			opsRoot := filepath.Join(root, ".backlogit", "ops")
			require.NoError(t, os.MkdirAll(opsRoot, 0o755))
			tt.seed(t, opsRoot)

			ws, err := NewWorkspace(context.Background(), root)
			if ws != nil {
				t.Cleanup(func() { require.NoError(t, ws.Close()) })
			}
			require.Error(t, err)
			require.Nil(t, ws)
			require.Contains(t, err.Error(), "shipment")
		})
	}
}

func TestNewWorkspace_RejectsRedirectedShipmentOperationJournalWithoutReadingPreimage(t *testing.T) {
	root := newShipmentOpsSecurityWorkspace(t)
	opsRoot := filepath.Join(root, ".backlogit", "ops")
	require.NoError(t, os.MkdirAll(opsRoot, 0o755))

	outsideRoot := t.TempDir()
	const outsidePreimage = "outside-preimage-must-not-be-exposed"
	outsideJournal := filepath.Join(outsideRoot, "outside.json")
	require.NoError(t, os.WriteFile(outsideJournal, []byte(outsidePreimage), 0o644))

	journalPath := filepath.Join(opsRoot, testShipmentOperationJournalName)
	if err := os.Symlink(outsideJournal, journalPath); err != nil {
		t.Skipf("filesystem cannot create a file symlink: %v", err)
	}

	ws, err := NewWorkspace(context.Background(), root)
	require.Error(t, err)
	require.Nil(t, ws)
	require.NotContains(t, err.Error(), outsidePreimage)
	require.Equal(t, outsidePreimage, string(requireReadFile(t, outsideJournal)))
}

func TestWriteShipmentLifecycleJournal_RejectsRedirectedEntryWithoutExposingPreimage(t *testing.T) {
	root := newShipmentOpsSecurityWorkspace(t)
	ws, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })

	opsRoot := filepath.Join(root, ".backlogit", "ops")
	require.NoError(t, os.MkdirAll(opsRoot, 0o755))
	outsideJournal := filepath.Join(t.TempDir(), "outside.json")
	const outsideContent = "outside-entry-must-remain-unchanged"
	require.NoError(t, os.WriteFile(outsideJournal, []byte(outsideContent), 0o644))
	if err := os.Symlink(outsideJournal, filepath.Join(opsRoot, testShipmentOperationJournalName)); err != nil {
		t.Skipf("filesystem cannot create a file symlink: %v", err)
	}

	journal := shipmentLifecycleJournal{
		SchemaVersion: "shipment-operation/v1",
		CorrelationID: "0123456789abcdef0123456789abcdef",
		Phase:         "intent",
		Reason:        "secret-preimage-marker",
	}
	_, err = writeShipmentLifecycleJournalForWorkspace(ws, testShipmentOperationJournalName, journal)
	require.Error(t, err)
	require.Equal(t, outsideContent, string(requireReadFile(t, outsideJournal)))
}

func newShipmentOpsSecurityWorkspace(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))
	return root
}

func requireReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
