package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

var (
	selfInstallExecutablePathFn        = os.Executable
	selfInstallBeforeBinaryReplaceHook func()
	selfInstallBeforeIgnoreReplaceHook func(fileName string)
	selfInstallTempSeq                 atomic.Uint64
)

func resolveInstallStorageDir(root string) (string, error) {
	candidates := core.WorkspaceRootCandidates()
	var present []string
	for _, candidate := range candidates {
		candPath := filepath.Join(root, candidate)
		info, err := os.Lstat(candPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("inspect workspace dir %s: %w", candPath, err)
		}
		isReparse, inspectErr := core.IsSymlinkOrReparsePoint(info, candPath)
		if inspectErr != nil {
			return "", fmt.Errorf("inspect workspace dir %s: %w", candPath, inspectErr)
		}
		if isReparse || !info.IsDir() {
			return "", fmt.Errorf("workspace dir %s is redirected or not a directory: %w", candPath, corerrors.ErrValidation)
		}
		present = append(present, candidate)
	}
	if len(present) > 1 {
		return "", fmt.Errorf("conflicting workspace directories %v exist under %s: %w", present, root, corerrors.ErrValidation)
	}
	if len(present) == 1 {
		return present[0], nil
	}
	if override := strings.TrimSpace(os.Getenv("BACKLOGIT_WORKSPACE_DIR")); slices.Contains(candidates, override) {
		return override, nil
	}
	return candidates[len(candidates)-1], nil
}

func openValidatedWorkspaceAndStorageRoots(root, storageDir string) (string, *os.Root, *os.Root, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return "", nil, nil, fmt.Errorf("inspect workspace root: %w", err)
	}
	rootLink, err := core.IsSymlinkOrReparsePoint(rootInfo, absRoot)
	if err != nil || rootLink || !rootInfo.IsDir() {
		return "", nil, nil, fmt.Errorf("workspace root %s is redirected or not a directory: %w", absRoot, corerrors.ErrValidation)
	}

	wsRoot, err := os.OpenRoot(absRoot)
	if err != nil {
		return "", nil, nil, fmt.Errorf("open workspace root: %w", err)
	}
	closeWSRoot := true
	defer func() {
		if closeWSRoot {
			// Best-effort handle cleanup on early error return.
			_ = wsRoot.Close()
		}
	}()

	openedRootStat, err := wsRoot.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedRootStat) {
		return "", nil, nil, fmt.Errorf("workspace root %s identity changed after open: %w", absRoot, corerrors.ErrValidation)
	}

	storageInfo, err := wsRoot.Lstat(storageDir)
	if err != nil {
		return "", nil, nil, fmt.Errorf("inspect storage directory %s: %w", storageDir, err)
	}
	storageLink, err := core.IsSymlinkOrReparsePoint(storageInfo, filepath.Join(absRoot, storageDir))
	if err != nil || storageLink || !storageInfo.IsDir() {
		return "", nil, nil, fmt.Errorf("storage directory %s is redirected or not a directory: %w", storageDir, corerrors.ErrValidation)
	}

	storageHandle, err := wsRoot.OpenRoot(storageDir)
	if err != nil {
		return "", nil, nil, fmt.Errorf("open storage directory %s: %w", storageDir, err)
	}
	closeStorage := true
	defer func() {
		if closeStorage {
			// Best-effort handle cleanup on early error return.
			_ = storageHandle.Close()
		}
	}()

	openedStorage, err := storageHandle.Stat(".")
	if err != nil || !os.SameFile(storageInfo, openedStorage) {
		return "", nil, nil, fmt.Errorf("storage directory %s identity changed after open: %w", storageDir, corerrors.ErrValidation)
	}
	closeStorage = false
	closeWSRoot = false
	return absRoot, wsRoot, storageHandle, nil
}

