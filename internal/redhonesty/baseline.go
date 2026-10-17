package redhonesty

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

var (
	// baselineRel is the sanctioned workspace-contained, git-ignored location.
	baselineRel = []string{".backlogit", "runtime", "redhonesty"}
	// windowsReserved are device names Windows cannot hold as file names.
	windowsReserved = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])(\..*)?$`)
)

// ResolveCommit resolves ref to a full immutable commit SHA. The ref is vetted
// before git sees it (no option-like or control-character refs) and the result
// must be a full hex object name, so a branch moving later cannot change what
// was measured.
func ResolveCommit(ctx context.Context, g Git, root, ref string) (string, error) {
	if ref == "" || len(ref) > 256 || strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("redhonesty: invalid ref %q", truncate(ref))
	}
	for _, r := range ref {
		if r <= 0x20 || r == 0x7f {
			return "", fmt.Errorf("redhonesty: invalid ref %q", truncate(ref))
		}
	}
	out, err := gitOutput(ctx, g, root, 4<<10, "rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("redhonesty: resolve %q: %w", truncate(ref), err)
	}
	sha := strings.TrimSpace(string(out))
	if !faultline.ValidCommitSHA(sha) {
		return "", fmt.Errorf("redhonesty: %q did not resolve to a full commit SHA", truncate(ref))
	}
	return sha, nil
}

func truncate(s string) string {
	if len(s) > 64 {
		return s[:64] + "..."
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '?'
		}
		return r
	}, s)
}

// validRelPath validates a slash-separated repository-relative path: non-empty,
// not absolute, no backslashes or drive letters, and no "." / ".." elements.
func validRelPath(p string) error {
	if p == "" || len(p) > 1024 || strings.ContainsAny(p, "\\\x00:") || strings.HasPrefix(p, "/") {
		return fmt.Errorf("redhonesty: unsafe path %q", truncate(p))
	}
	if path.Clean(p) != p {
		return fmt.Errorf("redhonesty: non-canonical path %q", truncate(p))
	}
	for _, el := range strings.Split(p, "/") {
		if el == ".." || el == "." || el == "" || strings.HasSuffix(el, ".") || strings.HasSuffix(el, " ") || windowsReserved.MatchString(el) {
			return fmt.Errorf("redhonesty: unsafe path element in %q", truncate(p))
		}
	}
	return nil
}

// validArchivePath checks a baseline archive entry name for containment only:
// non-empty, relative, no backslash, NUL or ".." element. The stricter
// validRelPath (reserved device names, ':' and trailing dots) applies to paths
// the gate itself writes or runs, not to every file a repository may legally
// hold on another platform.
func validArchivePath(p string) error {
	if p == "" || len(p) > 4096 || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") || filepath.IsAbs(p) || filepath.VolumeName(p) != "" || (len(p) >= 2 && p[1] == ':') {
		return fmt.Errorf("redhonesty: unsafe archive path %q", truncate(p))
	}
	for _, el := range strings.Split(p, "/") {
		if el == ".." || el == "." || el == "" {
			return fmt.Errorf("redhonesty: unsafe archive path element in %q", truncate(p))
		}
	}
	return nil
}

func validPackageDir(p string) error {
	if !faultline.ValidPackageDir(p) {
		return fmt.Errorf("redhonesty: unsafe package %q", truncate(p))
	}
	if p == "." {
		return nil
	}
	return validRelPath(p)
}

func isOverlayPath(p string) bool {
	return strings.HasSuffix(p, "_test.go") || isTestdata(p)
}

// CollectChanges lists the changed Go sources and testdata between two commit
// SHAs with their base and head contents. Renames are reported as a delete plus
// an add. Entries are sorted by path.
func CollectChanges(ctx context.Context, g Git, root, baseSHA, headSHA string) ([]FileChange, error) {
	if !faultline.ValidCommitSHA(baseSHA) || !faultline.ValidCommitSHA(headSHA) {
		return nil, errors.New("redhonesty: CollectChanges requires full commit SHAs")
	}
	raw, err := gitOutput(ctx, g, root, maxGitOutputBytes, "diff", "--raw", "-z", "--no-renames", "--no-ext-diff", baseSHA, headSHA, "--")
	if err != nil {
		return nil, fmt.Errorf("redhonesty: list changes: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	fields := bytes.Split(bytes.TrimRight(raw, "\x00"), []byte{0})
	if len(fields)%2 != 0 {
		return nil, errors.New("redhonesty: malformed git diff output")
	}
	var out []FileChange
	var held int // cumulative blob bytes retained for this change
	for i := 0; i < len(fields); i += 2 {
		// Header: ":<oldmode> <newmode> <oldsha> <newsha> <status>", then the path.
		meta := strings.Fields(strings.TrimPrefix(string(fields[i]), ":"))
		p := string(fields[i+1])
		if len(meta) != 5 || meta[4] == "" {
			return nil, errors.New("redhonesty: malformed git diff header")
		}
		status := meta[4][:1]
		// Filter first so an unrelated file with an exotic name cannot abort the gate.
		if !isGoSource(p) && !isOverlayPath(p) {
			continue
		}
		if err := validRelPath(p); err != nil {
			return nil, err
		}
		if len(out) >= maxChangedFiles {
			return nil, fmt.Errorf("redhonesty: more than %d changed files", maxChangedFiles)
		}
		c := FileChange{Path: p, Exec: meta[1] == "100755"}
		switch status {
		case "A":
			c.Status = 'A'
		case "M", "T":
			c.Status = 'M'
		case "D":
			c.Status = 'D'
		default:
			return nil, fmt.Errorf("redhonesty: unsupported change status %q for %s", truncate(status), truncate(p))
		}
		if c.Status != 'A' {
			if c.Base, err = blob(ctx, g, root, baseSHA, p); err != nil {
				return nil, err
			}
			held += len(c.Base)
		}
		if c.Status != 'D' {
			if c.Head, err = blob(ctx, g, root, headSHA, p); err != nil {
				return nil, err
			}
			held += len(c.Head)
		}
		if held > maxChangeBytes {
			return nil, fmt.Errorf("redhonesty: changed content exceeds %d bytes", maxChangeBytes)
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func blob(ctx context.Context, g Git, root, sha, p string) ([]byte, error) {
	b, err := gitOutput(ctx, g, root, maxSourceBytes, "cat-file", "blob", sha+":"+p)
	if err != nil {
		return nil, fmt.Errorf("redhonesty: read %s at %s: %w", truncate(p), sha[:12], err)
	}
	if b == nil {
		b = []byte{}
	}
	return b, nil
}

// extractArchive extracts a tar stream into dir. Only regular files and
// directories are accepted; links, devices, absolute or escaping names and
// over-budget archives are rejected. Nothing is ever created outside dir.
func extractArchive(r io.Reader, dir string) error {
	_, err := extractArchiveCount(r, dir)
	return err
}

// extractArchiveCount is extractArchive that also reports how many regular files
// it created, so the caller can prove the export is complete.
func extractArchiveCount(r io.Reader, dir string) (int, error) {
	tr := tar.NewReader(r)
	var entries, files int
	var total int64
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return files, fmt.Errorf("redhonesty: read archive: %w", err)
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		entries++
		if entries > maxArchiveEntries {
			return files, fmt.Errorf("redhonesty: archive exceeds %d entries", maxArchiveEntries)
		}
		name := strings.TrimSuffix(h.Name, "/")
		if err := validArchivePath(name); err != nil {
			return files, err
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if rel, err := filepath.Rel(dir, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return files, fmt.Errorf("redhonesty: archive entry %q escapes the baseline directory", truncate(h.Name))
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return files, err
			}
		case tar.TypeReg:
			if h.Size < 0 || h.Size > maxArchiveFileSize {
				return files, fmt.Errorf("redhonesty: archive file %q exceeds %d bytes", truncate(h.Name), maxArchiveFileSize)
			}
			total += h.Size
			if total > maxArchiveBytes {
				return files, fmt.Errorf("redhonesty: archive exceeds %d bytes", int64(maxArchiveBytes))
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return files, err
			}
			perm := os.FileMode(0o644)
			if h.Mode&0o111 != 0 { // preserve executable bits (scripts the tests run)
				perm = 0o755
			}
			if err := writeNew(target, io.LimitReader(tr, h.Size), perm); err != nil {
				return files, err
			}
			files++
		default:
			// Links, devices and everything else are refused rather than dropped: a
			// baseline missing a linked fixture would manufacture a failure the real
			// base never had, which would be accepted as honest red evidence.
			return files, fmt.Errorf("redhonesty: archive entry %q has unsupported type %q; the baseline cannot be reproduced faithfully", truncate(h.Name), string(h.Typeflag))
		}
	}
}

func writeNew(target string, r io.Reader, perm os.FileMode) error {
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// treeBlobCount returns how many files (blobs) commit sha contains. It is the
// ground truth the exported baseline is compared with, because git archive
// silently honours export-ignore attributes and would otherwise drop files.
func treeBlobCount(ctx context.Context, g Git, root, sha string) (int, error) {
	out, err := gitOutput(ctx, g, root, maxTreeListBytes, "ls-tree", "-r", "-z", "--full-tree", sha)
	if err != nil {
		return 0, fmt.Errorf("redhonesty: list base tree: %w", err)
	}
	n := 0
	for _, rec := range bytes.Split(bytes.TrimRight(out, "\x00"), []byte{0}) {
		meta, _, ok := bytes.Cut(rec, []byte{'\t'})
		if ok && bytes.Contains(meta, []byte(" blob ")) {
			n++
		}
	}
	return n, nil
}

// applyOverlay makes the baseline's TEST state equal head's: head test files
// and testdata are written over the baseline tree and test files/testdata that
// head deleted (including the old half of a moved test) are removed. Production
// changes are never applied (that would erase the baseline). Every path is
// validated, including skipped ones.
func applyOverlay(dir string, changes []FileChange) error {
	for _, c := range changes {
		if err := validRelPath(c.Path); err != nil {
			return err
		}
	}
	for _, c := range changes {
		if !isOverlayPath(c.Path) {
			continue
		}
		target := filepath.Join(dir, filepath.FromSlash(c.Path))
		if c.Status == 'D' {
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		if c.Head == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if fi, err := os.Lstat(target); err == nil && !fi.Mode().IsRegular() {
			return fmt.Errorf("redhonesty: overlay target %q is not a regular file", truncate(c.Path))
		}
		perm := os.FileMode(0o644)
		if c.Exec {
			perm = 0o755
		}
		if err := os.WriteFile(target, c.Head, perm); err != nil {
			return err
		}
		// WriteFile keeps an existing file's mode; apply head's explicitly so
		// mode-only changes and replaced files match head's test state.
		if err := os.Chmod(target, perm); err != nil {
			return err
		}
	}
	return nil
}

// within reports whether path is root or lies inside it (both already resolved).
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func noSymlinkChain(root string, rel []string) error {
	cur := root
	for _, el := range rel {
		cur = filepath.Join(cur, el)
		fi, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			return fmt.Errorf("redhonesty: %s must be a real directory", filepath.Join(rel...))
		}
	}
	return nil
}

// PrepareBaseline materializes the immutable baseSHA tree plus the head test
// overlay in a fresh directory under <root>/.backlogit/runtime/redhonesty/.
// The location must be git-ignored (verified before anything is created) and
// contain no symlink component. cleanup removes the directory.
func PrepareBaseline(ctx context.Context, g Git, root, baseSHA string, overlay []FileChange) (string, func() error, error) {
	if !faultline.ValidCommitSHA(baseSHA) {
		return "", nil, errors.New("redhonesty: baseline requires a full immutable commit SHA")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	if absRoot, err = filepath.EvalSymlinks(absRoot); err != nil {
		return "", nil, err
	}
	if err := noSymlinkChain(absRoot, baselineRel); err != nil {
		return "", nil, err
	}
	probe := strings.Join(baselineRel, "/") + "/baseline-probe/x"
	if err := g.Run(ctx, absRoot, io.Discard, "check-ignore", "-q", "--no-index", "--", probe); err != nil {
		return "", nil, fmt.Errorf("redhonesty: baseline location %s is not git-ignored: %w", strings.Join(baselineRel, "/"), err)
	}
	parent := filepath.Join(append([]string{absRoot}, baselineRel...)...)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, err
	}
	if err := noSymlinkChain(absRoot, baselineRel); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(parent, "base-")
	if err != nil {
		return "", nil, err
	}
	// Re-verify containment after creation: the resolved directory must still be
	// inside the resolved workspace root even if a path component was swapped.
	if real, rerr := filepath.EvalSymlinks(dir); rerr != nil || !within(absRoot, real) {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("redhonesty: baseline directory escaped the workspace root")
	}
	cleanup := func() error {
		if rel, err := filepath.Rel(parent, dir); err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			return fmt.Errorf("redhonesty: refusing to remove %q outside the baseline root", dir)
		}
		return os.RemoveAll(dir)
	}
	fail := func(err error) (string, func() error, error) {
		_ = cleanup()
		return "", nil, err
	}

	pr, pw := io.Pipe()
	gitErr := make(chan error, 1)
	go func() {
		// Archive the peeled TREE, not the commit: export-subst only expands
		// $Format:...$ placeholders when archiving a commit, so this keeps every
		// file byte-identical to what is committed (export-ignore is covered by
		// the blob-count proof below).
		err := g.Run(ctx, absRoot, pw, "archive", "--format=tar", baseSHA+"^{tree}")
		_ = pw.CloseWithError(err)
		gitErr <- err
	}()
	extracted, extractErr := extractArchiveCount(pr, dir)
	if extractErr == nil {
		// The tar reader stops at the end-of-archive marker; drain git's trailing
		// padding so git exits cleanly instead of failing on a closed pipe.
		_, extractErr = io.Copy(io.Discard, pr)
	}
	_ = pr.CloseWithError(errors.New("redhonesty: extraction finished"))
	if err := <-gitErr; err != nil && extractErr == nil {
		extractErr = fmt.Errorf("redhonesty: git archive: %w", err)
	}
	if extractErr != nil {
		return fail(extractErr)
	}
	// git archive honours export-ignore attributes and would silently omit files;
	// prove the export holds every blob of the base commit before trusting it.
	want, err := treeBlobCount(ctx, g, absRoot, baseSHA)
	if err != nil {
		return fail(err)
	}
	if extracted != want {
		return fail(fmt.Errorf("redhonesty: export of %s holds %d files but the commit has %d (export-ignore attributes?); the baseline would be incomplete", baseSHA[:12], extracted, want))
	}
	if err := applyOverlay(dir, overlay); err != nil {
		return fail(err)
	}
	return dir, cleanup, nil
}
