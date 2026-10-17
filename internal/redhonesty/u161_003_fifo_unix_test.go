//go:build unix

package redhonesty

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestU161_003_StoreLoadDoesNotHangOnFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "161.003-T"), 0o755); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "161.003-T", u3Head+".json")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	st, _ := NewStore(dir)
	done := make(chan error, 2)
	go func() { _, err := st.Load("161.003-T", u3Head); done <- err }()
	a, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	go func() { _, err := st.Write(a); done <- err }()
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("special file accepted as evidence")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Load/Write hung on a FIFO")
		}
	}
}
