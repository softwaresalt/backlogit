//go:build unix

package redhonesty

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"
)

func TestU161_001_ExtractArchivePreservesExecutableBit(t *testing.T) {
	dir := t.TempDir()
	if err := extractArchive(tarOf(t,
		tar.Header{Name: "scripts/run.sh", Typeflag: tar.TypeReg, Mode: 0o755},
		tar.Header{Name: "scripts/data.txt", Typeflag: tar.TypeReg, Mode: 0o644},
	), dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "scripts", "run.sh"))
	if err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("executable bit lost: %v %v", fi, err)
	}
	fi, _ = os.Stat(filepath.Join(dir, "scripts", "data.txt"))
	if fi.Mode().Perm()&0o111 != 0 {
		t.Fatal("non-executable file became executable")
	}
}
