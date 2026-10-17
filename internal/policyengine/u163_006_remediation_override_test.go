package policyengine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

func TestU163_006_Remediation7AbsentAnchorFailsClosedUnlessGenesisIsExplicit(t *testing.T) {
	e := newEnv(t, enforceRegistry)

	t.Run("absent anchor is an error by default", func(t *testing.T) {
		o := newOverrideEnv(t, e)
		_, err := VerifyOverrideLog(o.ledger, o.anchors)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAnchorMissing)
	})
	t.Run("absent anchor with an existing ledger is still an error", func(t *testing.T) {
		o := newOverrideEnv(t, e)
		require.NoError(t, os.WriteFile(o.lpath, []byte("{}\n"), 0o600))
		_, err := VerifyOverrideLog(o.ledger, o.anchors)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAnchorMissing)
	})
	t.Run("explicit genesis accepts an empty ledger", func(t *testing.T) {
		o := newOverrideEnv(t, e)
		log, err := VerifyOverrideLog(o.ledger, o.anchors, AllowGenesis())
		require.NoError(t, err)
		assert.Zero(t, log.Records)
		assert.NotNil(t, log.Entries)
	})
	t.Run("explicit genesis still rejects a non-empty ledger", func(t *testing.T) {
		o := newOverrideEnv(t, e)
		require.NoError(t, os.WriteFile(o.lpath, []byte("{}\n"), 0o600))
		_, err := VerifyOverrideLog(o.ledger, o.anchors, AllowGenesis())
		require.Error(t, err)
		assert.ErrorIs(t, err, evidencedag.ErrLedgerTampered)
	})
	t.Run("a stored genesis anchor needs no option", func(t *testing.T) {
		o := newOverrideEnv(t, e)
		require.NoError(t, o.anchors.Store(evidencedag.Anchor{}))
		log, err := VerifyOverrideLog(o.ledger, o.anchors)
		require.NoError(t, err)
		assert.Zero(t, log.Records)
	})
}
