package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// linkDirForMemberSwapTest points linkPath at targetDir without elevated
// privileges: a junction on Windows (mklink /J) and a symlink elsewhere.
func linkDirForMemberSwapTest(linkPath, targetDir string) error {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetDir).CombinedOutput()
		if err != nil {
			return fmt.Errorf("mklink /J failed: %w (%s)", err, out)
		}
		return nil
	}
	return os.Symlink(targetDir, linkPath)
}

// TestValidateShipmentReconcileManifestMembers_SwapAfterContainmentCheckIsNotRead
// swaps an intermediate directory of a manifest member for a link to an
// outside location after the member path was resolved and containment-checked
// but before its bytes are read. The outside file carries a different (rejected)
// status; validation must refuse the swap rather than read the substituted file.
func TestValidateShipmentReconcileManifestMembers_SwapAfterContainmentCheckIsNotRead(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)

	originalPath, err := FindArtifactPath(ctx, ws, memberID)
	require.NoError(t, err)
	raw, err := os.ReadFile(originalPath)
	require.NoError(t, err)

	swapDir := filepath.Join(filepath.Dir(originalPath), "toctou-swap")
	require.NoError(t, os.MkdirAll(swapDir, 0o755))
	memberPath := filepath.Join(swapDir, filepath.Base(originalPath))
	require.NoError(t, os.Rename(originalPath, memberPath))

	substituted := strings.Replace(string(raw), "status: done", "status: rejected", 1)
	require.NotEqual(t, string(raw), substituted, "fixture must carry a status line to substitute")
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, filepath.Base(memberPath)), []byte(substituted), 0o644))

	t.Cleanup(func() { shipmentReconcileMemberReadSeam = nil })
	seamCalled := false
	shipmentReconcileMemberReadSeam = func(string) {
		seamCalled = true
		require.NoError(t, os.Rename(swapDir, swapDir+".orig"))
		if linkErr := linkDirForMemberSwapTest(swapDir, outside); linkErr != nil {
			t.Skipf("filesystem cannot create a directory link for the swap: %v", linkErr)
		}
	}

	err = validateShipmentReconcileManifestMembers(ctx, ws, []string{memberID})
	assert.True(t, seamCalled, "the member read seam must run between the containment check and the read")
	require.Error(t, err, "a swap to an outside location must be refused")
	assert.NotErrorIs(t, err, blerrors.ErrUnsupportedLegacyDescope, "the substituted outside file must never be read")
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}
