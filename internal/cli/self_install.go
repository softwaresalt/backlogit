package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/toolchain"
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

func ensureAndOpenTargetToolsDirWithCreated(wsRoot *os.Root, absRoot, targetDir string) (*os.Root, string, []string, error) {
	if slices.Contains(core.WorkspaceRootCandidates(), targetDir) {
		targetAbs := filepath.Join(absRoot, targetDir)
		storageInfo, err := wsRoot.Lstat(targetDir)
		if err != nil {
			return nil, "", nil, fmt.Errorf("inspect storage directory %s: %w", targetDir, err)
		}
		storageLink, linkErr := core.IsSymlinkOrReparsePoint(storageInfo, targetAbs)
		if linkErr != nil || storageLink || storageInfo.Mode()&os.ModeIrregular != 0 || !storageInfo.IsDir() {
			return nil, "", nil, fmt.Errorf("storage directory %s is redirected or not a directory: %w", targetDir, corerrors.ErrValidation)
		}
		storageHandle, err := wsRoot.OpenRoot(targetDir)
		if err != nil {
			return nil, "", nil, fmt.Errorf("open storage directory %s: %w", targetDir, err)
		}
		openedStorage, statErr := storageHandle.Stat(".")
		if statErr != nil || !os.SameFile(storageInfo, openedStorage) {
			_ = storageHandle.Close()
			return nil, "", nil, fmt.Errorf("storage directory %s identity changed after open: %w", targetDir, corerrors.ErrValidation)
		}
		return storageHandle, targetAbs, nil, nil
	}

	cleanRel, err := validateTargetToolsDirInput(targetDir)
	if err != nil {
		return nil, "", nil, err
	}

	segments := strings.Split(filepath.ToSlash(cleanRel), "/")
	current := wsRoot
	ownsCurrent := false
	currentAbs := absRoot
	var createdDirs []string
	var builtParts []string
	closeOwned := func() {
		if ownsCurrent && current != nil {
			_ = current.Close()
		}
	}

	for _, part := range segments {
		if part == "" || part == "." {
			continue
		}
		builtParts = append(builtParts, part)
		nextAbs := filepath.Join(currentAbs, part)
		info, statErr := current.Lstat(part)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkErr := current.Mkdir(part, 0o755); mkErr != nil {
				closeOwned()
				return nil, "", createdDirs, fmt.Errorf("create target tools directory %s: %w", nextAbs, mkErr)
			}
			createdDirs = append(createdDirs, strings.Join(builtParts, "/"))
			info, statErr = current.Lstat(part)
		}
		if statErr != nil {
			closeOwned()
			return nil, "", createdDirs, fmt.Errorf("inspect target tools directory %s: %w", nextAbs, statErr)
		}
		isLink, linkErr := core.IsSymlinkOrReparsePoint(info, nextAbs)
		if linkErr != nil || isLink || info.Mode()&os.ModeIrregular != 0 || !info.IsDir() {
			closeOwned()
			return nil, "", createdDirs, fmt.Errorf("target tools directory %s is redirected or not a directory: %w", nextAbs, corerrors.ErrValidation)
		}
		next, openErr := current.OpenRoot(part)
		if openErr != nil {
			closeOwned()
			return nil, "", createdDirs, fmt.Errorf("open target tools directory %s: %w", nextAbs, openErr)
		}
		opened, openedErr := next.Stat(".")
		if openedErr != nil || !os.SameFile(info, opened) {
			_ = next.Close()
			closeOwned()
			return nil, "", createdDirs, fmt.Errorf("target tools directory %s identity changed after open: %w", nextAbs, corerrors.ErrValidation)
		}
		closeOwned()
		current = next
		ownsCurrent = true
		currentAbs = nextAbs
	}
	return current, currentAbs, createdDirs, nil
}

func writeFileAtomicallyInRoot(targetRoot *os.Root, targetDirAbs, fileName string, data []byte, defaultMode os.FileMode, execTarget bool, beforeReplaceHook func(string)) error {
	return writeFileAtomicallyWithModeInRoot(targetRoot, targetDirAbs, fileName, data, defaultMode, false, execTarget, beforeReplaceHook)
}