func installWorkspaceRuntimeExecutable(root, storageDir string) (string, error) {
	srcPath, err := selfInstallExecutablePathFn()
	if err != nil {
		return "", fmt.Errorf("resolve running executable: %w", err)
	}
	resolvedSrcPath, err := filepath.EvalSymlinks(srcPath)
	if err != nil {
		return "", fmt.Errorf("resolve running executable symlinks %s: %w", srcPath, err)
	}
	srcLstat, err := os.Lstat(resolvedSrcPath)
	if err != nil {
		return "", fmt.Errorf("inspect running executable %s: %w", resolvedSrcPath, err)
	}
	srcLink, err := core.IsSymlinkOrReparsePoint(srcLstat, resolvedSrcPath)
	if err != nil || srcLink || !srcLstat.Mode().IsRegular() {
		return "", fmt.Errorf("running executable %s is redirected or not a regular file: %w", resolvedSrcPath, corerrors.ErrValidation)
	}
	srcFile, err := os.Open(resolvedSrcPath)
	if err != nil {
		return "", fmt.Errorf("open running executable %s: %w", resolvedSrcPath, err)
	}
	srcClosed := false
	defer func() {
		if !srcClosed {
			// Best-effort cleanup on early return.
			_ = srcFile.Close()
		}
	}()
	srcStat, err := srcFile.Stat()
	if err != nil || !os.SameFile(srcStat, srcLstat) {
		return "", fmt.Errorf("running executable %s identity mismatch: %w", resolvedSrcPath, corerrors.ErrValidation)
	}

	binName := "backlogit"
	if runtime.GOOS == "windows" || strings.EqualFold(filepath.Ext(srcPath), ".exe") {
		binName = "backlogit.exe"
	}

	absRoot, wsRoot, storageHandle, err := openValidatedWorkspaceAndStorageRoots(root, storageDir)
	if err != nil {
		return "", err
	}
	storageClosed := false
	wsClosed := false
	defer func() {
		if !storageClosed {
			_ = storageHandle.Close()
		}
		if !wsClosed {
			_ = wsRoot.Close()
		}
	}()

	storageDirAbs := filepath.Join(absRoot, storageDir)
	dstInfo, dstErr := storageHandle.Lstat(binName)
	if dstErr == nil {
		dstLink, linkErr := core.IsSymlinkOrReparsePoint(dstInfo, filepath.Join(storageDirAbs, binName))
		if linkErr != nil || dstLink || !dstInfo.Mode().IsRegular() {
			return "", fmt.Errorf("existing runtime executable %s is redirected or not a regular file: %w", binName, corerrors.ErrValidation)
		}
		if os.SameFile(srcStat, dstInfo) {
			srcClosed = true
			if err := srcFile.Close(); err != nil {
				return "", fmt.Errorf("close running executable: %w", err)
			}
			storageClosed = true
			if err := storageHandle.Close(); err != nil {
				return "", fmt.Errorf("close storage handle: %w", err)
			}
			wsClosed = true
			if err := wsRoot.Close(); err != nil {
				return "", fmt.Errorf("close workspace root: %w", err)
			}
			return binName, nil
		}
	} else if !errors.Is(dstErr, os.ErrNotExist) {
		return "", fmt.Errorf("inspect destination runtime executable %s: %w", binName, dstErr)
	}

	tmpName := fmt.Sprintf("%s.tmp.%d.%d", binName, os.Getpid(), selfInstallTempSeq.Add(1))
	tmpFile, err := storageHandle.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return "", fmt.Errorf("create temp runtime executable: %w", err)
	}
	tmpClosed := false
	tmpRemoved := false
	defer func() {
		if !tmpClosed {
			_ = tmpFile.Close()
		}
		if !tmpRemoved {
			_ = storageHandle.Remove(tmpName)
		}
	}()

	openedTmpStat, err := tmpFile.Stat()
	if err != nil {
		return "", fmt.Errorf("stat temp runtime executable: %w", err)
	}
	lstatTmpStat, err := storageHandle.Lstat(tmpName)
	if err != nil || !os.SameFile(openedTmpStat, lstatTmpStat) {
		return "", fmt.Errorf("temp runtime executable identity changed after open: %w", corerrors.ErrValidation)
	}
	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		return "", fmt.Errorf("copy runtime executable: %w", err)
	}
	srcClosed = true
	if err := srcFile.Close(); err != nil {
		return "", fmt.Errorf("close source runtime executable: %w", err)
	}
	if err := tmpFile.Chmod(0o755); err != nil {
		return "", fmt.Errorf("chmod temp runtime executable: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return "", fmt.Errorf("sync temp runtime executable: %w", err)
	}
	tmpClosed = true
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close temp runtime executable: %w", err)
	}

	if selfInstallBeforeBinaryReplaceHook != nil {
		selfInstallBeforeBinaryReplaceHook()
	}

	preDstInfo, preDstErr := storageHandle.Lstat(binName)
	if preDstErr == nil {
		preLink, linkErr := core.IsSymlinkOrReparsePoint(preDstInfo, filepath.Join(storageDirAbs, binName))
		if linkErr != nil || preLink || !preDstInfo.Mode().IsRegular() {
			return "", fmt.Errorf("destination runtime executable %s became redirected or non-regular: %w", binName, corerrors.ErrValidation)
		}
	} else if !errors.Is(preDstErr, os.ErrNotExist) {
		return "", fmt.Errorf("re-inspect runtime executable %s before replace: %w", binName, preDstErr)
	}

	if err := atomicfile.ReplaceTempFileInRoot(storageHandle, storageDirAbs, tmpName, binName, true); err != nil {
		return "", fmt.Errorf("install runtime executable %s: %w", binName, err)
	}
	tmpRemoved = true

	storageClosed = true
	if err := storageHandle.Close(); err != nil {
		return "", fmt.Errorf("close storage directory %s: %w", storageDir, err)
	}
	wsClosed = true
	if err := wsRoot.Close(); err != nil {
		return "", fmt.Errorf("close workspace root: %w", err)
	}
	return binName, nil
}

