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
	initBeforeWriteDefaultsHook        func()
	selfInstallTempSeq                 atomic.Uint64
)

func resolveInstallStorageDir(root string) (string, error) {
	candidates := core.WorkspaceRootCandidates()
	if override := strings.TrimSpace(os.Getenv("BACKLOGIT_WORKSPACE_DIR")); override != "" {
		if !slices.Contains(candidates, override) {
			return "", fmt.Errorf("invalid BACKLOGIT_WORKSPACE_DIR %q (expected one of %v): %w", override, candidates, corerrors.ErrValidation)
		}
		candPath := filepath.Join(root, override)
		info, err := os.Lstat(candPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect workspace dir %s: %w", candPath, err)
		}
		if err == nil {
			isReparse, inspectErr := core.IsSymlinkOrReparsePoint(info, candPath)
			if inspectErr != nil {
				return "", fmt.Errorf("inspect workspace dir %s: %w", candPath, inspectErr)
			}
			if isReparse || !info.IsDir() {
				return "", fmt.Errorf("workspace dir %s is redirected or not a directory: %w", candPath, corerrors.ErrValidation)
			}
		}
		return override, nil
	}

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
	return candidates[len(candidates)-1], nil
}

func openOrCreateValidatedWorkspaceRoot(cleanRoot string, createIfMissing bool) (string, *os.Root, error) {
	absRoot, err := filepath.Abs(filepath.Clean(cleanRoot))
	if err != nil {
		return "", nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	vol := filepath.VolumeName(absRoot)
	anchor := string(filepath.Separator)
	if vol != "" {
		anchor = vol + string(filepath.Separator)
	}
	anchorInfo, err := os.Lstat(anchor)
	if err != nil {
		return "", nil, fmt.Errorf("inspect workspace root anchor %s: %w", anchor, err)
	}
	if !anchorInfo.IsDir() {
		return "", nil, fmt.Errorf("workspace root anchor %s is not a directory: %w", anchor, corerrors.ErrValidation)
	}

	current, err := os.OpenRoot(anchor)
	if err != nil {
		return "", nil, fmt.Errorf("open workspace root anchor %s: %w", anchor, err)
	}
	closeCurrent := func() {
		if current != nil {
			_ = current.Close()
		}
	}
	openedAnchor, err := current.Stat(".")
	if err != nil || !os.SameFile(anchorInfo, openedAnchor) {
		closeCurrent()
		return "", nil, fmt.Errorf("workspace root anchor %s identity changed after open: %w", anchor, corerrors.ErrValidation)
	}

	relFromAnchor, err := filepath.Rel(anchor, absRoot)
	if err != nil {
		closeCurrent()
		return "", nil, fmt.Errorf("compute workspace root relative path from %s: %w", anchor, err)
	}
	if relFromAnchor == "." {
		return absRoot, current, nil
	}

	currentAbs := anchor
	for _, part := range strings.Split(relFromAnchor, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			closeCurrent()
			return "", nil, fmt.Errorf("workspace root %s escapes anchor %s: %w", absRoot, anchor, corerrors.ErrValidation)
		}
		nextAbs := filepath.Join(currentAbs, part)
		info, statErr := current.Lstat(part)
		if errors.Is(statErr, os.ErrNotExist) {
			if !createIfMissing {
				closeCurrent()
				return "", nil, fmt.Errorf("inspect workspace root %s: %w", nextAbs, statErr)
			}
			if mkErr := current.Mkdir(part, 0o755); mkErr != nil {
				closeCurrent()
				return "", nil, fmt.Errorf("create workspace root %s: %w", nextAbs, mkErr)
			}
			info, statErr = current.Lstat(part)
		}
		if statErr != nil {
			closeCurrent()
			return "", nil, fmt.Errorf("inspect workspace root %s: %w", nextAbs, statErr)
		}
		isLink, linkErr := core.IsSymlinkOrReparsePoint(info, nextAbs)
		if linkErr != nil || isLink || info.Mode()&os.ModeIrregular != 0 || !info.IsDir() {
			closeCurrent()
			return "", nil, fmt.Errorf("workspace root %s is redirected or not a directory: %w", nextAbs, corerrors.ErrValidation)
		}
		next, openErr := current.OpenRoot(part)
		if openErr != nil {
			closeCurrent()
			return "", nil, fmt.Errorf("open workspace root %s: %w", nextAbs, openErr)
		}
		openedNext, openedErr := next.Stat(".")
		if openedErr != nil || !os.SameFile(info, openedNext) {
			_ = next.Close()
			closeCurrent()
			return "", nil, fmt.Errorf("workspace root %s identity changed after open: %w", nextAbs, corerrors.ErrValidation)
		}
		_ = current.Close()
		current = next
		currentAbs = nextAbs
	}
	return absRoot, current, nil
}

func validateTargetToolsDirInput(targetDir string) (string, error) {
	for _, r := range targetDir {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("target tools directory %q contains invalid control character: %w", targetDir, corerrors.ErrValidation)
		}
	}
	return validateDaemonToolsDirRel(targetDir)
}

