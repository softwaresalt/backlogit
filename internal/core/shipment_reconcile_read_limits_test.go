package core

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestReadAllShipmentReconcileBounded(t *testing.T) {
	data, err := readAllShipmentReconcileBounded(bytes.NewReader(make([]byte, maxShipmentReconcileReadBytes)), "at limit")
	require.NoError(t, err)
	assert.Len(t, data, maxShipmentReconcileReadBytes)

	_, err = readAllShipmentReconcileBounded(bytes.NewReader(make([]byte, maxShipmentReconcileReadBytes+1)), "over limit")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}

func TestValidateShipmentReconcileFileName(t *testing.T) {
	for name, wantErr := range map[string]bool{
		"201-S.md":    false,
		"":            true,
		".":           true,
		"..":          true,
		"a/b.md":      true,
		`a\b.md`:      true,
		"a.md:stream": true,
		"../x":        true,
	} {
		err := validateShipmentReconcileFileName(name)
		if wantErr {
			require.Error(t, err, name)
			assert.ErrorIs(t, err, blerrors.ErrValidation, name)
		} else {
			require.NoError(t, err, name)
		}
	}
}

// TestReconcileReadSeamsAreNilInProduction proves the test-only read seams are
// never set outside tests.
func TestReconcileReadSeamsAreNilInProduction(t *testing.T) {
	assert.Nil(t, shipmentReconcileMemberReadSeam)
	assert.Nil(t, shipmentSnapshotReadSeam)
}
