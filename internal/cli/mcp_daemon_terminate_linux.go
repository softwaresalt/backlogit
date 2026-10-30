//go:build linux

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// terminateVerifiedProcess opens a pidfd for pid, verifies the recorded
// identity, confirms the pidfd still refers to pid, and signals through the
// pidfd so a recycled PID is never signaled.
func terminateVerifiedProcess(pid int, startTime int64, exePath string, reverify func() error) error {
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			return nil
		}
		return fmt.Errorf("%w: pidfd_open %d: %v", corerrors.ErrValidation, pid, err)
	}
	defer func() { _ = unix.Close(fd) }()

	procDir := "/proc/" + strconv.Itoa(pid)
	got, err := readProcStartTime(procDir)
	if err != nil {
		return fmt.Errorf("%w: %v", corerrors.ErrValidation, err)
	}
	if got != startTime {
		return fmt.Errorf("%w: process %d start time does not match daemon metadata", corerrors.ErrValidation, pid)
	}
	exe, err := os.Readlink(procDir + "/exe")
	if err != nil {
		return fmt.Errorf("%w: read exe of %d: %v", corerrors.ErrValidation, pid, err)
	}
	if filepath.Clean(strings.TrimSuffix(exe, " (deleted)")) != filepath.Clean(exePath) {
		return fmt.Errorf("%w: process %d executable does not match daemon metadata", corerrors.ErrValidation, pid)
	}
	// The pidfd was opened before verification; a dead or recycled target
	// would show a different Pid in its fdinfo.
	info, err := os.ReadFile("/proc/self/fdinfo/" + strconv.Itoa(fd))
	if err != nil || !strings.Contains("\n"+string(info), "\nPid:\t"+strconv.Itoa(pid)+"\n") {
		return fmt.Errorf("%w: pidfd no longer refers to process %d", corerrors.ErrValidation, pid)
	}
	if err := reverify(); err != nil {
		return err
	}
	if err := unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
		return fmt.Errorf("signal process %d: %w", pid, err)
	}
	return nil
}

// readProcStartTime returns field 22 (starttime) of procDir/stat.
func readProcStartTime(procDir string) (int64, error) {
	data, err := os.ReadFile(procDir + "/stat")
	if err != nil {
		return 0, fmt.Errorf("read %s/stat: %w", procDir, err)
	}
	s := string(data)
	end := strings.LastIndexByte(s, ')')
	if end < 0 {
		return 0, fmt.Errorf("malformed %s/stat", procDir)
	}
	fields := strings.Fields(s[end+1:])
	if len(fields) < 20 {
		return 0, fmt.Errorf("malformed %s/stat", procDir)
	}
	v, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse starttime: %w", err)
	}
	return v, nil
}
