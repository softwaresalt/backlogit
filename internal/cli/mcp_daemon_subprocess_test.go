package cli

import (
	"bufio"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const u4HelperRootEnv = "BACKLOGIT_U4_HELPER_ROOT"

// TestU4_HelperHoldMCPDaemonLock is a subprocess helper that holds the real
// daemon lock until stdin closes or it is terminated; it is a no-op otherwise.
func TestU4_HelperHoldMCPDaemonLock(t *testing.T) {
	root := os.Getenv(u4HelperRootEnv)
	if root == "" {
		t.Skip("helper process only")
	}
	release, err := AcquireMCPDaemonLock(t.Context(), root, false)
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(2)
	}
	defer func() { _ = release() }()
	os.Stdout.WriteString("ready\n")
	buf := make([]byte, 1)
	_, _ = os.Stdin.Read(buf)
}

func TestU4_CrossProcessMCPDaemonLockAndStop(t *testing.T) {
	root, _, _ := u2Workspace(t)

	cmd := exec.Command(os.Args[0], "-test.run=^TestU4_HelperHoldMCPDaemonLock$")
	cmd.Env = append(os.Environ(), u4HelperRootEnv+"="+root)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		<-done
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ready\n", line)
	pid := cmd.Process.Pid

	status, err := InspectMCPDaemonStatus(root)
	require.NoError(t, err)
	assert.True(t, status.Running)
	assert.Equal(t, pid, status.PID)

	_, err = AcquireMCPDaemonLock(t.Context(), root, false)
	require.ErrorIs(t, err, corerrors.ErrDaemonAlreadyRunning)

	res, err := StopMCPDaemon(t.Context(), root)
	require.NoError(t, err)
	assert.True(t, res.Stopped)
	assert.Equal(t, pid, res.PID)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("child did not exit after stop")
	}

	release, err := AcquireMCPDaemonLock(t.Context(), root, false)
	require.NoError(t, err)
	require.NoError(t, release())
}