func writeFileAtomicallyWithModeInRoot(targetRoot *os.Root, targetDirAbs, fileName string, data []byte, defaultMode os.FileMode, forceMode, execTarget bool, beforeReplaceHook func(string)) error {
	mode := defaultMode
	targetAbs := filepath.Join(targetDirAbs, fileName)
	info, statErr := targetRoot.Lstat(fileName)
	if statErr == nil {
		link, linkErr := core.IsSymlinkOrReparsePoint(info, targetAbs)
		if linkErr != nil || link || info.Mode()&os.ModeIrregular != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("%s is redirected or not a regular file: %w", fileName, corerrors.ErrValidation)
		}
		if !forceMode {
			mode = info.Mode().Perm() &^ 0o022
			if mode == 0 {
				mode = defaultMode
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", fileName, statErr)
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

	openedStat, err := tf.Stat()
	if err != nil {
		return fmt.Errorf("stat temp %s: %w", fileName, err)
	}
	lstatStat, err := targetRoot.Lstat(tmpName)
	if err != nil || !os.SameFile(openedStat, lstatStat) {
		return fmt.Errorf("temp %s identity changed after open: %w", fileName, corerrors.ErrValidation)
	}
	written := 0
	for written < len(data) {
		n, wErr := tf.Write(data[written:])
		if wErr != nil {
			return fmt.Errorf("write temp %s: %w", fileName, wErr)
		}
		if n == 0 {
			return fmt.Errorf("short write to temp %s: %w", fileName, io.ErrShortWrite)
		}
		written += n
	}
	if err := tf.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp %s: %w", fileName, err)
	}
	if err := tf.Sync(); err != nil {
		return fmt.Errorf("sync temp %s: %w", fileName, err)
	}
	tfClosed = true
	if err := tf.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", fileName, err)
	}

	if beforeReplaceHook != nil {
		beforeReplaceHook(fileName)
	}

	preInfo, preErr := targetRoot.Lstat(fileName)
	if preErr == nil {
		preLink, linkErr := core.IsSymlinkOrReparsePoint(preInfo, targetAbs)
		if linkErr != nil || preLink || preInfo.Mode()&os.ModeIrregular != 0 || !preInfo.Mode().IsRegular() {
			return fmt.Errorf("%s became redirected or non-regular before replace: %w", fileName, corerrors.ErrValidation)
		}
	} else if !errors.Is(preErr, os.ErrNotExist) {
		return fmt.Errorf("re-inspect %s before replace: %w", fileName, preErr)
	}

	if err := atomicfile.ReplaceTempFileInRoot(targetRoot, targetDirAbs, tmpName, fileName, execTarget); err != nil {
		return fmt.Errorf("replace %s: %w", fileName, err)
	}
	tmpRemoved = true
	return nil
}

func parseAndValidateMCPJSONRaw(wsRoot *os.Root, absRoot string) (map[string]json.RawMessage, string, map[string]json.RawMessage, string, map[string]json.RawMessage, string, map[string]json.RawMessage, error) {
	top := make(map[string]json.RawMessage)
	existingBytes, err := readLocalFileInRoot(wsRoot, absRoot, ".mcp.json", maxMCPDaemonInfoBytes)
	if errors.Is(err, os.ErrNotExist) {
		return top, "", nil, "", nil, "", nil, nil
	}
	if err != nil {
		return nil, "", nil, "", nil, "", nil, err
	}
	if err := scanDaemonJSON(existingBytes); err != nil {
		return nil, "", nil, "", nil, "", nil, err
	}
	trimmed := bytes.TrimSpace(existingBytes)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError(".mcp.json top-level must be a JSON object")
	}
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError("decode .mcp.json: %v", err)
	}

	var mcpKey, srvKey string
	var mcpMap, srvMap map[string]json.RawMessage
	type matchedEntry struct {
		section string
		key     string
		raw     map[string]json.RawMessage
		parsed  daemonMCPJSONServer
	}
	var matched []matchedEntry

	for _, sec := range []string{"mcpServers", "servers"} {
		var actualKey string
		var rawSec json.RawMessage
		for k, v := range top {
			if foldDaemonKey(k) == foldDaemonKey(sec) {
				actualKey = k
				rawSec = v
				break
			}
		}
		if actualKey == "" {
			continue
		}
		if _, err := decodeDaemonMCPServersMap(rawSec, sec); err != nil {
			return nil, "", nil, "", nil, "", nil, err
		}
		var secEntries map[string]json.RawMessage
		if err := json.Unmarshal(rawSec, &secEntries); err != nil {
			return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError("decode .mcp.json %s: %v", sec, err)
		}
		if sec == "mcpServers" {
			mcpKey = actualKey
			mcpMap = secEntries
		} else {
			srvKey = actualKey
			srvMap = secEntries
		}

		names := make([]string, 0, len(secEntries))
		for name := range secEntries {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			var parsed daemonMCPJSONServer
			if err := json.Unmarshal(secEntries[name], &parsed); err != nil {
				return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError("decode .mcp.json %s.%s: %v", sec, name, err)
			}
			if foldDaemonKey(strings.TrimSpace(name)) == foldDaemonKey("backlogit") || isBacklogitDaemonCommand(parsed.Command) {
				var rawObj map[string]json.RawMessage
				if err := json.Unmarshal(secEntries[name], &rawObj); err != nil {
					return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError("decode .mcp.json %s.%s object: %v", sec, name, err)
				}
				matched = append(matched, matchedEntry{section: sec, key: name, raw: rawObj, parsed: parsed})
			}
		}
	}

	if len(matched) > 1 {
		return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError("ambiguous multiple backlogit entries in .mcp.json")
	}
	var matchedSec string
	var matchedRaw map[string]json.RawMessage
	if len(matched) == 1 {
		srvType := strings.TrimSpace(matched[0].parsed.Type)
		if strings.TrimSpace(matched[0].parsed.URL) != "" || (srvType != "" && !strings.EqualFold(srvType, "stdio")) {
			return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError(".mcp.json backlogit entry must use stdio transport without url")
		}
		matchedSec = matched[0].section
		matchedRaw = matched[0].raw
		if matched[0].section == "mcpServers" {
			delete(mcpMap, matched[0].key)
		} else {
			delete(srvMap, matched[0].key)
		}
	}
	if srvKey != "" && len(srvMap) > 0 {
		return nil, "", nil, "", nil, "", nil, mcpDaemonValidationError(".mcp.json servers block contains non-backlogit entries")
	}
	return top, mcpKey, mcpMap, srvKey, srvMap, matchedSec, matchedRaw, nil
}

