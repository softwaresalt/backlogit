package tests_test

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	u175FrozenList   = "docs/memory/2026-10-15/175-001-frozen-path-list.txt"
	u175MaxOffenders = 20
)

type u175Entry struct {
	index string
	work  string
	attr  string
}

func u175RepoRoot(t *testing.T) string {
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

func u175ParseEol(t *testing.T, out []byte) map[string]u175Entry {
	t.Helper()
	entries := map[string]u175Entry{}
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		meta, path, ok := strings.Cut(string(rec), "\t")
		if !ok {
			t.Fatalf("malformed git ls-files --eol record: %q", rec)
		}
		var e u175Entry
		for _, f := range strings.Fields(meta) {
			switch {
			case strings.HasPrefix(f, "i/"):
				e.index = strings.TrimPrefix(f, "i/")
			case strings.HasPrefix(f, "w/"):
				e.work = strings.TrimPrefix(f, "w/")
			case strings.HasPrefix(f, "attr/"):
				e.attr = strings.TrimPrefix(f, "attr/")
			}
		}
		entries[strings.TrimSpace(path)] = e
	}
	return entries
}

func u175Report(t *testing.T, label string, offenders []string) {
	t.Helper()
	if len(offenders) == 0 {
		return
	}
	shown := offenders
	if len(shown) > u175MaxOffenders {
		shown = shown[:u175MaxOffenders]
	}
	t.Errorf("%s: %d offending path(s); first %d:\n  %s", label, len(offenders), len(shown), strings.Join(shown, "\n  "))
}

func TestU175_001_BaselineEol(t *testing.T) {
	root := u175RepoRoot(t)

	cmd := exec.Command("git", "ls-files", "-z", "--eol", "--", "*.go", "*.json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files --eol: %v", err)
	}
	all := u175ParseEol(t, out)
	if len(all) == 0 {
		t.Fatal("git ls-files --eol returned no *.go/*.json paths")
	}

	var indexCRLF, workCRLF, workMixed []string
	checked, excluded := 0, 0
	for path, e := range all {
		if strings.Contains(e.attr, "-text") {
			excluded++
			t.Logf("excluded (declared binary, attr/%s): %s", e.attr, path)
			continue
		}
		checked++
		if e.index != "lf" && e.index != "none" {
			indexCRLF = append(indexCRLF, path+" (i/"+e.index+")")
		}
		switch e.work {
		case "lf", "none":
		case "mixed":
			workMixed = append(workMixed, path)
		default:
			workCRLF = append(workCRLF, path+" (w/"+e.work+")")
		}
	}
	t.Logf("checked=%d excluded=%d index-not-lf=%d work-not-lf=%d w/mixed=%d",
		checked, excluded, len(indexCRLF), len(workCRLF), len(workMixed))

	u175Report(t, "index blobs must be i/lf", indexCRLF)
	u175Report(t, "working tree must be w/lf", workCRLF)
	u175Report(t, "w/mixed paths require operator decision", workMixed)

	f, err := os.Open(filepath.Join(root, filepath.FromSlash(u175FrozenList)))
	if err != nil {
		t.Fatalf("open frozen path list: %v", err)
	}
	defer f.Close()

	var badExt, notLF []string
	listed := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		listed++
		if !strings.HasSuffix(line, ".go") && !strings.HasSuffix(line, ".json") {
			badExt = append(badExt, line)
		}
		e, ok := all[line]
		if !ok {
			notLF = append(notLF, line+" (not tracked)")
		} else if e.index != "lf" || e.work != "lf" {
			notLF = append(notLF, line+" (i/"+e.index+" w/"+e.work+")")
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read frozen path list: %v", err)
	}
	if listed == 0 {
		t.Fatal("frozen path list contains no paths")
	}
	u175Report(t, "frozen list paths must end in .go or .json", badExt)
	u175Report(t, "frozen list paths must be i/lf and w/lf", notLF)
}
