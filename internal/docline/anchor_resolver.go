package docline

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/softwaresalt/backlogit/internal/core"
)

// Sentinel errors reported by the anchor resolver. ErrAnchorOutsideRoot also
// wraps ErrPathEscapesWorkspace so callers can match either.
var (
	// ErrAnchorSyntax marks a string that is not a file:line or file::symbol anchor.
	ErrAnchorSyntax = errors.New("docline: malformed anchor")
	// ErrAnchorOutsideRoot marks an anchor that resolves outside the workspace root.
	ErrAnchorOutsideRoot = errors.New("docline: anchor resolves outside workspace root")
	// ErrAnchorNotFound marks an anchor whose file does not exist.
	ErrAnchorNotFound = errors.New("docline: anchor file not found")
	// ErrAnchorLineOutOfRange marks an anchor line beyond the end of its file.
	ErrAnchorLineOutOfRange = errors.New("docline: anchor line out of range")
	// ErrAnchorSymbolNotFound marks an anchor symbol absent from its file.
	ErrAnchorSymbolNotFound = errors.New("docline: anchor symbol not found")
)

// maxAnchorFileBytes bounds how much of a referenced file the resolver will read.
const maxAnchorFileBytes = 8 << 20

// Anchor is a parsed reference to a file location or symbol. Exactly one of
// Line or Symbol is set.
type Anchor struct {
	Path    string // workspace-relative path as written in the document
	Line    int    // 1-based start line (0 when Symbol is used)
	EndLine int    // optional 1-based inclusive end line (0 = single line)
	Symbol  string // optional symbol name; Type.Method matches on the last segment
}

// Resolved is a verified anchor target inside the workspace root.
type Resolved struct {
	Abs  string // absolute path
	Real string // symlink-resolved path actually read; always inside the root
	Rel  string // repo-relative POSIX path
}

var lineSpec = regexp.MustCompile(`^(\d+)(?:-(\d+))?$`)

// ParseAnchor parses path:line, path:start-end or path::Symbol. It performs
// no filesystem access.
func ParseAnchor(s string) (Anchor, error) {
	if i := strings.Index(s, "::"); i >= 0 {
		path, sym := s[:i], s[i+2:]
		if path == "" || sym == "" {
			return Anchor{}, fmt.Errorf("%w: %q", ErrAnchorSyntax, s)
		}
		return Anchor{Path: path, Symbol: sym}, nil
	}
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return Anchor{}, fmt.Errorf("%w: %q", ErrAnchorSyntax, s)
	}
	m := lineSpec.FindStringSubmatch(s[i+1:])
	if m == nil {
		return Anchor{}, fmt.Errorf("%w: %q", ErrAnchorSyntax, s)
	}
	start, _ := strconv.Atoi(m[1])
	end := 0
	if m[2] != "" {
		end, _ = strconv.Atoi(m[2])
	}
	if start < 1 || (m[2] != "" && end < start) {
		return Anchor{}, fmt.Errorf("%w: invalid line range in %q", ErrAnchorSyntax, s)
	}
	return Anchor{Path: s[:i], Line: start, EndLine: end}, nil
}

// ResolveWorkspacePath resolves rel to an existing path confined to the
// workspace root. It rejects absolute paths, drive-qualified paths, ".."
// traversal and symlinks whose real target leaves the root, wrapping
// ErrAnchorOutsideRoot; a missing target wraps ErrAnchorNotFound. The returned
// Resolved carries the lexical absolute path and the symlink-resolved Real path, both verified.
func ResolveWorkspacePath(root, rel string) (Resolved, error) {
	outside := func() (Resolved, error) {
		return Resolved{}, fmt.Errorf("%w: %w: %q", ErrAnchorOutsideRoot, ErrPathEscapesWorkspace, rel)
	}
	if rel == "" || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" ||
		strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") {
		return outside()
	}
	abs, err := core.SafeResolve(root, rel)
	if err != nil {
		return outside()
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Resolved{}, fmt.Errorf("docline.ResolveWorkspacePath: resolve root: %w", err)
	}
	realTarget, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Resolved{}, fmt.Errorf("%w: %q", ErrAnchorNotFound, rel)
		}
		return Resolved{}, fmt.Errorf("docline.ResolveWorkspacePath: %q: %w", rel, err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return Resolved{}, fmt.Errorf("docline.ResolveWorkspacePath: resolve root: %w", err)
	}
	within, err := filepath.Rel(realRoot, realTarget)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) || filepath.IsAbs(within) {
		return outside()
	}
	relOut, err := filepath.Rel(absRoot, abs)
	if err != nil {
		return outside()
	}
	return Resolved{Abs: abs, Real: realTarget, Rel: filepath.ToSlash(relOut)}, nil
}

