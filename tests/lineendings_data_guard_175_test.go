package tests_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const u040ScriptRel = "scripts/check-line-endings.ps1"

const u040Attributes = "* text=auto\n" +
	"internal/faultline/testdata/parity_v1.golden.json text eol=lf\n" +
	"*.go eol=lf\n" +
	"*.json eol=lf\n"

func u040RepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

func u040Git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func u040WriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// u040NewRepo builds a disposable repo whose attribute contract is valid and
// whose committed *.go blob is LF, with the shipped guard script copied in.
func u040NewRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	u040Git(t, repo, "init", "-q")
	u040Git(t, repo, "config", "user.email", "guard@example.com")
	u040Git(t, repo, "config", "user.name", "guard")
	u040Git(t, repo, "config", "core.autocrlf", "false")

	script, err := os.ReadFile(filepath.Join(u040RepoRoot(t), filepath.FromSlash(u040ScriptRel)))
	if err != nil {
		t.Fatalf("shipped guard script missing: %v", err)
	}
	u040WriteFile(t, filepath.Join(repo, filepath.FromSlash(u040ScriptRel)), script)
	u040WriteFile(t, filepath.Join(repo, ".gitattributes"), []byte(u040Attributes))
	u040WriteFile(t, filepath.Join(repo, "a.go"), []byte("package a\n"))
	u040WriteFile(t, filepath.Join(repo, "data.json"), []byte("{\"k\": 1}\n"))
	u040Git(t, repo, "add", ".")
	u040Git(t, repo, "commit", "-q", "-m", "init")
	return repo
}

func u040RunGuard(t *testing.T, repo string) (string, error) {
	t.Helper()
	cmd := exec.Command("pwsh", "-NoProfile", "-File", filepath.FromSlash(u040ScriptRel), "-RepoRoot", repo)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestU175_040_DataGuard(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Fatalf("pwsh is required to execute the guard script: %v", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is required: %v", err)
	}

	t.Run("CRLFWorktreeFails", func(t *testing.T) {
		repo := u040NewRepo(t)
		u040WriteFile(t, filepath.Join(repo, "a.go"), []byte("package a\r\n\r\nvar x = 1\r\n"))
		out, err := u040RunGuard(t, repo)
		if err == nil {
			t.Fatalf("guard passed on CRLF worktree file; output:\n%s", out)
		}
	})

	t.Run("CRLFCommittedBlobFails", func(t *testing.T) {
		repo := u040NewRepo(t)
		blob := filepath.Join(repo, "blob_crlf.go")
		u040WriteFile(t, blob, []byte("package a\r\n"))
		cmd := exec.Command("git", "hash-object", "-w", "--no-filters", "blob_crlf.go")
		cmd.Dir = repo
		oid, err := cmd.Output()
		if err != nil {
			t.Fatalf("hash-object: %v", err)
		}
		u040Git(t, repo, "update-index", "--add", "--cacheinfo", "100644,"+string(oid[:len(oid)-1])+",blob_crlf.go")
		u040WriteFile(t, blob, []byte("package a\n"))
		out, runErr := u040RunGuard(t, repo)
		if runErr == nil {
			t.Fatalf("guard passed on CRLF committed blob; output:\n%s", out)
		}
	})

	t.Run("FullyLFPasses", func(t *testing.T) {
		repo := u040NewRepo(t)
		out, err := u040RunGuard(t, repo)
		if err != nil {
			t.Fatalf("guard failed on fully LF repo: %v\n%s", err, out)
		}
	})
}
