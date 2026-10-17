package redhonesty

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestU161_002_HelperChainDepthIsDeterministic(t *testing.T) {
	// h1 calls Compute; h2 calls h1; ... A chain longer than maxHelperDepth is
	// cut at the bound, and the cut must not depend on map iteration order.
	build := func(chain int) string {
		var b strings.Builder
		b.WriteString("package p\nimport \"testing\"\nfunc h1(n int) int { return Compute(n) }\n")
		for i := 2; i <= chain; i++ {
			fmt.Fprintf(&b, "func h%d(n int) int { return h%d(n) }\n", i, i-1)
		}
		fmt.Fprintf(&b, "func TestX(t *testing.T) {\n\twant := h%d(2)\n\tif Compute(2) != want { t.Fatal(\"x\") }\n}\n", chain)
		return b.String()
	}
	s := syms([]string{"Compute"}, nil, nil, nil)
	for i := 0; i < 40; i++ { // many runs: map order must not matter
		within := evalOne(t, build(maxHelperDepth), ResultFail, ResultPass, s)
		wantRejections(t, within, RejectBuggyPathExpected)
		beyond := evalOne(t, build(maxHelperDepth+2), ResultFail, ResultPass, s)
		wantRejections(t, beyond) // documented bound: deeper chains are not followed
	}
}

func TestU161_001_PrepareBaseline_DoesNotExpandExportSubst(t *testing.T) {
	root := newRepo(t, true)
	writeFile(t, root, "p/version.go", "package p\n\nconst Subject = \"$Format:%s$\"\n")
	writeFile(t, root, ".gitattributes", "p/version.go export-subst\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "first subject")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	dir, cleanup, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	b, err := os.ReadFile(filepath.Join(dir, "p", "version.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "$Format:%s$") || strings.Contains(string(b), "first subject") {
		t.Fatalf("export-subst expanded the committed content: %q", b)
	}
}

func TestU161_001_PrepareBaseline_RefusesIncompleteExport(t *testing.T) {
	root := newRepo(t, true)
	writeFile(t, root, "p/secret_fixture.txt", "fixture")
	writeFile(t, root, ".gitattributes", "p/secret_fixture.txt export-ignore\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "attrs")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	if _, _, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, nil); err == nil {
		t.Fatal("an export that silently dropped an export-ignore file was accepted as the baseline")
	}
}
