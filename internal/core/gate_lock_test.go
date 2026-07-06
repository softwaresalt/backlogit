package core

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestLockTaskFileWithHeartbeat_Contention(t *testing.T) {
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task-082.002-T.md")
	if err := os.WriteFile(taskFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	unlock, err := lockTaskFileWithHeartbeat(context.Background(), taskFile, 200*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer func() { _ = unlock() }()

	// A second acquire cannot get the lock within the bounded wait -> retryable.
	_, err2 := lockTaskFileWithHeartbeat(context.Background(), taskFile, 150*time.Millisecond, 0)
	if !stderrors.Is(err2, bkerrors.ErrGateInProgress) {
		t.Fatalf("second acquire err = %v, want ErrGateInProgress", err2)
	}
}

func TestLockTaskFileWithHeartbeat_Release(t *testing.T) {
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task-release.md")
	if err := os.WriteFile(taskFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	unlock, err := lockTaskFileWithHeartbeat(context.Background(), taskFile, 200*time.Millisecond, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unlock(); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	// unlock is idempotent.
	if err := unlock(); err != nil {
		t.Fatalf("second unlock: %v", err)
	}
	// After release a fresh acquire succeeds.
	unlock2, err := lockTaskFileWithHeartbeat(context.Background(), taskFile, 200*time.Millisecond, 0)
	if err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	_ = unlock2()
}

func TestLockTaskFileWithHeartbeat_RefreshesSidecar(t *testing.T) {
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task-heartbeat.md")
	if err := os.WriteFile(taskFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.Abs(taskFile)
	sidecar := taskLockSidecarPath(filepath.Clean(resolved))

	unlock, err := lockTaskFileWithHeartbeat(context.Background(), taskFile, 200*time.Millisecond, 15*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()

	info0, err := os.Stat(sidecar)
	if err != nil {
		t.Fatalf("stat sidecar: %v", err)
	}
	// Give the heartbeat several ticks.
	time.Sleep(120 * time.Millisecond)
	info1, err := os.Stat(sidecar)
	if err != nil {
		t.Fatalf("stat sidecar after: %v", err)
	}
	if !info1.ModTime().After(info0.ModTime()) {
		t.Fatalf("sidecar ModTime did not advance: before=%v after=%v", info0.ModTime(), info1.ModTime())
	}
}