// ResolveAnchor verifies that a resolves inside root: the file exists, any
// line range lies within it and any symbol occurs in it.
func ResolveAnchor(root string, a Anchor) (Resolved, error) {
	r, _, err := loadAnchor(root, a)
	return r, err
}

// ResolveAnchorString parses and resolves s in one step.
func ResolveAnchorString(root, s string) (Resolved, error) {
	a, err := ParseAnchor(s)
	if err != nil {
		// An absolute or drive-qualified path with no line/symbol is still an
		// escape attempt worth classifying, but syntax errors take precedence.
		return Resolved{}, err
	}
	return ResolveAnchor(root, a)
}

// afterAnchorResolveHook is a test seam invoked between path resolution and open.
var afterAnchorResolveHook func(real string)

// openConfined opens real through a root-confined handle on the workspace root,
// so a symlink swapped in after resolution cannot lead outside the root.
func openConfined(root, real string) (*os.File, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("docline.openConfined: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("docline.openConfined: %w", err)
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil {
		return nil, fmt.Errorf("docline.openConfined: %w", err)
	}
	rt, err := os.OpenRoot(realRoot)
	if err != nil {
		return nil, fmt.Errorf("docline.openConfined: %w", err)
	}
	defer func() { _ = rt.Close() }()
	f, err := rt.Open(rel)
	if err != nil {
		return nil, fmt.Errorf("docline.openConfined: %w", err)
	}
	return f, nil
}

// loadAnchor resolves a and returns the verified file content.
func loadAnchor(root string, a Anchor) (Resolved, string, error) {
	r, err := ResolveWorkspacePath(root, a.Path)
	if err != nil {
		return Resolved{}, "", err
	}
	if afterAnchorResolveHook != nil {
		afterAnchorResolveHook(r.Real)
	}
	f, err := openConfined(root, r.Real)
	if err != nil {
		return Resolved{}, "", fmt.Errorf("%w: %q", ErrAnchorNotFound, a.Path)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Resolved{}, "", fmt.Errorf("%w: %q", ErrAnchorNotFound, a.Path)
	}
	if info.Size() > maxAnchorFileBytes {
		return Resolved{}, "", fmt.Errorf("%w: %q exceeds %d bytes", ErrAnchorNotFound, a.Path, maxAnchorFileBytes)
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxAnchorFileBytes+1))
	if err != nil {
		return Resolved{}, "", fmt.Errorf("docline.loadAnchor: read %q: %w", a.Path, err)
	}
	if len(raw) > maxAnchorFileBytes {
		return Resolved{}, "", fmt.Errorf("%w: %q exceeds %d bytes", ErrAnchorNotFound, a.Path, maxAnchorFileBytes)
	}
	content := string(raw)
	if a.Line > 0 {
		lines := strings.Count(content, "\n")
		if content != "" && !strings.HasSuffix(content, "\n") {
			lines++
		}
		last := a.Line
		if a.EndLine > last {
			last = a.EndLine
		}
		if last > lines {
			return Resolved{}, "", fmt.Errorf("%w: %q has %d lines, anchor wants %d", ErrAnchorLineOutOfRange, a.Path, lines, last)
		}
	}
	if a.Symbol != "" && !symbolPattern(a.Symbol).MatchString(content) {
		return Resolved{}, "", fmt.Errorf("%w: %q in %q", ErrAnchorSymbolNotFound, a.Symbol, a.Path)
	}
	return r, content, nil
}

// symbolPattern matches the last dotted segment of sym as a whole word.
func symbolPattern(sym string) *regexp.Regexp {
	if i := strings.LastIndex(sym, "."); i >= 0 && i < len(sym)-1 {
		sym = sym[i+1:]
	}
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(sym) + `\b`)
}