func ensureAndOpenTargetToolsDir(wsRoot *os.Root, absRoot, targetDir string) (*os.Root, string, bool, error) {
	if slices.Contains(core.WorkspaceRootCandidates(), targetDir) {
		targetAbs := filepath.Join(absRoot, targetDir)
		storageInfo, err := wsRoot.Lstat(targetDir)
		if err != nil {
			return nil, "", true, fmt.Errorf("inspect storage directory %s: %w", targetDir, err)
		}
		storageLink, linkErr := core.IsSymlinkOrReparsePoint(storageInfo, targetAbs)
		if linkErr != nil || storageLink || storageInfo.Mode()&os.ModeIrregular != 0 || !storageInfo.IsDir() {
			return nil, "", true, fmt.Errorf("storage directory %s is redirected or not a directory: %w", targetDir, corerrors.ErrValidation)
		}
		storageHandle, err := wsRoot.OpenRoot(targetDir)
		if err != nil {
			return nil, "", true, fmt.Errorf("open storage directory %s: %w", targetDir, err)
		}
		openedStorage, statErr := storageHandle.Stat(".")
		if statErr != nil || !os.SameFile(storageInfo, openedStorage) {
			_ = storageHandle.Close()
			return nil, "", true, fmt.Errorf("storage directory %s identity changed after open: %w", targetDir, corerrors.ErrValidation)
		}
		return storageHandle, targetAbs, true, nil
	}

	cleanRel, err := validateTargetToolsDirInput(targetDir)
	if err != nil {
		return nil, "", false, err
	}

	segments := strings.Split(filepath.ToSlash(cleanRel), "/")
	current := wsRoot
	ownsCurrent := false
	currentAbs := absRoot
	closeOwned := func() {
		if ownsCurrent && current != nil {
			_ = current.Close()
		}
	}

	for _, part := range segments {
		if part == "" || part == "." {
			continue
		}
		nextAbs := filepath.Join(currentAbs, part)
		info, statErr := current.Lstat(part)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkErr := current.Mkdir(part, 0o755); mkErr != nil {
				closeOwned()
				return nil, "", false, fmt.Errorf("create target tools directory %s: %w", nextAbs, mkErr)
			}
			info, statErr = current.Lstat(part)
		}
		if statErr != nil {
			closeOwned()
			return nil, "", false, fmt.Errorf("inspect target tools directory %s: %w", nextAbs, statErr)
		}
		isLink, linkErr := core.IsSymlinkOrReparsePoint(info, nextAbs)
		if linkErr != nil || isLink || info.Mode()&os.ModeIrregular != 0 || !info.IsDir() {
			closeOwned()
			return nil, "", false, fmt.Errorf("target tools directory %s is redirected or not a directory: %w", nextAbs, corerrors.ErrValidation)
		}
		next, openErr := current.OpenRoot(part)
		if openErr != nil {
			closeOwned()
			return nil, "", false, fmt.Errorf("open target tools directory %s: %w", nextAbs, openErr)
		}
		opened, openedErr := next.Stat(".")
		if openedErr != nil || !os.SameFile(info, opened) {
			_ = next.Close()
			closeOwned()
			return nil, "", false, fmt.Errorf("target tools directory %s identity changed after open: %w", nextAbs, corerrors.ErrValidation)
		}
		closeOwned()
		current = next
		ownsCurrent = true
		currentAbs = nextAbs
	}
	return current, currentAbs, false, nil
}

func openValidatedWorkspaceAndStorageRoots(root, storageDir string) (string, *os.Root, *os.Root, error) {
	absRoot, wsRoot, err := openOrCreateValidatedWorkspaceRoot(root, false)
	if err != nil {
		return "", nil, nil, err
	}
	closeWSRoot := true
	defer func() {
		if closeWSRoot {
			// Best-effort handle cleanup on early error return.
			_ = wsRoot.Close()
		}
	}()

	storageHandle, _, _, err := ensureAndOpenTargetToolsDir(wsRoot, absRoot, storageDir)
	if err != nil {
		return "", nil, nil, err
	}
	closeWSRoot = false
	return absRoot, wsRoot, storageHandle, nil
}

