package soundness

import (
	"errors"
	"testing"
)

func TestReferenceResolutionCachedPerRun(t *testing.T) {
	const n = 1000
	units := make([]Unit, 0, n)
	for i := 0; i < n; i++ {
		units = append(units, Unit{
			ID: "T" + string(rune('A'+i%26)) + itoa(i), Kind: KindTask,
			References: []string{"a.md::X"}, Requires: []string{"a.md::X"},
		})
	}
	cfg := DefaultConfig()
	cfg.Root = t.TempDir()
	plan := Plan{Units: units}

	run := func() (*checkContext, int) {
		ctx := newCheckContext(plan, cfg)
		ctx.refResolver = func(string, string) error {
			return errors.New("boom")
		}
		refs, err := checkReferences(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := checkMissingArtifact(ctx); err != nil {
			t.Fatal(err)
		}
		return ctx, len(refs)
	}

	ctx1, findings := run()
	if findings != n {
		t.Fatalf("want %d per-unit findings, got %d", n, findings)
	}
	if ctx1.refCalls != 1 {
		t.Fatalf("first run resolved %d times, want 1", ctx1.refCalls)
	}
	ctx2, _ := run()
	if ctx2.refCalls != 1 {
		t.Fatalf("second run resolved %d times, want 1 (fresh cache)", ctx2.refCalls)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}