func configureWorkspaceMCPJSON(wsRoot *os.Root, absRoot, toolsDirRel, binName string) error {
	cleanToolsDir, err := validateTargetToolsDirInput(toolsDirRel)
	if err != nil {
		return err
	}
	installedFile, _, err := openLocalFileInRoot(wsRoot, absRoot, filepath.Join(cleanToolsDir, binName))
	if err != nil {
		return fmt.Errorf("verify installed tool executable before configuring .mcp.json: %w", err)
	}
	if closeErr := installedFile.Close(); closeErr != nil {
		return fmt.Errorf("close verified tool executable: %w", closeErr)
	}

	top, mcpKey, mcpMap, srvKey, _, _, matchedRaw, err := parseAndValidateMCPJSONRaw(wsRoot, absRoot)
	if err != nil {
		return err
	}
	if mcpKey != "" && mcpKey != "mcpServers" {
		delete(top, mcpKey)
	}
	if srvKey != "" {
		delete(top, srvKey)
	}
	if mcpMap == nil {
		mcpMap = make(map[string]json.RawMessage)
	}

	entryObj := make(map[string]json.RawMessage)
	for k, v := range matchedRaw {
		switch foldDaemonKey(k) {
		case foldDaemonKey("type"), foldDaemonKey("command"), foldDaemonKey("args"), foldDaemonKey("url"):
			continue
		default:
			entryObj[k] = v
		}
	}
	typeJSON, err := json.Marshal("stdio")
	if err != nil {
		return fmt.Errorf("marshal .mcp.json type: %w", err)
	}
	cmdJSON, err := json.Marshal("./" + filepath.ToSlash(filepath.Join(cleanToolsDir, binName)))
	if err != nil {
		return fmt.Errorf("marshal .mcp.json command: %w", err)
	}
	argsJSON, err := json.Marshal([]string{"mcp", "--cwd", absRoot})
	if err != nil {
		return fmt.Errorf("marshal .mcp.json args: %w", err)
	}
	entryObj["type"] = typeJSON
	entryObj["command"] = cmdJSON
	entryObj["args"] = argsJSON

	entryBytes, err := json.Marshal(entryObj)
	if err != nil {
		return fmt.Errorf("marshal .mcp.json backlogit entry: %w", err)
	}
	mcpMap["backlogit"] = json.RawMessage(entryBytes)

	mcpMapBytes, err := json.Marshal(mcpMap)
	if err != nil {
		return fmt.Errorf("marshal .mcp.json mcpServers: %w", err)
	}
	top["mcpServers"] = json.RawMessage(mcpMapBytes)

	formatted, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("format .mcp.json: %w", err)
	}
	formatted = append(formatted, '\n')

	return writeFileAtomicallyInRoot(wsRoot, absRoot, ".mcp.json", formatted, 0o644, false, selfInstallBeforeIgnoreReplaceHook)
}

