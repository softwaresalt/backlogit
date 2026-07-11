package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/release"
)

func TestRunSelfUpdateReplacesBinaryAfterSHA256Verification(t *testing.T) {
	t.Parallel()

	target := writeSelfUpdateTarget(t, "backlogit.exe", []byte("old-binary"))
	newBinary := []byte("new-binary")
	client := newFakeSelfUpdateClient("v1.2.0", "windows", "amd64", newBinary)

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "windows",
		GOARCH:         "amd64",
	})

	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, "1.0.0", result.OldVersion)
	assert.Equal(t, "v1.2.0", result.NewVersion)
	assert.Equal(t, newBinary, readFile(t, target))
	assert.True(t, client.downloaded("backlogit-windows-amd64.exe"))
	assert.True(t, client.downloaded("SHA256SUMS"))
}

func TestRunSelfUpdateAlreadyCurrentNoops(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit", oldBinary)
	client := newFakeSelfUpdateClient("v1.0.0", "linux", "amd64", []byte("new-binary"))

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})

	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.False(t, result.UpdateAvailable)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.Empty(t, client.downloads)
}

func TestRunSelfUpdateCheckReportsWithoutApplying(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit", oldBinary)
	client := newFakeSelfUpdateClient("v1.2.0", "linux", "amd64", []byte("new-binary"))

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CheckOnly:      true,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})

	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.True(t, result.UpdateAvailable)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.Empty(t, client.downloads)
}

func TestRunSelfUpdateTargetsSpecificTag(t *testing.T) {
	t.Parallel()

	target := writeSelfUpdateTarget(t, "backlogit", []byte("old-binary"))
	newBinary := []byte("specific-binary")
	client := newFakeSelfUpdateClient("v9.9.9", "linux", "amd64", []byte("latest-binary"))
	client.releasesByTag["v1.2.3"] = fakeRelease("v1.2.3", "linux", "amd64", newBinary)
	sum := sha256.Sum256(newBinary)
	name := selfUpdateAssetName("linux", "amd64")
	client.payloads[name] = newBinary
	client.sums = fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		TargetTag:      "v1.2.3",
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "linux",
		GOARCH:         "amd64",
		Rename:         posixRenameForTest,
	})

	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, "v1.2.3", result.NewVersion)
	assert.Equal(t, newBinary, readFile(t, target))
	assert.False(t, client.latestCalled)
	assert.Equal(t, []string{"v1.2.3"}, client.tagsCalled)
}

func TestRunSelfUpdateFailsClosedOnSHA256Problems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configure  func(*fakeSelfUpdateClient)
		wantErrSub string
	}{
		{
			name: "mismatch",
			configure: func(client *fakeSelfUpdateClient) {
				client.sums = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  backlogit-linux-amd64\n"
			},
			wantErrSub: "SHA256 mismatch",
		},
		{
			name: "missing sums asset",
			configure: func(client *fakeSelfUpdateClient) {
				client.release.Assets = client.release.Assets[:1]
			},
			wantErrSub: "SHA256SUMS",
		},
		{
			name: "malformed sums",
			configure: func(client *fakeSelfUpdateClient) {
				client.sums = "not-a-valid-sums-file\n"
			},
			wantErrSub: "parse SHA256SUMS",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldBinary := []byte("old-binary")
			target := writeSelfUpdateTarget(t, "backlogit", oldBinary)
			client := newFakeSelfUpdateClient("v1.2.0", "linux", "amd64", []byte("new-binary"))
			tt.configure(client)

			result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
				Client:         client,
				CurrentVersion: "1.0.0",
				TargetPath:     target,
				GOOS:           "linux",
				GOARCH:         "amd64",
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrSub)
			assert.False(t, result.Updated)
			assert.Equal(t, oldBinary, readFile(t, target))
		})
	}
}

func TestValidateSelfUpdateAssetNameRejectsTraversal(t *testing.T) {
	t.Parallel()

	tests := []string{
		"../backlogit-linux-amd64",
		"nested/backlogit-linux-amd64",
		"nested\\backlogit-linux-amd64",
		"backlogit-linux-amd64.exe",
		"backlogit-linux-arm64",
	}

	for _, name := range tests {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := validateSelfUpdateAssetName(name, "linux", "amd64")

			require.Error(t, err)
		})
	}

	require.NoError(t, validateSelfUpdateAssetName("backlogit-linux-amd64", "linux", "amd64"))
	require.NoError(t, validateSelfUpdateAssetName("backlogit-windows-amd64.exe", "windows", "amd64"))
}

