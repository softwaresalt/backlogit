package evidencedag

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type lockInfo struct {
	PID   int    `json:"pid"`
	Token string `json:"token"`
}

// reapHook and linkFile are test seams invoked by the stale-lock reaper.
var (
	reapHook func()
	linkFile = os.Link
)

// dirLock is an exclusive advisory lock realised as a create-exclusive file
// beneath a canonical directory. It is a cross-platform best-effort lock, not
// an OS-level lock: see the design note's limitations.
type dirLock struct{ dir, name string }

func (d dirLock) path() string { return filepath.Join(d.dir, d.name) }

// heldLock is an acquired dirLock with a heartbeat that keeps its mtime fresh
// so a long operation is not mistaken for a crashed holder.
type heldLock struct {
	d     dirLock
	token string
	stop  chan struct{}
	done  chan struct{}
}

// snapshot reads the identity and content of the named regular file beneath
// the lock directory through one descriptor.
func (d dirLock) snapshot(name string) (os.FileInfo, lockInfo, error) {
	f, err := openBeneath(d.dir, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, lockInfo{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, lockInfo{}, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxSidecarBytes+1))
	if err != nil {
		return nil, lockInfo{}, err
	}
	var li lockInfo
	if len(raw) <= maxSidecarBytes {
		_ = json.Unmarshal(raw, &li) // an unreadable body leaves an empty token
	}
	return fi, li, nil
}

// retryPermission retries fn while it fails with a permission error (Windows
// reports sharing violations and delete-pending files that way) until the
// deadline.
func retryPermission(deadline time.Time, fn func() error) error {
	delay := 2 * time.Millisecond
	for {
		err := fn()
		if err == nil || !errors.Is(err, fs.ErrPermission) || !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay < 50*time.Millisecond {
			delay *= 2
		}
	}
}

// nonRegularLeaf reports whether the lock leaf is persistently something other
// than a regular file (directory, symlink, reparse point, device). It looks
// twice so a file mid-deletion is not mistaken for one.
func (d dirLock) nonRegularLeaf() bool {
	for i := 0; i < 2; i++ {
		fi, err := os.Lstat(d.path())
		if err != nil || fi.Mode().IsRegular() {
			return false
		}
		if i == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}
	return true
}

// reap moves a lock older than lockStale aside by atomic rename so two reapers
// cannot both claim it. Identity is proven twice: the file is snapshotted
// before the rename and the renamed-away file must be the same file with the
// same token. If it is not (the holder released and a new holder acquired in
// between) the lock is restored with a hard link and, if that fails, nothing
// is deleted and the caller fails closed with ErrLedgerBusy.
func (d dirLock) reap() error {
	fi, seen, err := d.snapshot(d.name)
	switch {
	case errors.Is(err, errNotRegular):
		if d.nonRegularLeaf() {
			return tampered("%s is not a regular file", d.name)
		}
		return nil
	case err != nil:
		return nil // gone, replaced or transiently unreadable: keep waiting
	}
	if time.Since(fi.ModTime()) <= lockStale {
		return nil
	}
	if reapHook != nil {
		reapHook()
	}
	asideName := d.name + ".stale-" + randHex(8)
	if err := os.Rename(d.path(), filepath.Join(d.dir, asideName)); err != nil {
		return nil // someone else reaped it, or it is held: keep waiting
	}
	ai, got, aerr := d.snapshot(asideName)
	if aerr == nil && os.SameFile(fi, ai) && got.Token == seen.Token {
		_ = os.Remove(filepath.Join(d.dir, asideName))
		return nil
	}
	if err := linkFile(filepath.Join(d.dir, asideName), d.path()); err != nil {
		return fmt.Errorf("%w: a live lock was moved aside as %s and could not be restored", ErrLedgerBusy, asideName)
	}
	_ = os.Remove(filepath.Join(d.dir, asideName))
	return nil
}

// acquire takes the lock, honouring lockWait with a bounded exponential
// back-off. Permission errors are retried until the deadline.
func (d dirLock) acquire() (*heldLock, error) {
	token := randHex(16)
	deadline := time.Now().Add(lockWait)
	delay := 2 * time.Millisecond
	for {
		f, err := createExclusive(d.dir, d.name, 0o600)
		if err == nil {
			info, _ := json.Marshal(lockInfo{PID: os.Getpid(), Token: token})
			_, werr := f.Write(info)
			cerr := f.Close()
			if werr != nil || cerr != nil {
				_ = os.Remove(d.path())
				return nil, fmt.Errorf("evidencedag: write lock %s: %w", d.name, errors.Join(werr, cerr))
			}
			h := &heldLock{d: d, token: token, stop: make(chan struct{}), done: make(chan struct{})}
			go h.heartbeat(lockStale / 3)
			return h, nil
		}
		if d.nonRegularLeaf() {
			return nil, tampered("%s is not a regular file", d.name)
		}
		if !errors.Is(err, fs.ErrExist) && !errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("evidencedag: acquire lock %s: %w", d.name, err)
		}
		if rerr := d.reap(); rerr != nil {
			return nil, rerr
		}
		if !time.Now().Before(deadline) {
			if errors.Is(err, fs.ErrPermission) {
				if _, lerr := os.Lstat(d.path()); lerr != nil {
					return nil, fmt.Errorf("evidencedag: acquire lock %s: %w", d.name, err)
				}
			}
			return nil, ErrLedgerBusy
		}
		time.Sleep(delay)
		if delay < 50*time.Millisecond {
			delay *= 2
		}
	}
}

func (h *heldLock) heartbeat(every time.Duration) {
	defer close(h.done)
	if every < time.Millisecond {
		every = time.Millisecond
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-t.C:
			if h.verify() == nil {
				now := time.Now()
				_ = os.Chtimes(h.d.path(), now, now) // best effort: a failure only risks reclamation, which verify() detects
			}
		}
	}
}

// verify proves the lock file still carries this holder's token.
func (h *heldLock) verify() error {
	var li lockInfo
	err := retryPermission(time.Now().Add(lockWait), func() error {
		var e error
		_, li, e = h.d.snapshot(h.d.name)
		return e
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLockLost, err)
	}
	if li.Token != h.token {
		return fmt.Errorf("%w: held by another owner (it was reclaimed)", ErrLockLost)
	}
	return nil
}

// release stops the heartbeat, verifies ownership and removes the lock.
func (h *heldLock) release() error {
	close(h.stop)
	<-h.done
	if err := h.verify(); err != nil {
		return err
	}
	err := retryPermission(time.Now().Add(lockWait), func() error { return os.Remove(h.d.path()) })
	if err != nil {
		return fmt.Errorf("remove lock %s: %w", h.d.name, err)
	}
	return nil
}
