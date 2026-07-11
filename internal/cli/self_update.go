package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/release"
	"github.com/softwaresalt/backlogit/internal/version"
)

const (
	selfUpdateTimeout          = 2 * time.Minute
	selfUpdateMaxDownloadBytes = 100 << 20
	selfUpdateMaxSumsBytes     = 1 << 20
	selfUpdateLockStaleAfter   = time.Hour
)

type selfUpdateReleaseClient interface {
	LatestRelease(context.Context) (release.Release, error)
	ReleaseByTag(context.Context, string) (release.Release, error)
	DownloadAsset(context.Context, release.Asset, io.Writer, int64) (int64, error)
}

type selfUpdateOptions struct {
	Client           selfUpdateReleaseClient
	CheckOnly        bool
	TargetTag        string
	CurrentVersion   string
	TargetPath       string
	GOOS             string
	GOARCH           string
	MaxDownloadBytes int64
	MaxSumsBytes     int64
	Rename           func(string, string) error
	Remove           func(string) error
	OpenForWrite     func(string) (io.Closer, error)
	Now              func() time.Time
	ProcessRunning   func(int) bool
	LockStaleAfter   time.Duration
}

type selfUpdateResult struct {
	OldVersion      string
	NewVersion      string
	TargetPath      string
	UpdateAvailable bool
	Updated         bool
	CheckOnly       bool
	CleanupWarning  string
}

var selfUpdateRun = runSelfUpdate

func defaultSelfUpdateOptions() selfUpdateOptions {
	return selfUpdateOptions{
		Client: &release.Client{
			HTTPClient: &http.Client{Timeout: selfUpdateTimeout},
			Token:      os.Getenv("GITHUB_TOKEN"),
		},
	}
}