func TestClampSelfUpdateModePreservesRestrictiveExecutableMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode os.FileMode
		want os.FileMode
	}{
		{name: "owner executable only", mode: 0o700, want: 0o700},
		{name: "clamps group world write", mode: 0o777, want: 0o755},
		{name: "preserves nonexecutable restrictive mode", mode: 0o600, want: 0o600},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, clampSelfUpdateMode(tt.mode))
		})
	}
}

func TestRunSelfUpdateUnwritableTargetFailsBeforeDownload(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit", oldBinary)
	client := newFakeSelfUpdateClient("v1.2.0", "linux", "amd64", []byte("new-binary"))

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "linux",
		GOARCH:         "amd64",
		CreateProbe: func(string, string) (string, io.Closer, error) {
			return "", nil, os.ErrPermission
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "manual install")
	assert.False(t, result.Updated)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.Empty(t, client.downloads)
}

func TestRunSelfUpdateLockPermissionFailureGivesManualInstall(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit", oldBinary)
	client := newFakeSelfUpdateClient("v1.2.0", "linux", "amd64", []byte("new-binary"))

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "linux",
		GOARCH:         "amd64",
		OpenLock: func(string) (*os.File, bool, error) {
			return nil, false, os.ErrPermission
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "manual install required")
	assert.False(t, result.Updated)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.Empty(t, client.downloads)
}

func TestRunSelfUpdateWriteProbeUsesUniqueTemporaryFile(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	newBinary := []byte("new-binary")
	target := writeSelfUpdateTarget(t, "backlogit", oldBinary)
	staleFixedProbe := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".write-test")
	require.NoError(t, os.WriteFile(staleFixedProbe, []byte("orphan"), 0o600))
	client := newFakeSelfUpdateClient("v1.2.0", "linux", "amd64", newBinary)

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "linux",
		GOARCH:         "amd64",
		Rename:         posixRenameForTest,
	})

	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, newBinary, readFile(t, target))
	assert.FileExists(t, staleFixedProbe)
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".write-test.*"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestSelfUpdateAvailableExplicitTargetUsesBuildMetadataIdentity(t *testing.T) {
	t.Parallel()

	available, err := selfUpdateAvailable("1.2.3+build.1", "v1.2.3+build.2", true)
	require.NoError(t, err)
	assert.True(t, available)

	available, err = selfUpdateAvailable("v1.2.3+build.1", "1.2.3+build.1", true)
	require.NoError(t, err)
	assert.False(t, available)
}

func TestRunSelfUpdateAlreadyCurrentCleansWindowsOldBinary(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit.exe", oldBinary)
	backupPath := target + ".old"
	require.NoError(t, os.WriteFile(backupPath, []byte("previous"), 0o600))
	client := newFakeSelfUpdateClient("v1.0.0", "windows", "amd64", []byte("new-binary"))

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "windows",
		GOARCH:         "amd64",
	})

	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.NoFileExists(t, backupPath)
	assert.FileExists(t, target+".update.lock")
	assert.Empty(t, client.downloads)
}

func TestRunSelfUpdateRestoresOriginalWhenWindowsMoveNewFails(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit.exe", oldBinary)
	client := newFakeSelfUpdateClient("v1.2.0", "windows", "amd64", []byte("new-binary"))
	var renamedOriginal bool

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "windows",
		GOARCH:         "amd64",
		Rename: func(oldPath, newPath string) error {
			if oldPath == target {
				renamedOriginal = true
				return os.Rename(oldPath, newPath)
			}
			if strings.HasSuffix(oldPath, ".old") && newPath == target {
				return os.Rename(oldPath, newPath)
			}
			if renamedOriginal && newPath == target {
				return errors.New("injected move-new failure")
			}
			return os.Rename(oldPath, newPath)
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback")
	assert.False(t, result.Updated)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.Empty(t, globTempUpdates(t, filepath.Dir(target)))
	assert.NoFileExists(t, target+".old")
}

func TestRunSelfUpdateKeepsOriginalWhenWindowsFirstRenameFails(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit.exe", oldBinary)
	client := newFakeSelfUpdateClient("v1.2.0", "windows", "amd64", []byte("new-binary"))

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "windows",
		GOARCH:         "amd64",
		Rename: func(oldPath, newPath string) error {
			if oldPath == target && newPath == target+".old" {
				return errors.New("injected first rename failure")
			}
			return os.Rename(oldPath, newPath)
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename current binary to backup")
	assert.False(t, result.Updated)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.Empty(t, globTempUpdates(t, filepath.Dir(target)))
	assert.NoFileExists(t, target+".old")
}

func TestRunSelfUpdateIgnoresStaleUnlockedLockFile(t *testing.T) {
	t.Parallel()

	target := writeSelfUpdateTarget(t, "backlogit.exe", []byte("old-binary"))
	lockPath := target + ".update.lock"
	require.NoError(t, os.WriteFile(lockPath, []byte("pid=12345\n"), 0o600))
	newBinary := []byte("new-binary")
	client := newFakeSelfUpdateClient("v1.2.0", "windows", "amd64", newBinary)

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "windows",
		GOARCH:         "amd64",
	})

	require.NoError(t, err)
	assert.True(t, result.Updated)
	assert.Equal(t, newBinary, readFile(t, target))
	assert.FileExists(t, lockPath)
}

