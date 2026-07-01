package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/mdfront"
)

// U8 (071.008-T): `backlogit update --size` routes to the body-preserving
// SetArtifactSize seam, is mutually exclusive with other field flags, and maps a
// busy task lock to exit code 4.

func createSizeTask(t *testing.T, root string) string {
	t.Helper()
	featCmd := cli.NewRootCommand()
	featBuf := new(bytes.Buffer)
	featCmd.SetOut(featBuf)
	featCmd.SetErr(featBuf)
	featCmd.SetArgs([]string{"--cwd", root, "add", "--type", "feature", "--title", "Size feature"})
	require.NoError(t, featCmd.Execute())
	featID := extractID(t, featBuf.String())

	taskCmd := cli.NewRootCommand()
	taskBuf := new(bytes.Buffer)
	taskCmd.SetOut(taskBuf)
	taskCmd.SetErr(taskBuf)
	taskCmd.SetArgs([]string{"--cwd", root, "add", "--type", "task", "--title", "Size task", "--parent", featID})
	require.NoError(t, taskCmd.Execute())
	return extractID(t, taskBuf.String())
}

func taskFilePath(root, id string) string {
	return filepath.Join(root, ".backlogit", "queue", id+".md")
}

func TestUpdateSize_PersistsAndPreservesBody(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)
	path := taskFilePath(root, id)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)
	mdBefore, err := mdfront.Decode(rawBefore)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--size", "M"})
	require.NoError(t, cmd.Execute())

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	mdAfter, err := mdfront.Decode(rawAfter)
	require.NoError(t, err)

	cf, ok := mdAfter.Frontmatter["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must be present")
	assert.Equal(t, "M", cf["size"])
	assert.Equal(t, mdBefore.Body, mdAfter.Body, "body must be preserved byte-for-byte")
}

func TestUpdateSize_InvalidEnumErrors(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--size", "XXL"})
	err := cmd.Execute()
	require.Error(t, err, "out-of-enum size must be rejected")
}

func TestUpdateSize_MutualExclusionErrorsBeforeWrite(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)
	path := taskFilePath(root, id)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--size", "M", "--status", "done"})
	err = cmd.Execute()
	require.Error(t, err, "combining --size with --status must error")

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, rawBefore, rawAfter, "no write may occur when the mutual-exclusion guard fires")
}

func TestUpdateSize_BusyReturnsExit4(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)
	path := taskFilePath(root, id)

	// Plant a fresh lock sidecar to simulate a concurrent holder.
	sidecar := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".lock")
	require.NoError(t, os.WriteFile(sidecar, []byte{}, 0o644))
	t.Cleanup(func() { _ = os.Remove(sidecar) })

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--size", "L"})
	err := cmd.Execute()
	require.Error(t, err)
	var ee *cli.ExitError
	require.True(t, errors.As(err, &ee), "expected ExitError, got %T", err)
	assert.Equal(t, 4, ee.Code, "busy task lock must map to exit 4")
}
