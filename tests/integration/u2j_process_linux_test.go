package integration_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

type u2jLinuxLease struct {
	mu        sync.Mutex
	pid       int
	cancelled bool
	closed    bool
	cancelErr error
}

func u2jNewProcessLease(cmd *exec.Cmd) (u2jProcessLease, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Setsid || cmd.SysProcAttr.Pgid != 0 {
		return nil, errors.New("u2j owned process group conflicts with command attributes")
	}
	cmd.SysProcAttr.Setpgid = true
	return &u2jLinuxLease{}, nil
}

func (l *u2jLinuxLease) start(cmd *exec.Cmd) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := cmd.Start(); err != nil {
		return err
	}
	l.pid = cmd.Process.Pid
	return nil
}

func (l *u2jLinuxLease) waitParent() error {
	// WNOWAIT leaves the exited group leader unreaped, reserving its PID/PGID
	// throughout drain and group termination. Only Cmd.Wait performs the reap.
	var info unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, l.pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("u2j observe owned parent %d without reaping: %w", l.pid, err)
		}
		return nil
	}
}

func (l *u2jLinuxLease) cancelLocked() error {
	if !l.cancelled && l.pid != 0 {
		l.cancelled = true
		if err := unix.Kill(-l.pid, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
			l.cancelErr = fmt.Errorf("u2j kill owned process group %d: %w", l.pid, err)
		}
	}
	return l.cancelErr
}

func (l *u2jLinuxLease) cancel() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return os.ErrProcessDone
	}
	return l.cancelLocked()
}

func (l *u2jLinuxLease) close() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		l.closed = true
		l.cancelErr = l.cancelLocked()
	}
	return fmt.Sprintf("owned_pgid=%d; group_cancelled=%t; group_kill_error=%v",
		l.pid, l.cancelled, l.cancelErr), l.cancelErr
}
