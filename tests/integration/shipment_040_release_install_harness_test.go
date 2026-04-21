package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRootForShipment040Harness(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repo root: runtime caller unavailable")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readShipment040RepoFile(t *testing.T, relativePath string) string {
	t.Helper()

	absPath := filepath.Join(repoRootForShipment040Harness(t), filepath.FromSlash(relativePath))
	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}

	return string(data)
}

func TestTask039009_ReleaseWorkflowRequiresCompleteLdflags(t *testing.T) {
	workflow := readShipment040RepoFile(t, ".github/workflows/release.yml")

	var missing []string
	for _, token := range []string{
		"version.Commit=",
		"version.BuildDate=",
		"GITHUB_SHA",
		"date -u",
	} {
		if !strings.Contains(workflow, token) {
			missing = append(missing, token)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("not implemented: release workflow still missing %s", strings.Join(missing, ", "))
	}
}

func TestTask039010_ReleaseWorkflowPublishesSHA256Sums(t *testing.T) {
	workflow := readShipment040RepoFile(t, ".github/workflows/release.yml")

	var missing []string
	for _, token := range []string{
		"sha256sum",
		"SHA256SUMS",
	} {
		if !strings.Contains(workflow, token) {
			missing = append(missing, token)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("not implemented: release workflow still missing checksum support: %s", strings.Join(missing, ", "))
	}
}

func TestTask039011_UnixInstallScriptExistsForCurlPipe(t *testing.T) {
	scriptPath := filepath.Join(repoRootForShipment040Harness(t), "scripts", "install", "install.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("not implemented: unix one-liner install script missing at %s: %v", scriptPath, err)
	}

	content := string(data)
	var missing []string
	for _, token := range []string{
		"releases/latest",
		"SHA256SUMS",
	} {
		if !strings.Contains(content, token) {
			missing = append(missing, token)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("not implemented: unix install script still missing %s", strings.Join(missing, ", "))
	}
}

func TestTask039011_PowerShellInstallScriptExistsForIRM(t *testing.T) {
	scriptPath := filepath.Join(repoRootForShipment040Harness(t), "scripts", "install", "install.ps1")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("not implemented: PowerShell one-liner install script missing at %s: %v", scriptPath, err)
	}

	content := string(data)
	var missing []string
	for _, token := range []string{
		"releases/latest",
		"SHA256SUMS",
	} {
		if !strings.Contains(content, token) {
			missing = append(missing, token)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("not implemented: PowerShell install script still missing %s", strings.Join(missing, ", "))
	}
}

func TestTask039012_InstallationDocsLeadWithBinaryInstallMethods(t *testing.T) {
	docs := readShipment040RepoFile(t, "docs/installation.md")

	idxCurl := strings.Index(docs, "curl -fsSL")
	idxIRM := strings.Index(docs, "irm ")
	idxReleases := strings.Index(docs, "GitHub Releases")
	idxGoInstall := strings.Index(docs, "go install github.com/softwaresalt/backlogit/cmd/backlogit@latest")
	idxPATH := strings.Index(docs, "PATH")

	switch {
	case idxCurl == -1:
		t.Fatal("not implemented: installation docs still missing the unix one-liner install command")
	case idxIRM == -1:
		t.Fatal("not implemented: installation docs still missing the Windows irm one-liner install command")
	case idxReleases == -1:
		t.Fatal("not implemented: installation docs still missing the direct GitHub Releases download path")
	case idxGoInstall == -1:
		t.Fatal("not implemented: installation docs lost the go install fallback")
	case idxPATH == -1:
		t.Fatal("not implemented: installation docs still missing PATH guidance")
	case idxGoInstall < idxCurl || idxGoInstall < idxReleases:
		t.Fatal("not implemented: installation docs still lead with go install instead of binary-first methods")
	}
}