func installWorkspaceRuntimeExecutable(root, targetDir string) (string, error) {
	if !slices.Contains(core.WorkspaceRootCandidates(), targetDir) {
		if _, err := validateTargetToolsDirInput(targetDir); err != nil {
			return "", err
		}
	}

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

	absRoot, wsRoot, err := openOrCreateValidatedWorkspaceRoot(root, false)
	if err != nil {
		return "", err
	}
	wsClosed := false
	defer func() {
		if !wsClosed {
			_ = wsRoot.Close()
		}
	}()

	targetHandle, targetDirAbs, _, err := ensureAndOpenTargetToolsDir(wsRoot, absRoot, targetDir)
	if err != nil {
		return "", err
	}
	targetClosed := false
	defer func() {
		if !targetClosed {
			_ = targetHandle.Close()
		}
	}()

	skipCopy := false
	dstInfo, dstErr := targetHandle.Lstat(binName)
	if dstErr == nil {
		dstLink, linkErr := core.IsSymlinkOrReparsePoint(dstInfo, filepath.Join(targetDirAbs, binName))
		if linkErr != nil || dstLink || !dstInfo.Mode().IsRegular() {
			return "", fmt.Errorf("existing runtime executable %s is redirected or not a regular file: %w", binName, corerrors.ErrValidation)
		}
		if os.SameFile(srcStat, dstInfo) {
			skipCopy = true
		}
	} else if !errors.Is(dstErr, os.ErrNotExist) {
		return "", fmt.Errorf("inspect destination runtime executable %s: %w", binName, dstErr)
	}

	if !skipCopy {
		tmpName := fmt.Sprintf("%s.tmp.%d.%d", binName, os.Getpid(), selfInstallTempSeq.Add(1))
		tmpFile, err := targetHandle.OpenFile(tmpName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
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
				_ = targetHandle.Remove(tmpName)
			}
		}()

		openedTmpStat, err := tmpFile.Stat()
		if err != nil {
			return "", fmt.Errorf("stat temp runtime executable: %w", err)
		}
		lstatTmpStat, err := targetHandle.Lstat(tmpName)
		if err != nil || !os.SameFile(openedTmpStat, lstatTmpStat) {
			return "", fmt.Errorf("temp runtime executable identity changed after open: %w", corerrors.ErrValidation)
		}
		if _, err := io.Copy(tmpFile, srcFile); err != nil {
			return "", fmt.Errorf("copy runtime executable: %w", err)
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

		preDstInfo, preDstErr := targetHandle.Lstat(binName)
		if preDstErr == nil {
			preLink, linkErr := core.IsSymlinkOrReparsePoint(preDstInfo, filepath.Join(targetDirAbs, binName))
			if linkErr != nil || preLink || !preDstInfo.Mode().IsRegular() {
				return "", fmt.Errorf("destination runtime executable %s became redirected or non-regular: %w", binName, corerrors.ErrValidation)
			}
		} else if !errors.Is(preDstErr, os.ErrNotExist) {
			return "", fmt.Errorf("re-inspect runtime executable %s before replace: %w", binName, preDstErr)
		}

		if err := atomicfile.ReplaceTempFileInRoot(targetHandle, targetDirAbs, tmpName, binName, true); err != nil {
			return "", fmt.Errorf("install runtime executable %s: %w", binName, err)
		}
		tmpRemoved = true
	}

	srcClosed = true
	if err := srcFile.Close(); err != nil {
		return "", fmt.Errorf("close source runtime executable: %w", err)
	}

	targetClosed = true
	if err := targetHandle.Close(); err != nil {
		return "", fmt.Errorf("close target directory %s: %w", targetDir, err)
	}
	wsClosed = true
	if err := wsRoot.Close(); err != nil {
		return "", fmt.Errorf("close workspace root: %w", err)
	}
	return binName, nil
}

func escapeGitignorePatternPath(relSlash string) string {
	var b strings.Builder
	b.Grow(len(relSlash))
	for i := 0; i < len(relSlash); i++ {
		c := relSlash[i]
		switch c {
		case '\\', '*', '?', '[', ']', '#', '!':
			b.WriteByte('\\')
			b.WriteByte(c)
		case ' ':
			if i == len(relSlash)-1 || relSlash[i+1] == '/' {
				b.WriteString(`\ `)
			} else {
				b.WriteByte(' ')
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func workspaceRuntimeIgnoreEntries() []string {
	var entries []string
	for _, candidate := range core.WorkspaceRootCandidates() {
		entries = append(entries,
			"/"+candidate+"/backlogit",
			"/"+candidate+"/backlogit.exe",
			"/"+candidate+"/.locks/",
		)
	}
	return entries
}

func ensureWorkspaceRuntimeIgnored(root, toolsDirRel, binName string) error {
	var extraEntries []string
	if toolsDirRel != "" && !slices.Contains(core.WorkspaceRootCandidates(), toolsDirRel) {
		cleanToolsDir, err := validateTargetToolsDirInput(toolsDirRel)
		if err != nil {
			return err
		}
		escaped := escapeGitignorePatternPath(filepath.ToSlash(cleanToolsDir))
		extraEntries = append(extraEntries,
			"/"+escaped+"/backlogit",
			"/"+escaped+"/backlogit.exe",
		)
		if binName != "" && binName != "backlogit" && binName != "backlogit.exe" {
			extraEntries = append(extraEntries, "/"+escaped+"/"+binName)
		}
	}

	absRoot, wsRoot, err := openOrCreateValidatedWorkspaceRoot(root, false)
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

	entries := append(workspaceRuntimeIgnoreEntries(), extraEntries...)
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
	eol := "\n"
	if strings.Contains(existing, "\r\n") {
		eol = "\r\n"
	}
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
		b.WriteString(eol)
	}
	for _, entry := range missing {
		b.WriteString(entry)
		b.WriteString(eol)
	}
	return b.String(), true
}