func TestRunSelfUpdateRefusesLiveLock(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit.exe", oldBinary)
	lockPath := target + ".update.lock"
	lockFile, acquired, err := openSelfUpdateLockFile(lockPath)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, lockFile.Close())
	})
	client := newFakeSelfUpdateClient("v1.2.0", "windows", "amd64", []byte("new-binary"))

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "windows",
		GOARCH:         "amd64",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "another update is in progress")
	assert.False(t, result.Updated)
	assert.Equal(t, oldBinary, readFile(t, target))
	assert.FileExists(t, lockPath)
	assert.Empty(t, client.downloads)
}

func TestReplaceSelfUpdateBinaryPlatformPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		goos       string
		failFirst  bool
		failSecond bool
		wantErr    string
		wantOps    []string
		wantTarget []byte
		wantOld    bool
	}{
		{
			name:       "unix single atomic rename",
			goos:       "linux",
			wantOps:    []string{"new->backlogit"},
			wantTarget: []byte("new"),
		},
		{
			name:       "windows two step",
			goos:       "windows",
			wantOps:    []string{"backlogit->backlogit.old", "new->backlogit"},
			wantTarget: []byte("new"),
		},
		{
			name:       "windows first rename fails",
			goos:       "windows",
			failFirst:  true,
			wantErr:    "rename current binary to backup",
			wantOps:    []string{"backlogit->backlogit.old"},
			wantTarget: []byte("old"),
		},
		{
			name:       "windows second rename rolls back",
			goos:       "windows",
			failSecond: true,
			wantErr:    "rollback restored original",
			wantOps:    []string{"backlogit->backlogit.old", "new->backlogit", "backlogit.old->backlogit"},
			wantTarget: []byte("old"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			target := filepath.Join(dir, "backlogit")
			tempPath := filepath.Join(dir, "new")
			require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))
			require.NoError(t, os.WriteFile(tempPath, []byte("new"), 0o755))
			var ops []string

			result, err := replaceSelfUpdateBinary(selfUpdateOptions{
				TargetPath: target,
				GOOS:       tt.goos,
				Remove:     os.Remove,
				Rename: func(oldPath, newPath string) error {
					op := filepath.Base(oldPath) + "->" + filepath.Base(newPath)
					ops = append(ops, op)
					switch {
					case tt.failFirst && op == "backlogit->backlogit.old":
						return errors.New("injected first rename failure")
					case tt.failSecond && op == "new->backlogit":
						return errors.New("injected second rename failure")
					case tt.goos == "linux":
						return posixRenameForTest(oldPath, newPath)
					default:
						return os.Rename(oldPath, newPath)
					}
				},
			}, tempPath)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Empty(t, result.cleanupWarning)
			}
			assert.Equal(t, tt.wantOps, ops)
			assert.Equal(t, tt.wantTarget, readFile(t, target))
			if tt.wantOld {
				assert.FileExists(t, target+".old")
			} else {
				assert.NoFileExists(t, target+".old")
			}
		})
	}
}

