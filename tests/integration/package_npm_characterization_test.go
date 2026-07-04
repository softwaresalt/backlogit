package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestPackageNpmCharacterization runs the shell characterization test for
// scripts/package-npm.sh (task 080.002-T). The assertions live in the shell
// script (scripts/package-npm.characterization.sh); this Go wrapper exists so
// the repo's `go test ./...` gate exercises the characterization on CI.
//
// The characterization requires a POSIX bash plus jq (jq is also a runtime
// dependency of package-npm.sh). It is skipped on Windows, where the resolved
// `bash` may be WSL and cannot consume Windows-style paths; the Linux CI runner
// is the authoritative execution surface. Run the shell script directly with
// Git Bash + jq to verify locally on Windows.
func TestPackageNpmCharacterization(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("characterization requires POSIX bash + jq; run the shell script directly on Windows")
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available on PATH")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not available on PATH")
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repo root: runtime caller unavailable")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	script := filepath.Join(repoRoot, "scripts", "package-npm.characterization.sh")

	if _, err := os.Stat(script); err != nil {
		t.Fatalf("characterization script missing: %v", err)
	}

	cmd := exec.Command(bash, script)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("package-npm.sh characterization failed: %v\n%s", err, out)
	}
	t.Logf("package-npm.sh characterization passed:\n%s", out)
}
