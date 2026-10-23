package cli_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

// failFirstWriter fails its first Write and records every later write.
type failFirstWriter struct {
	calls int
	later bytes.Buffer
}

func (w *failFirstWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		return 0, errors.New("simulated closed stdout")
	}
	return w.later.Write(p)
}

// A failed migration-summary write must not skip rehydration; the output
// error is returned only after cache maintenance has run.
func TestMigrateCommand_SummaryWriteFailureStillRehydrates(t *testing.T) {
	root := t.TempDir()

	initCmd := cli.NewRootCommand()
	initCmd.SetOut(new(bytes.Buffer))
	initCmd.SetErr(new(bytes.Buffer))
	initCmd.SetArgs([]string{"init", "--cwd", root})
	require.NoError(t, initCmd.Execute())

	w := &failFirstWriter{}
	cmd := cli.NewRootCommand()
	cmd.SetOut(w)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"migrate", "--cwd", root})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write output")
	assert.Contains(t, w.later.String(), "Rehydrated", "rehydration must still run after a failed summary write")
}