func TestRunSelfUpdateNetworkFailuresLeaveOriginalIntact(t *testing.T) {
	t.Parallel()

	oldBinary := []byte("old-binary")
	target := writeSelfUpdateTarget(t, "backlogit", oldBinary)
	client := newFakeSelfUpdateClient("v1.2.0", "linux", "amd64", []byte("new-binary"))
	client.latestErr = context.DeadlineExceeded

	result, err := runSelfUpdate(context.Background(), selfUpdateOptions{
		Client:         client,
		CurrentVersion: "1.0.0",
		TargetPath:     target,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.False(t, result.Updated)
	assert.Equal(t, oldBinary, readFile(t, target))
}

func TestUpdateCommandSelfUpdateFlags(t *testing.T) {
	orig := selfUpdateRun
	t.Cleanup(func() { selfUpdateRun = orig })

	var got selfUpdateOptions
	selfUpdateRun = func(_ context.Context, opts selfUpdateOptions) (selfUpdateResult, error) {
		got = opts
		return selfUpdateResult{
			OldVersion:      "1.0.0",
			NewVersion:      "v1.2.0",
			UpdateAvailable: true,
			CheckOnly:       opts.CheckOnly,
		}, nil
	}
	cwd := "."
	cmd := newUpdateCommand(&cwd)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--check", "--to", "v1.2.0"})

	err := cmd.Execute()

	require.NoError(t, err)
	assert.True(t, got.CheckOnly)
	assert.Equal(t, "v1.2.0", got.TargetTag)
	assert.Contains(t, out.String(), "update available")
	assert.Contains(t, out.String(), "1.0.0 -> v1.2.0")
}

func TestUpdateCommandArtifactFlagWithoutIDDoesNotSelfUpdate(t *testing.T) {
	orig := selfUpdateRun
	t.Cleanup(func() { selfUpdateRun = orig })

	called := false
	selfUpdateRun = func(context.Context, selfUpdateOptions) (selfUpdateResult, error) {
		called = true
		return selfUpdateResult{}, nil
	}
	cwd := "."
	cmd := newUpdateCommand(&cwd)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--status", "done"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "artifact ID is required")
	assert.False(t, called)
}

type fakeSelfUpdateClient struct {
	release       release.Release
	releasesByTag map[string]release.Release
	downloads     []string
	payloads      map[string][]byte
	sums          string
	latestErr     error
	tagErr        error
	downloadErr   error
	latestCalled  bool
	tagsCalled    []string
}

func newFakeSelfUpdateClient(tag, goos, goarch string, payload []byte) *fakeSelfUpdateClient {
	rel := fakeRelease(tag, goos, goarch, payload)
	sum := sha256.Sum256(payload)
	name := selfUpdateAssetName(goos, goarch)
	return &fakeSelfUpdateClient{
		release:       rel,
		releasesByTag: map[string]release.Release{tag: rel},
		payloads: map[string][]byte{
			name:         payload,
			"SHA256SUMS": []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)),
		},
		sums: fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name),
	}
}

func fakeRelease(tag, goos, goarch string, payload []byte) release.Release {
	name := selfUpdateAssetName(goos, goarch)
	sum := sha256.Sum256(payload)
	return release.Release{
		TagName: tag,
		Assets: []release.Asset{
			{Name: name, BrowserDownloadURL: "mem://" + name},
			{Name: "SHA256SUMS", BrowserDownloadURL: "mem://SHA256SUMS-" + hex.EncodeToString(sum[:])},
		},
	}
}

func (c *fakeSelfUpdateClient) LatestRelease(context.Context) (release.Release, error) {
	c.latestCalled = true
	if c.latestErr != nil {
		return release.Release{}, c.latestErr
	}
	return c.release, nil
}

func (c *fakeSelfUpdateClient) ReleaseByTag(_ context.Context, tag string) (release.Release, error) {
	c.tagsCalled = append(c.tagsCalled, tag)
	if c.tagErr != nil {
		return release.Release{}, c.tagErr
	}
	rel, ok := c.releasesByTag[tag]
	if !ok {
		return release.Release{}, fmt.Errorf("missing fake release %s", tag)
	}
	return rel, nil
}

func (c *fakeSelfUpdateClient) DownloadAsset(_ context.Context, asset release.Asset, w io.Writer, maxBytes int64) (int64, error) {
	if c.downloadErr != nil {
		return 0, c.downloadErr
	}
	if asset.Name == "SHA256SUMS" {
		payload := []byte(c.sums)
		if maxBytes > 0 && int64(len(payload)) > maxBytes {
			return 0, fmt.Errorf("fake asset %s exceeds limit", asset.Name)
		}
		n, err := w.Write(payload)
		if err != nil {
			return 0, fmt.Errorf("write fake asset: %w", err)
		}
		c.downloads = append(c.downloads, asset.Name)
		return int64(n), nil
	}
	payload, ok := c.payloads[asset.Name]
	if !ok {
		return 0, fmt.Errorf("missing fake asset %s", asset.Name)
	}
	if maxBytes > 0 && int64(len(payload)) > maxBytes {
		return 0, fmt.Errorf("fake asset %s exceeds limit", asset.Name)
	}
	n, err := w.Write(payload)
	if err != nil {
		return 0, fmt.Errorf("write fake asset: %w", err)
	}
	c.downloads = append(c.downloads, asset.Name)
	return int64(n), nil
}

func (c *fakeSelfUpdateClient) downloaded(name string) bool {
	for _, got := range c.downloads {
		if got == name {
			return true
		}
	}
	return false
}

func writeSelfUpdateTarget(t *testing.T, name string, content []byte) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o755))
	return path
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

func globTempUpdates(t *testing.T, dir string) []string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, ".backlogit.*.new"))
	require.NoError(t, err)
	return matches
}

func posixRenameForTest(oldPath, newPath string) error {
	if err := os.Remove(newPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(oldPath, newPath)
}
