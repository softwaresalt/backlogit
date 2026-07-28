package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkWriteFileAtomic_DurableOff measures the fast (sync-free) write path.
func BenchmarkWriteFileAtomic_DurableOff(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.md")
	data := []byte("benchmark payload for the atomic write fsync micro-benchmark\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteFileAtomicWithOptions(path, data, Options{DurableWrites: false}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWriteFileAtomic_DurableOn measures the fsync (durable) write path so
// the critical-vs-bulk latency policy can be finalized against real numbers.
func BenchmarkWriteFileAtomic_DurableOn(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.md")
	data := []byte("benchmark payload for the atomic write fsync micro-benchmark\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WriteFileAtomicWithOptions(path, data, Options{DurableWrites: true}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFsyncFile isolates the raw fsync cost of a single file so the
// per-write durability overhead is attributable in the record (U7).
func BenchmarkFsyncFile(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "fsync.bin")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write([]byte("payload")); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := f.Sync(); err != nil {
			b.Fatal(err)
		}
	}
}