func workspaceRuntimeIgnoreEntries() []string {
	var entries []string
	for _, candidate := range core.WorkspaceRootCandidates() {
		entries = append(entries, "/"+candidate+"/backlogit", "/"+candidate+"/backlogit.exe")
	}
	return entries
}

func ensureWorkspaceRuntimeIgnored(root, _ /* storageDir */, _ /* binName */ string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve workspace root for ignore wiring: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return fmt.Errorf("inspect workspace root for ignore wiring: %w", err)
	}
	rootLink, err := core.IsSymlinkOrReparsePoint(rootInfo, absRoot)
	if err != nil || rootLink || !rootInfo.IsDir() {
		return fmt.Errorf("workspace root %s is redirected or not a directory: %w", absRoot, corerrors.ErrValidation)
	}

	wsRoot, err := os.OpenRoot(absRoot)
	if err != nil {
		return fmt.Errorf("open workspace root for ignore wiring: %w", err)
	}
	wsClosed := false
	defer func() {
		if !wsClosed {
			// Best-effort cleanup on early return.
			_ = wsRoot.Close()
		}
	}()
	openedRootStat, err := wsRoot.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedRootStat) {
		return fmt.Errorf("workspace root %s identity changed after open: %w", absRoot, corerrors.ErrValidation)
	}

	entries := workspaceRuntimeIgnoreEntries()
	if err := ensureIgnoreEntriesInRoot(wsRoot, absRoot, ".gitignore", entries, 0o644); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	gitInfo, gitErr := wsRoot.Lstat(".git")
	if gitErr == nil {
		gitDirAbs := filepath.Join(absRoot, ".git")
		gitLink, linkErr := core.IsSymlinkOrReparsePoint(gitInfo, gitDirAbs)
		if linkErr != nil || gitLink {
			return fmt.Errorf(".git is redirected: %w", corerrors.ErrValidation)
		}
		if gitInfo.IsDir() {
			gitRoot, openGitErr := wsRoot.OpenRoot(".git")
			if openGitErr != nil {
				return fmt.Errorf("open .git root: %w", openGitErr)
			}
			gitClosed := false
			defer func() {
				if !gitClosed {
					_ = gitRoot.Close()
				}
			}()

			infoStat, infoErr := gitRoot.Lstat("info")
			if infoErr == nil {
				infoDirAbs := filepath.Join(gitDirAbs, "info")
				infoLink, infoLinkErr := core.IsSymlinkOrReparsePoint(infoStat, infoDirAbs)
				if infoLinkErr != nil || infoLink || !infoStat.IsDir() {
					return fmt.Errorf(".git/info is redirected or not a directory: %w", corerrors.ErrValidation)
				}
				gitInfoRoot, openInfoErr := gitRoot.OpenRoot("info")
				if openInfoErr != nil {
					return fmt.Errorf("open .git/info root: %w", openInfoErr)
				}
				infoClosed := false
				defer func() {
					if !infoClosed {
						_ = gitInfoRoot.Close()
					}
				}()
				if err := ensureIgnoreEntriesInRoot(gitInfoRoot, infoDirAbs, "exclude", entries, 0o644); err != nil {
					return fmt.Errorf("update .git/info/exclude: %w", err)
				}
				infoClosed = true
				if err := gitInfoRoot.Close(); err != nil {
					return fmt.Errorf("close .git/info root: %w", err)
				}
			} else if !errors.Is(infoErr, os.ErrNotExist) {
				return fmt.Errorf("inspect .git/info: %w", infoErr)
			}

			gitClosed = true
			if err := gitRoot.Close(); err != nil {
				return fmt.Errorf("close .git root: %w", err)
			}
		}
	} else if !errors.Is(gitErr, os.ErrNotExist) {
		return fmt.Errorf("inspect .git: %w", gitErr)
	}

	wsClosed = true
	if err := wsRoot.Close(); err != nil {
		return fmt.Errorf("close workspace root: %w", err)
	}
	return nil
}