func runSelfUpdate(ctx context.Context, opts selfUpdateOptions) (selfUpdateResult, error) {
	opts = normalizeSelfUpdateOptions(opts)
	opts.TargetTag = strings.TrimSpace(opts.TargetTag)
	if opts.TargetPath == "" {
		target, err := resolveSelfUpdateTarget()
		if err != nil {
			return selfUpdateResult{}, err
		}
		opts.TargetPath = target
	}
	result := selfUpdateResult{
		OldVersion: opts.CurrentVersion,
		TargetPath: opts.TargetPath,
		CheckOnly:  opts.CheckOnly,
	}

	rel, err := lookupSelfUpdateRelease(ctx, opts)
	if err != nil {
		return result, err
	}
	result.NewVersion = rel.TagName

	available, err := selfUpdateAvailable(opts.CurrentVersion, rel.TagName, opts.TargetTag != "")
	if err != nil {
		return result, err
	}
	result.UpdateAvailable = available
	if !available {
		return result, nil
	}
	if opts.CheckOnly {
		return result, nil
	}

	assetName := selfUpdateAssetName(opts.GOOS, opts.GOARCH)
	asset, ok := release.FindAsset(rel.Assets, assetName)
	if !ok {
		return result, fmt.Errorf("find update asset %q in release %s: asset not found", assetName, rel.TagName)
	}
	if err := validateSelfUpdateAssetName(asset.Name, opts.GOOS, opts.GOARCH); err != nil {
		return result, err
	}
	sumsAsset, ok := release.FindAsset(rel.Assets, "SHA256SUMS")
	if !ok {
		return result, fmt.Errorf("find SHA256SUMS in release %s: asset not found", rel.TagName)
	}
	err = withSelfUpdateLock(opts, func() error {
		if err := ensureSelfUpdateTargetWritable(opts.TargetPath, opts.OpenForWrite, opts.Remove); err != nil {
			return err
		}
		expected, err := fetchExpectedSelfUpdateSHA(ctx, opts.Client, sumsAsset, assetName, opts.MaxSumsBytes)
		if err != nil {
			return err
		}
		tempPath, err := downloadSelfUpdateAsset(ctx, opts, asset, expected)
		if err != nil {
			return err
		}
		replaced, err := replaceSelfUpdateBinary(opts, tempPath)
		if err != nil {
			if removeErr := opts.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return fmt.Errorf("%w; remove temporary update file: %w", err, removeErr)
			}
			return err
		}
		result.Updated = true
		result.CleanupWarning = replaced.cleanupWarning
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func normalizeSelfUpdateOptions(opts selfUpdateOptions) selfUpdateOptions {
	if opts.Client == nil {
		opts.Client = defaultSelfUpdateOptions().Client
	}
	if opts.CurrentVersion == "" {
		opts.CurrentVersion = version.Resolve()
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	if opts.MaxDownloadBytes <= 0 {
		opts.MaxDownloadBytes = selfUpdateMaxDownloadBytes
	}
	if opts.MaxSumsBytes <= 0 {
		opts.MaxSumsBytes = selfUpdateMaxSumsBytes
	}
	if opts.Rename == nil {
		opts.Rename = os.Rename
	}
	if opts.Remove == nil {
		opts.Remove = os.Remove
	}
	if opts.OpenForWrite == nil {
		opts.OpenForWrite = func(path string) (io.Closer, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.ProcessRunning == nil {
		opts.ProcessRunning = func(int) bool {
			return true
		}
	}
	if opts.LockStaleAfter <= 0 {
		opts.LockStaleAfter = selfUpdateLockStaleAfter
	}
	return opts
}

func lookupSelfUpdateRelease(ctx context.Context, opts selfUpdateOptions) (release.Release, error) {
	if strings.TrimSpace(opts.TargetTag) != "" {
		rel, err := opts.Client.ReleaseByTag(ctx, opts.TargetTag)
		if err != nil {
			return release.Release{}, fmt.Errorf("fetch release %s: %w", opts.TargetTag, err)
		}
		return rel, nil
	}
	rel, err := opts.Client.LatestRelease(ctx)
	if err != nil {
		return release.Release{}, fmt.Errorf("fetch latest release: %w", err)
	}
	return rel, nil
}

func selfUpdateAvailable(current, target string, explicitTarget bool) (bool, error) {
	cmp, err := release.CompareVersions(current, target)
	if err == nil {
		if explicitTarget {
			return cmp != 0, nil
		}
		return cmp < 0, nil
	}
	if explicitTarget {
		return true, nil
	}
	return false, fmt.Errorf("compare current version %q with latest %q: %w; use --to <tag> for an explicit target", current, target, err)
}

func resolveSelfUpdateTarget() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve running executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlinks: %w", err)
	}
	return resolved, nil
}

func selfUpdateAssetName(goos, goarch string) string {
	name := "backlogit-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func validateSelfUpdateAssetName(name, goos, goarch string) error {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("validate update asset name %q: path separators are not allowed", name)
	}
	expected := selfUpdateAssetName(goos, goarch)
	if name != expected {
		return fmt.Errorf("validate update asset name %q: expected %q", name, expected)
	}
	return nil
}

func ensureSelfUpdateTargetWritable(targetPath string, openForWrite func(string) (io.Closer, error), remove func(string) error) error {
	if strings.TrimSpace(targetPath) == "" {
		return errors.New("self-update target path is empty; manual install required")
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("check update target %s: %w; manual install required", targetPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("check update target %s: target is a directory; manual install required", targetPath)
	}
	probePath := filepath.Join(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".write-test")
	f, err := openForWrite(probePath)
	if err != nil {
		return fmt.Errorf("check update target %s is writable: %w; manual install required", targetPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close update target writability probe: %w", err)
	}
	if err := remove(probePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove update target writability probe: %w", err)
	}
	return nil
}

func fetchExpectedSelfUpdateSHA(ctx context.Context, client selfUpdateReleaseClient, asset release.Asset, targetName string, maxBytes int64) (string, error) {
	var buf strings.Builder
	if _, err := client.DownloadAsset(ctx, asset, &buf, maxBytes); err != nil {
		return "", fmt.Errorf("download SHA256SUMS: %w", err)
	}
	sums, err := release.ParseSHA256SUMS(buf.String())
	if err != nil {
		return "", fmt.Errorf("parse SHA256SUMS: %w", err)
	}
	expected, ok := sums[targetName]
	if !ok {
		return "", fmt.Errorf("parse SHA256SUMS: missing SHA256 entry for %s", targetName)
	}
	return expected, nil
}

func downloadSelfUpdateAsset(ctx context.Context, opts selfUpdateOptions, asset release.Asset, expectedSHA string) (tempPath string, returnErr error) {
	targetDir := filepath.Dir(opts.TargetPath)
	tmp, err := os.CreateTemp(targetDir, "."+filepath.Base(opts.TargetPath)+".*.new")
	if err != nil {
		return "", fmt.Errorf("create temporary update file in target directory: %w", err)
	}
	tempPath = tmp.Name()
	cleanupPath := tempPath
	closed := false
	defer func() {
		if !closed {
			if err := tmp.Close(); err != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("close temporary update file: %w", err))
			}
		}
		if returnErr != nil {
			if err := opts.Remove(cleanupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary update file: %w", err))
			}
		}
	}()

	hasher := sha256.New()
	if _, err := opts.Client.DownloadAsset(ctx, asset, io.MultiWriter(tmp, hasher), opts.MaxDownloadBytes); err != nil {
		return "", fmt.Errorf("download update asset %s: %w", asset.Name, err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != strings.ToLower(expectedSHA) {
		return "", fmt.Errorf("SHA256 mismatch for %s: expected %s, got %s", asset.Name, expectedSHA, actual)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary update file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary update file: %w", err)
	}
	closed = true
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod temporary update file: %w", err)
	}
	return tempPath, nil
}

type replaceSelfUpdateResult struct {
	cleanupWarning string
}

func replaceSelfUpdateBinary(opts selfUpdateOptions, tempPath string) (replaceSelfUpdateResult, error) {
	if opts.GOOS == "windows" {
		return replaceSelfUpdateBinaryWindows(opts, tempPath)
	}
	return replaceSelfUpdateBinaryUnix(opts, tempPath)
}

func replaceSelfUpdateBinaryUnix(opts selfUpdateOptions, tempPath string) (replaceSelfUpdateResult, error) {
	if err := opts.Rename(tempPath, opts.TargetPath); err != nil {
		return replaceSelfUpdateResult{}, fmt.Errorf("atomically replace binary: %w", err)
	}
	var result replaceSelfUpdateResult
	if err := syncSelfUpdateDir(filepath.Dir(opts.TargetPath)); err != nil {
		result.cleanupWarning = fmt.Sprintf("target directory sync failed after atomic replace: %v", err)
	}
	return result, nil
}

func replaceSelfUpdateBinaryWindows(opts selfUpdateOptions, tempPath string) (replaceSelfUpdateResult, error) {
	backupPath := opts.TargetPath + ".old"
	if err := removeStaleSelfUpdateBackup(opts.Remove, backupPath); err != nil {
		return replaceSelfUpdateResult{}, err
	}
	// Windows cannot rename over a running .exe. The bounded self-healing path
	// renames the current binary to .old, moves the verified replacement into
	// place, and rolls back .old if that second metadata operation fails. A crash
	// between the two renames leaves .old recoverable for manual or next-run
	// cleanup instead of corrupting the verified replacement.
	if err := opts.Rename(opts.TargetPath, backupPath); err != nil {
		return replaceSelfUpdateResult{}, fmt.Errorf("rename current binary to backup: %w", err)
	}
	if err := opts.Rename(tempPath, opts.TargetPath); err != nil {
		rollbackErr := rollbackSelfUpdateReplacement(opts, backupPath)
		if rollbackErr != nil {
			return replaceSelfUpdateResult{}, fmt.Errorf("move new binary into place: %w; rollback failed: %w", err, rollbackErr)
		}
		return replaceSelfUpdateResult{}, fmt.Errorf("move new binary into place: %w; rollback restored original", err)
	}
	var result replaceSelfUpdateResult
	if err := syncSelfUpdateDir(filepath.Dir(opts.TargetPath)); err != nil {
		result.cleanupWarning = fmt.Sprintf("target directory sync failed after replacement: %v", err)
	}
	if err := opts.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		result.cleanupWarning = fmt.Sprintf("old binary cleanup deferred: %v", err)
	}
	return result, nil
}

func withSelfUpdateLock(opts selfUpdateOptions, fn func() error) (returnErr error) {
	lockPath := opts.TargetPath + ".update.lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("acquire self-update lock: %w", err)
		}
		reclaimed, reclaimErr := reclaimSelfUpdateLock(opts, lockPath)
		if reclaimErr != nil {
			return reclaimErr
		}
		if !reclaimed {
			return fmt.Errorf("acquire self-update lock: another update is in progress at %s", lockPath)
		}
		lock, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("acquire reclaimed self-update lock: %w", err)
		}
	}
	defer func() {
		if err := lock.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close self-update lock: %w", err))
		}
		if err := opts.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove self-update lock: %w", err))
		}
	}()
	if _, err := fmt.Fprintf(lock, "pid=%d\n", os.Getpid()); err != nil {
		return fmt.Errorf("write self-update lock: %w", err)
	}
	if err := fn(); err != nil {
		return err
	}
	return nil
}