func migrateLauncherBytes(data []byte, oldToolsDir, newToolsDir, binName string) ([]byte, bool) {
	oldSlash := filepath.ToSlash(filepath.Clean(oldToolsDir))
	newSlash := filepath.ToSlash(filepath.Clean(newToolsDir))
	if oldSlash == newSlash {
		return data, false
	}
	oldWin := filepath.FromSlash(oldSlash)
	newWin := filepath.FromSlash(newSlash)

	text := string(data)
	shebang := ""
	rest := text
	if strings.HasPrefix(text, "#!") {
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			shebang = text[:idx+1]
			rest = text[idx+1:]
		} else {
			return data, false
		}
	}

	pairs := []string{
		"./" + oldSlash + "/" + binName, "./" + newSlash + "/" + binName,
		"./" + oldSlash + "/backlogit.exe", "./" + newSlash + "/" + binName,
		"./" + oldSlash + "/backlogit", "./" + newSlash + "/" + binName,
		`.\` + oldWin + `\` + binName, `.\` + newWin + `\` + binName,
		`.\` + oldWin + `\backlogit.exe`, `.\` + newWin + `\` + binName,
		`.\` + oldWin + `\backlogit`, `.\` + newWin + `\` + binName,
		`"$workspace_root/` + oldSlash + `/`, `"$workspace_root/` + newSlash + `/`,
		`"$workspace_root/` + oldSlash + `"`, `"$workspace_root/` + newSlash + `"`,
		`"${workspace_root}/` + oldSlash + `/`, `"${workspace_root}/` + newSlash + `/`,
		`"${workspace_root}/` + oldSlash + `"`, `"${workspace_root}/` + newSlash + `"`,
		`"` + oldSlash + `"`, `"` + newSlash + `"`,
		`'` + oldSlash + `'`, `'` + newSlash + `'`,
	}
	if oldWin != oldSlash {
		pairs = append(pairs,
			`"`+oldWin+`"`, `"`+newWin+`"`,
			`'`+oldWin+`'`, `'`+newWin+`'`,
		)
	}
	replacer := strings.NewReplacer(pairs...)
	updated := shebang + replacer.Replace(rest)
	return []byte(updated), updated != text
}

func sha256FileInRoot(wsRoot *os.Root, absRoot, rel string) (string, error) {
	f, _, err := openLocalFileInRoot(wsRoot, absRoot, rel)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	if copyErr != nil {
		return "", fmt.Errorf("hash %s: %w", rel, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close %s: %w", rel, closeErr)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func reconcileWorkspaceToolchainYAML(wsRoot *os.Root, absRoot, toolsDirRel, binName, installedSHA256Hex string) error {
	cleanToolsDir, err := validateTargetToolsDirInput(toolsDirRel)
	if err != nil {
		return err
	}
	storageDir, hasStorage, err := resolveDaemonStorageDirName(wsRoot, absRoot)
	if err != nil || !hasStorage {
		return err
	}

	storageInfo, err := wsRoot.Lstat(storageDir)
	if err != nil {
		return fmt.Errorf("inspect storage directory %s: %w", storageDir, err)
	}
	storageAbs := filepath.Join(absRoot, storageDir)
	storageLink, linkErr := core.IsSymlinkOrReparsePoint(storageInfo, storageAbs)
	if linkErr != nil || storageLink || storageInfo.Mode()&os.ModeIrregular != 0 || !storageInfo.IsDir() {
		return fmt.Errorf("storage directory %s is redirected or not a directory: %w", storageDir, corerrors.ErrValidation)
	}
	storageHandle, err := wsRoot.OpenRoot(storageDir)
	if err != nil {
		return fmt.Errorf("open storage directory %s: %w", storageDir, err)
	}
	defer func() { _ = storageHandle.Close() }()
	openedStorage, statErr := storageHandle.Stat(".")
	if statErr != nil || !os.SameFile(storageInfo, openedStorage) {
		return fmt.Errorf("storage directory %s identity changed after open: %w", storageDir, corerrors.ErrValidation)
	}

	tcRel := filepath.Join(storageDir, "toolchain.yaml")
	data, readErr := readLocalFileInRoot(wsRoot, absRoot, tcRel, maxMCPDaemonInfoBytes)
	if errors.Is(readErr, os.ErrNotExist) {
		return nil
	}
	if readErr != nil {
		return readErr
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return mcpDaemonValidationError("parse toolchain.yaml: %v", err)
	}
	if err := validateDaemonYAMLKeys(&doc, "toolchain.yaml"); err != nil {
		return err
	}
	if hasYAMLMappingKey(&doc, "tools_dir") {
		if valNode, ok := findYAMLMappingValueNode(&doc, "tools_dir"); ok {
			if valNode.Kind != yaml.ScalarNode || valNode.Tag != "!!str" {
				return mcpDaemonValidationError("toolchain.yaml tools_dir must be a string scalar")
			}
		}
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var manifest toolchain.ToolchainManifest
	if err := decoder.Decode(&manifest); err != nil {
		return mcpDaemonValidationError("decode toolchain.yaml: %v", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return mcpDaemonValidationError("additional YAML document in toolchain.yaml")
	}

	loaded, loadErr := toolchain.LoadToolchainManifest(absRoot)
	if loadErr != nil {
		return loadErr
	}
	manifest = *loaded

	var matchedIdxs []int
	for i, tool := range manifest.Tools {
		if strings.EqualFold(strings.TrimSpace(tool.ID), "backlogit") || isBacklogitDaemonCommand(tool.ExecutablePath) {
			matchedIdxs = append(matchedIdxs, i)
		}
	}
	if len(matchedIdxs) != 1 {
		return mcpDaemonValidationError("toolchain.yaml must declare exactly 1 backlogit entry, found %d", len(matchedIdxs))
	}
	idx := matchedIdxs[0]
	bt := manifest.Tools[idx]
	if bt.ContainmentClass != toolchain.WorkspaceLocalPinned ||
		!strings.EqualFold(strings.TrimSpace(bt.Kind), "mcp") ||
		strings.TrimSpace(bt.EntrypointPath) != "" {
		return mcpDaemonValidationError("toolchain.yaml backlogit entry must be workspace-local-pinned kind mcp without entrypoint_path")
	}

	for i, tool := range manifest.Tools {
		if i != idx && strings.EqualFold(strings.TrimSpace(tool.ID), "backlogit") {
			return mcpDaemonValidationError("duplicate backlogit tool identity in toolchain.yaml")
		}
	}
	for _, prereq := range manifest.HostPrerequisites {
		if strings.EqualFold(strings.TrimSpace(prereq.ID), "backlogit") {
			return mcpDaemonValidationError("duplicate backlogit identity in host_prerequisites")
		}
	}

	oldToolsDir := manifest.ToolsDir
	if strings.TrimSpace(oldToolsDir) != "" {
		if _, tdErr := validateTargetToolsDirInput(oldToolsDir); tdErr != nil {
			return tdErr
		}
	} else if hasYAMLMappingKey(&doc, "tools_dir") {
		if _, tdErr := validateTargetToolsDirInput(oldToolsDir); tdErr != nil {
			return tdErr
		}
	}
	if strings.TrimSpace(oldToolsDir) == "" {
		oldRel, relErr := validateBacklogitCommandRel(absRoot, bt.ExecutablePath)
		if relErr != nil {
			return relErr
		}
		oldToolsDir = filepath.ToSlash(filepath.Dir(oldRel))
	}

	if strings.TrimSpace(bt.PinnedVersion) == "" && strings.TrimSpace(bt.LockfilePath) == "" {
		return mcpDaemonValidationError("toolchain.yaml backlogit entry missing pinned_version or lockfile_path")
	}
	if strings.TrimSpace(bt.SHA256) == "" && strings.TrimSpace(bt.LockfilePath) == "" {
		return mcpDaemonValidationError("toolchain.yaml backlogit entry missing sha256 or lockfile_path")
	}
	if err := validateDaemonBacklogitBindings(wsRoot, absRoot, bt.WorkspaceBindingArgs); err != nil {
		return err
	}

	if strings.TrimSpace(bt.LockfilePath) != "" {
		if err := validateDaemonLockfileRel(bt.LockfilePath); err != nil {
			return err
		}
		lockData, lockErr := readLocalFileInRoot(wsRoot, absRoot, bt.LockfilePath, maxMCPDaemonInfoBytes)
		if lockErr != nil {
			return mcpDaemonValidationError("read backlogit lockfile_path %q: %v", bt.LockfilePath, lockErr)
		}
		versionPin := strings.TrimSpace(bt.PinnedVersion)
		if len(bytes.TrimSpace(lockData)) == 0 ||
			!strings.Contains(string(lockData), "backlogit") ||
			(versionPin != "" && !strings.Contains(string(lockData), versionPin)) {
			return mcpDaemonValidationError("toolchain.yaml backlogit lockfile_path %q lacks canonical backlogit provenance", bt.LockfilePath)
		}
	}

	newToolsSlash := filepath.ToSlash(cleanToolsDir)
	proposedManifest := manifest
	proposedManifest.Tools = append([]toolchain.ToolEntry(nil), manifest.Tools...)
	proposedManifest.HostPrerequisites = append([]toolchain.HostPrerequisiteEntry(nil), manifest.HostPrerequisites...)
	proposedManifest.WorkspaceRoot = absRoot
	if manifest.ToolsDir != "" {
		proposedManifest.ToolsDir = newToolsSlash
	}
	proposedManifest.Tools[idx].ID = "backlogit"
	proposedManifest.Tools[idx].ExecutablePath = "./" + newToolsSlash + "/" + binName

	if installedSHA256Hex != "" {
		proposedManifest.Tools[idx].SHA256 = installedSHA256Hex
	} else {
		// In preflight mode, if the target binary already exists on disk (e.g.
		// upgrading in place), align the preflight copy with the on-disk bytes
		// so VerifyProposedLauncherScripts validates against current disk state.
		if existingSum, sumErr := sha256FileInRoot(wsRoot, absRoot, filepath.Join(cleanToolsDir, binName)); sumErr == nil {
			proposedManifest.Tools[idx].SHA256 = existingSum
		}
	}

	unchangedTools := make([]toolchain.ToolEntry, 0, len(proposedManifest.Tools)-1)
	for i, tool := range proposedManifest.Tools {
		if i != idx {
			unchangedTools = append(unchangedTools, tool)
		}
	}
	unchangedManifest := proposedManifest
	unchangedManifest.Tools = unchangedTools
	unchangedReport, unchangedErr := toolchain.VerifyWorkspaceToolchain(context.Background(), absRoot, &unchangedManifest)
	if unchangedErr != nil {
		return unchangedErr
	}
	if unchangedReport == nil {
		return mcpDaemonValidationError("unchanged toolchain inventory verification returned nil report")
	}
	if !unchangedReport.Valid || unchangedReport.Violations > 0 {
		return mcpDaemonValidationError("unchanged toolchain inventory contains %d violation(s)", unchangedReport.Violations)
	}

	var proposedPS1, proposedSH []byte
	var ps1Migrated, shMigrated bool
	if ps1Bytes, ps1Err := readLocalFileInRoot(wsRoot, absRoot, "start.ps1", maxMCPDaemonInfoBytes); ps1Err == nil {
		proposedPS1, ps1Migrated = migrateLauncherBytes(ps1Bytes, oldToolsDir, newToolsSlash, binName)
	} else if !errors.Is(ps1Err, os.ErrNotExist) {
		return ps1Err
	}
	if shBytes, shErr := readLocalFileInRoot(wsRoot, absRoot, "start.sh", maxMCPDaemonInfoBytes); shErr == nil {
		proposedSH, shMigrated = migrateLauncherBytes(shBytes, oldToolsDir, newToolsSlash, binName)
	} else if !errors.Is(shErr, os.ErrNotExist) {
		return shErr
	}

	if proposedPS1 != nil || proposedSH != nil {
		launcherFindings, lErr := toolchain.VerifyProposedLauncherScripts(absRoot, &proposedManifest, proposedPS1, proposedSH)
		if lErr != nil {
			return lErr
		}
		for _, f := range launcherFindings {
			if f.Status == "violation" {
				return mcpDaemonValidationError("launcher script %s (%s) verification failed: %s", f.ToolID, f.Code, f.Message)
			}
		}
	}

	if installedSHA256Hex == "" {
		return nil
	}

	fullReport, fullErr := toolchain.VerifyWorkspaceToolchain(context.Background(), absRoot, &proposedManifest)
	if fullErr != nil {
		return fullErr
	}
	if fullReport == nil {
		return mcpDaemonValidationError("proposed toolchain manifest verification returned nil report")
	}
	if !fullReport.Valid || fullReport.Violations > 0 {
		return mcpDaemonValidationError("proposed toolchain manifest contains %d violation(s)", fullReport.Violations)
	}

	encodedManifest, marshalErr := yaml.Marshal(&proposedManifest)
	if marshalErr != nil {
		return fmt.Errorf("marshal reconciled toolchain.yaml: %w", marshalErr)
	}
	if err := writeFileAtomicallyInRoot(storageHandle, storageAbs, "toolchain.yaml", encodedManifest, 0o600, true, nil); err != nil {
		return err
	}
	if ps1Migrated {
		if err := writeFileAtomicallyInRoot(wsRoot, absRoot, "start.ps1", proposedPS1, 0o644, false, nil); err != nil {
			return err
		}
	}
	if shMigrated {
		if err := writeFileAtomicallyInRoot(wsRoot, absRoot, "start.sh", proposedSH, 0o755, false, nil); err != nil {
			return err
		}
	}

	if proposedPS1 != nil && proposedSH != nil {
		diskFindings, diskErr := toolchain.VerifyLauncherScripts(absRoot)
		if diskErr != nil {
			return diskErr
		}
		for _, f := range diskFindings {
			if f.Status == "violation" {
				return mcpDaemonValidationError("disk launcher script %s verification failed: %s", f.ToolID, f.Message)
			}
		}
	} else {
		mcpFindings, mcpErr := toolchain.ValidateMCPConfigPinning(absRoot, ".mcp.json", &proposedManifest)
		if mcpErr != nil {
			return mcpErr
		}
		for _, f := range mcpFindings {
			if f.Status == "violation" {
				return mcpDaemonValidationError("disk .mcp.json %s verification failed: %s", f.ToolID, f.Message)
			}
		}
	}
	return nil
}

type selfInstallPreImage struct {
	rel     string
	existed bool
	data    []byte
	mode    os.FileMode
}

func captureSelfInstallPreImage(wsRoot *os.Root, absRoot, rel string) (selfInstallPreImage, error) {
	img := selfInstallPreImage{rel: rel}
	f, _, err := openLocalFileInRoot(wsRoot, absRoot, rel)
	if errors.Is(err, os.ErrNotExist) {
		return img, nil
	}
	if err != nil {
		return img, err
	}
	info, statErr := f.Stat()
	data, readErr := io.ReadAll(f)
	closeErr := f.Close()
	if statErr != nil {
		return img, statErr
	}
	if readErr != nil {
		return img, readErr
	}
	if closeErr != nil {
		return img, closeErr
	}
	img.existed = true
	img.data = data
	img.mode = info.Mode().Perm()
	return img, nil
}

func restoreSelfInstallPreImage(wsRoot *os.Root, absRoot string, img selfInstallPreImage) error {
	clean := filepath.Clean(img.rel)
	dirRel := filepath.Dir(clean)
	baseName := filepath.Base(clean)

	targetRoot := wsRoot
	targetAbs := absRoot
	var owned []*os.Root
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			_ = owned[i].Close()
		}
	}()
	if dirRel != "." && dirRel != "" {
		for _, part := range strings.Split(filepath.ToSlash(dirRel), "/") {
			if part == "" || part == "." {
				continue
			}
			nextAbs := filepath.Join(targetAbs, part)
			info, err := targetRoot.Lstat(part)
			if err != nil {
				if !img.existed && errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return fmt.Errorf("rollback inspect %s: %w", nextAbs, err)
			}
			link, linkErr := core.IsSymlinkOrReparsePoint(info, nextAbs)
			if linkErr != nil || link || !info.IsDir() {
				return fmt.Errorf("rollback %s is redirected or not a directory: %w", nextAbs, corerrors.ErrValidation)
			}
			next, openErr := targetRoot.OpenRoot(part)
			if openErr != nil {
				return fmt.Errorf("rollback open %s: %w", nextAbs, openErr)
			}
			owned = append(owned, next)
			targetRoot = next
			targetAbs = nextAbs
		}
	}

	if !img.existed {
		info, err := targetRoot.Lstat(baseName)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("rollback inspect %s: %w", filepath.Join(targetAbs, baseName), err)
		}
		link, linkErr := core.IsSymlinkOrReparsePoint(info, filepath.Join(targetAbs, baseName))
		if linkErr != nil || link || !info.Mode().IsRegular() {
			return fmt.Errorf("rollback %s is redirected or not a regular file: %w", filepath.Join(targetAbs, baseName), corerrors.ErrValidation)
		}
		if err := targetRoot.Remove(baseName); err != nil {
			return fmt.Errorf("rollback remove %s: %w", filepath.Join(targetAbs, baseName), err)
		}
		return nil
	}
	return writeFileAtomicallyWithModeInRoot(targetRoot, targetAbs, baseName, img.data, img.mode, true, false, nil)
}

func removeCreatedRelativeDir(wsRoot *os.Root, absRoot, relSlash string) error {
	parts := strings.Split(strings.Trim(relSlash, "/"), "/")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		return nil
	}
	targetRoot := wsRoot
	targetAbs := absRoot
	var owned []*os.Root
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			_ = owned[i].Close()
		}
	}()
	for _, part := range parts[:len(parts)-1] {
		if part == "" || part == "." {
			continue
		}
		nextAbs := filepath.Join(targetAbs, part)
		info, err := targetRoot.Lstat(part)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("rollback inspect created dir %s: %w", nextAbs, err)
		}
		link, linkErr := core.IsSymlinkOrReparsePoint(info, nextAbs)
		if linkErr != nil || link || !info.IsDir() {
			return fmt.Errorf("rollback created dir parent %s is redirected or not a directory: %w", nextAbs, corerrors.ErrValidation)
		}
		next, openErr := targetRoot.OpenRoot(part)
		if openErr != nil {
			return fmt.Errorf("rollback open created dir parent %s: %w", nextAbs, openErr)
		}
		owned = append(owned, next)
		targetRoot = next
		targetAbs = nextAbs
	}
	leaf := parts[len(parts)-1]
	info, err := targetRoot.Lstat(leaf)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("rollback inspect created dir %s: %w", filepath.Join(targetAbs, leaf), err)
	}
	link, linkErr := core.IsSymlinkOrReparsePoint(info, filepath.Join(targetAbs, leaf))
	if linkErr != nil || link || !info.IsDir() {
		return fmt.Errorf("rollback created dir %s is redirected or not a directory: %w", filepath.Join(targetAbs, leaf), corerrors.ErrValidation)
	}
	if err := targetRoot.Remove(leaf); err != nil {
		return fmt.Errorf("rollback remove created dir %s: %w", filepath.Join(targetAbs, leaf), err)
	}
	return nil
}

func installWorkspaceRuntimeExecutable(root, targetDir string) (installedBinName string, retErr error) {
	isStorageRoot := slices.Contains(core.WorkspaceRootCandidates(), targetDir)
	var cleanToolsDir string
	if !isStorageRoot {
		var err error
		cleanToolsDir, err = validateTargetToolsDirInput(targetDir)
		if err != nil {
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

	var preImages []selfInstallPreImage
	if !isStorageRoot {
		// Read-only preflight before creating targetDir or writing binary/.mcp.json.
		if _, _, _, _, _, _, _, mcpErr := parseAndValidateMCPJSONRaw(wsRoot, absRoot); mcpErr != nil {
			return "", mcpErr
		}
		if tcErr := reconcileWorkspaceToolchainYAML(wsRoot, absRoot, cleanToolsDir, binName, ""); tcErr != nil {
			return "", tcErr
		}

		prePaths := []string{
			filepath.Join(cleanToolsDir, binName),
			".mcp.json",
			"start.ps1",
			"start.sh",
		}
		storageDir, hasStorage, storErr := resolveDaemonStorageDirName(wsRoot, absRoot)
		if storErr != nil {
			return "", storErr
		}
		if hasStorage {
			prePaths = append(prePaths, filepath.Join(storageDir, "toolchain.yaml"))
		}
		for _, p := range prePaths {
			img, imgErr := captureSelfInstallPreImage(wsRoot, absRoot, p)
			if imgErr != nil {
				return "", imgErr
			}
			preImages = append(preImages, img)
		}
	}

	var createdDirs []string
	rollbackAll := func() error {
		var errs []error
		for i := len(preImages) - 1; i >= 0; i-- {
			if rbErr := restoreSelfInstallPreImage(wsRoot, absRoot, preImages[i]); rbErr != nil {
				errs = append(errs, rbErr)
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			if rmErr := removeCreatedRelativeDir(wsRoot, absRoot, createdDirs[i]); rmErr != nil {
				errs = append(errs, rmErr)
			}
		}
		return errors.Join(errs...)
	}

	rollbackActive := false
	defer func() {
		if rollbackActive && retErr != nil {
			retErr = errors.Join(retErr, rollbackAll())
		}
	}()

	targetHandle, targetDirAbs, newDirs, err := ensureAndOpenTargetToolsDirWithCreated(wsRoot, absRoot, targetDir)
	createdDirs = newDirs
	rollbackActive = true
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

	if !isStorageRoot {
		installedSHA256, sumErr := sha256FileInRoot(wsRoot, absRoot, filepath.Join(cleanToolsDir, binName))
		if sumErr != nil {
			return "", sumErr
		}
		if mcpErr := configureWorkspaceMCPJSON(wsRoot, absRoot, cleanToolsDir, binName); mcpErr != nil {
			return "", mcpErr
		}
		if tcErr := reconcileWorkspaceToolchainYAML(wsRoot, absRoot, cleanToolsDir, binName, installedSHA256); tcErr != nil {
			return "", tcErr
		}
	}

	rollbackActive = false
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