func ensureIgnoreEntriesInRoot(targetRoot *os.Root, targetDirAbs, fileName string, required []string, defaultMode os.FileMode) error {
	mode := defaultMode
	var existingBytes []byte
	targetAbs := filepath.Join(targetDirAbs, fileName)

	info, statErr := targetRoot.Lstat(fileName)
	if statErr == nil {
		link, linkErr := core.IsSymlinkOrReparsePoint(info, targetAbs)
		if linkErr != nil || link || !info.Mode().IsRegular() {
			return fmt.Errorf("%s is redirected or not a regular file: %w", fileName, corerrors.ErrValidation)
		}
		mode = info.Mode().Perm() &^ 0o022
		if mode == 0 {
			mode = defaultMode
		}
		rf, openErr := targetRoot.OpenFile(fileName, os.O_RDONLY, 0)
		if openErr != nil {
			return fmt.Errorf("open %s: %w", fileName, openErr)
		}
		openedStat, rfStatErr := rf.Stat()
		if rfStatErr != nil || !os.SameFile(openedStat, info) {
			_ = rf.Close()
			return fmt.Errorf("%s identity changed on open: %w", fileName, corerrors.ErrValidation)
		}
		data, readErr := io.ReadAll(rf)
		closeErr := rf.Close()
		if readErr != nil {
			return fmt.Errorf("read %s: %w", fileName, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", fileName, closeErr)
		}
		existingBytes = data
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", fileName, statErr)
	}

	updated, changed := appendMissingIgnoreLines(string(existingBytes), required)
	if statErr == nil && !changed {
		return nil
	}

	tmpName := fmt.Sprintf("%s.tmp.%d.%d", fileName, os.Getpid(), selfInstallTempSeq.Add(1))
	tf, err := targetRoot.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create temp %s: %w", fileName, err)
	}
	tfClosed := false
	tmpRemoved := false
	defer func() {
		if !tfClosed {
			_ = tf.Close()
		}
		if !tmpRemoved {
			_ = targetRoot.Remove(tmpName)
		}
	}()

	openedTmpStat, err := tf.Stat()
	if err != nil {
		return fmt.Errorf("stat temp %s: %w", fileName, err)
	}
	lstatTmpStat, err := targetRoot.Lstat(tmpName)
	if err != nil || !os.SameFile(openedTmpStat, lstatTmpStat) {
		return fmt.Errorf("temp %s identity changed after open: %w", fileName, corerrors.ErrValidation)
	}
	n, err := io.WriteString(tf, updated)
	if err != nil {
		return fmt.Errorf("write temp %s: %w", fileName, err)
	}
	if n != len(updated) {
		return fmt.Errorf("short write to temp %s: wrote %d of %d bytes: %w", fileName, n, len(updated), io.ErrShortWrite)
	}
	if err := tf.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp %s: %w", fileName, err)
	}
	tfClosed = true
	if err := tf.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", fileName, err)
	}

	if selfInstallBeforeIgnoreReplaceHook != nil {
		selfInstallBeforeIgnoreReplaceHook(fileName)
	}

	preInfo, preErr := targetRoot.Lstat(fileName)
	if preErr == nil {
		preLink, linkErr := core.IsSymlinkOrReparsePoint(preInfo, targetAbs)
		if linkErr != nil || preLink || !preInfo.Mode().IsRegular() {
			return fmt.Errorf("%s became redirected or non-regular before replace: %w", fileName, corerrors.ErrValidation)
		}
	} else if !errors.Is(preErr, os.ErrNotExist) {
		return fmt.Errorf("re-inspect %s before replace: %w", fileName, preErr)
	}

	if err := atomicfile.ReplaceTempFileInRoot(targetRoot, targetDirAbs, tmpName, fileName, false); err != nil {
		return fmt.Errorf("replace %s: %w", fileName, err)
	}
	tmpRemoved = true
	return nil
}

func appendMissingIgnoreLines(existing string, required []string) (string, bool) {
	normalized := strings.ReplaceAll(existing, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	present := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		present[strings.TrimSpace(line)] = struct{}{}
	}
	var missing []string
	for _, entry := range required {
		if _, ok := present[entry]; !ok {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return existing, false
	}
	var b strings.Builder
	b.WriteString(existing)
	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		b.WriteByte('\n')
	}
	for _, entry := range missing {
		b.WriteString(entry)
		b.WriteByte('\n')
	}
	return b.String(), true
}
