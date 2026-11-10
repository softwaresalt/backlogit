//go:build !windows

package cli

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestU6_InheritedStdinShutdown(t *testing.T) {
	for _, test := range []struct {
		name   string
		mode   string
		socket bool
	}{
		{name: "peer_eof", mode: "peer_eof"},
		{name: "cancel", mode: "cancel"},
		{name: "stdin_eof", mode: "stdin_eof"},
		{name: "socket_peer_eof", mode: "peer_eof", socket: true},
		{name: "socket_cancel", mode: "cancel", socket: true},
		{name: "socket_stdin_eof", mode: "stdin_eof", socket: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			binary, err := os.Executable()
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), u6Wait)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "-test.run=^TestU6_InheritedStdinHelper$")
			command.Env = append(os.Environ(), "BACKLOGIT_U6_STDIN_HELPER="+test.mode)
			var input io.WriteCloser
			var childInput *os.File
			if test.socket {
				fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
				require.NoError(t, err)
				childInput = os.NewFile(uintptr(fds[0]), "child-stdin")
				defer func() { _ = childInput.Close() }()
				input = os.NewFile(uintptr(fds[1]), "parent-stdin")
				command.Stdin = childInput
				command.Env = append(command.Env, "BACKLOGIT_U6_STDIN_SOCKET=1")
			} else {
				input, err = command.StdinPipe()
				require.NoError(t, err)
			}
			defer func() { _ = input.Close() }()
			output, err := command.StdoutPipe()
			require.NoError(t, err)
			defer func() { _ = output.Close() }()
			stderr := &u6Log{}
			command.Stderr = stderr
			require.NoError(t, command.Start())
			if childInput != nil {
				require.NoError(t, childInput.Close())
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(u6Wait):
					t.Error("owned stdin helper did not exit after cancellation")
				}
			})
			_, err = io.WriteString(input, "x")
			require.NoError(t, err)
			line, err := bufio.NewReader(output).ReadString('\n')
			require.NoError(t, err, "helper must bridge input before shutdown")
			require.Equal(t, "ready\n", line)
			if test.mode == "stdin_eof" {
				require.NoError(t, input.Close())
			}
			select {
			case err := <-done:
				done <- err // Leave the reaped result available to cleanup.
				assert.NoError(t, err, "shim must join while parent keeps stdin open; stderr=%s", stderr.String())
			case <-ctx.Done():
				t.Fatal("shim blocked on inherited stdin after shutdown")
			}
		})
	}
}

func TestU6_InheritedStdinHelper(t *testing.T) {
	mode := os.Getenv("BACKLOGIT_U6_STDIN_HELPER")
	if mode == "" {
		return
	}
	if os.Getenv("BACKLOGIT_U6_STDIN_SOCKET") == "1" {
		info, err := os.Stdin.Stat()
		require.NoError(t, err)
		require.NotZero(t, info.Mode()&os.ModeSocket, "child stdin must be an inherited kernel socket")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, peer := net.Pipe()
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		defer func() { _ = peer.Close() }()
		var first [1]byte
		if _, err := io.ReadFull(peer, first[:]); err != nil {
			return
		}
		if _, err := io.WriteString(peer, "ready\n"); err != nil {
			return
		}
		if mode != "stdin_eof" {
			// Allow the next inherited-stdin read to block with its writer open.
			time.Sleep(100 * time.Millisecond)
		}
		if mode == "cancel" {
			cancel()
		}
		if mode != "peer_eof" {
			// Keep the peer open until cancellation or stdin EOF detaches.
			_, _ = peer.Read(first[:])
		}
	}()
	err := proxyMCPSession(ctx, conn, os.Stdin, os.Stdout)
	<-peerDone
	if mode == "cancel" {
		require.ErrorIs(t, err, context.Canceled)
		return
	}
	require.NoError(t, err)
}
