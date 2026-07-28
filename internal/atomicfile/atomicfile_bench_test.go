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
// per-write durability overhead is attributable in the record (U7). Each
// iteration dirties the file before Sync so the fsync actually flushes new bytes
// — otherwise, after the first iteration's Sync the file is clean and the
// remaining iterations measure a near-noop, understating the per-write cost.
func BenchmarkFsyncFile(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "fsync.bin")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Dirty the file each iteration so Sync has new bytes to flush.
		if _, err := f.WriteAt([]byte{byte(i)}, 0); err != nil {
			b.Fatal(err)
		}
		if err := f.Sync(); err != nil {
			b.Fatal(err)
		}
	}
}