func reclaimSelfUpdateLock(opts selfUpdateOptions, lockPath string) (bool, error) {
	info, err := os.Stat(lockPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat self-update lock: %w", err)
	}
	lockAge := opts.Now().Sub(info.ModTime())
	raw, readErr := os.ReadFile(lockPath)
	if readErr != nil && lockAge <= opts.LockStaleAfter {
		return false, fmt.Errorf("read self-update lock: %w", readErr)
	}
	pid, hasPID := parseSelfUpdateLockPID(string(raw))
	if (hasPID && !opts.ProcessRunning(pid)) || lockAge > opts.LockStaleAfter {
		if err := opts.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("remove stale self-update lock: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func parseSelfUpdateLockPID(raw string) (int, bool) {
	for _, line := range strings.Split(raw, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || key != "pid" {
			continue
		}
		pid, err := strconv.Atoi(value)
		if err != nil || pid <= 0 {
			return 0, false
		}
		return pid, true
	}
	return 0, false
}

func rollbackSelfUpdateReplacement(opts selfUpdateOptions, backupPath string) error {
	if err := opts.Rename(backupPath, opts.TargetPath); err != nil {
		return fmt.Errorf("restore original binary: %w", err)
	}
	return nil
}

func removeStaleSelfUpdateBackup(remove func(string) error, backupPath string) error {
	if err := remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale update backup: %w", err)
	}
	return nil
}

func syncSelfUpdateDir(dir string) (returnErr error) {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open update target directory: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close update target directory: %w", err))
		}
	}()
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync update target directory: %w", err)
	}
	return nil
}

func writeSelfUpdateResult(w io.Writer, result selfUpdateResult) error {
	var line string
	switch {
	case result.CheckOnly && result.UpdateAvailable:
		line = fmt.Sprintf("update available: %s -> %s\n", result.OldVersion, result.NewVersion)
	case result.CheckOnly:
		line = fmt.Sprintf("backlogit is already current (%s)\n", result.OldVersion)
	case result.Updated:
		line = fmt.Sprintf("updated backlogit: %s -> %s\n", result.OldVersion, result.NewVersion)
	default:
		line = fmt.Sprintf("backlogit is already current (%s)\n", result.OldVersion)
	}
	if _, err := fmt.Fprint(w, line); err != nil {
		return fmt.Errorf("write update result: %w", err)
	}
	if result.CleanupWarning != "" {
		if _, err := fmt.Fprintf(w, "warning: %s\n", result.CleanupWarning); err != nil {
			return fmt.Errorf("write update cleanup warning: %w", err)
		}
	}
	return nil
}
