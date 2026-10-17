package redhonesty

import (
	"reflect"
	"testing"
)

func set(names ...string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, n := range names {
		m[n] = struct{}{}
	}
	return m
}

func syms(funcs, types, fields, siblings []string) Symbols {
	return Symbols{Funcs: set(funcs...), Types: set(types...), Fields: set(fields...), Siblings: set(siblings...)}
}

// evalOne runs Evaluate for one test with the given source and results.
func evalOne(t *testing.T, src string, base, head Result, s Symbols) Verdict {
	t.Helper()
	nt := NewTest{Package: "p", File: "p/p_test.go", Name: "TestX"}
	vs := Evaluate(EvalInput{
		Tests:    []NewTest{nt},
		Sources:  map[string][]byte{nt.File: []byte(src)},
		Baseline: map[string]Result{nt.Key(): base},
		Head:     map[string]Result{nt.Key(): head},
		Symbols:  s,
	})
	if len(vs) != 1 {
		t.Fatalf("verdicts = %d", len(vs))
	}
	return vs[0]
}

func wantRejections(t *testing.T, v Verdict, want ...RejectionClass) {
	t.Helper()
	if want == nil {
		want = []RejectionClass{}
	}
	got := append([]RejectionClass{}, v.Rejections...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rejections = %v, want %v", got, want)
	}
	if v.Honest() != (len(want) == 0) {
		t.Fatalf("Honest() = %v with rejections %v", v.Honest(), got)
	}
}

const genuineSrc = `package p
import "testing"
func TestX(t *testing.T) {
	got := Compute(2)
	if got != 3 { t.Fatalf("got %d", got) }
}
`

func TestU161_002_GenuineReproducerAccepted(t *testing.T) {
	v := evalOne(t, genuineSrc, ResultFail, ResultPass, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v)
}

func TestU161_002_RejectsAlreadyPassingAtBaseline(t *testing.T) {
	v := evalOne(t, genuineSrc, ResultPass, ResultPass, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v, RejectAlreadyPasses)
}

func TestU161_002_RejectsTestNotExercisingChangedCode(t *testing.T) {
	src := `package p
import "testing"
func TestX(t *testing.T) {
	if Unrelated() != 1 { t.Fatal("x") }
}
`
	v := evalOne(t, src, ResultFail, ResultPass, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v, RejectNoChangedBranch)
}

func TestU161_002_TransitiveSameFileHelperCountsAsExercising(t *testing.T) {
	src := `package p
import "testing"
func check(t *testing.T) {
	if Compute(1) != 2 { t.Fatal("x") }
}
func TestX(t *testing.T) { check(t) }
`
	v := evalOne(t, src, ResultFail, ResultPass, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v)
}

func TestU161_002_RejectsSiblingFieldOnlyAssertions(t *testing.T) {
	sibling := `package p
import "testing"
func TestX(t *testing.T) {
	r := Build()
	if r.A != 1 || r.B != "b" { t.Fatal("x") }
}
`
	s := syms([]string{"Build"}, []string{"Row"}, []string{"C"}, []string{"A", "B"})
	wantRejections(t, evalOne(t, sibling, ResultFail, ResultPass, s), RejectSiblingOnly)

	genuine := `package p
import "testing"
func TestX(t *testing.T) {
	r := Build()
	if r.A != 1 || r.C != true { t.Fatal("x") }
}
`
	wantRejections(t, evalOne(t, genuine, ResultFail, ResultPass, s))
}

func TestU161_002_RejectsExpectedOutputViaChangedPath(t *testing.T) {
	buggy := `package p
import "testing"
func TestX(t *testing.T) {
	want := Compute(2)
	got := Compute(2)
	if got != want { t.Fatal("x") }
}
`
	s := syms([]string{"Compute"}, nil, nil, nil)
	wantRejections(t, evalOne(t, buggy, ResultFail, ResultPass, s), RejectBuggyPathExpected)

	viaVar := `package p
import "testing"
func TestX(t *testing.T) {
	var expected = Compute(2)
	if Compute(2) != expected { t.Fatal("x") }
}
`
	wantRejections(t, evalOne(t, viaVar, ResultFail, ResultPass, s), RejectBuggyPathExpected)

	honest := `package p
import "testing"
func TestX(t *testing.T) {
	want := 3
	if Compute(2) != want { t.Fatal("x") }
}
`
	wantRejections(t, evalOne(t, honest, ResultFail, ResultPass, s))
}

func TestU161_002_FailsClosedWhenBaselineUnverifiable(t *testing.T) {
	v := evalOne(t, genuineSrc, ResultUnrunnable, ResultPass, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v, RejectUnverifiableBaseline)
	// An absent baseline observation is unverifiable too, never a silent pass.
	nt := NewTest{Package: "p", File: "p/p_test.go", Name: "TestX"}
	vs := Evaluate(EvalInput{Tests: []NewTest{nt}, Sources: map[string][]byte{nt.File: []byte(genuineSrc)},
		Head: map[string]Result{nt.Key(): ResultPass}, Symbols: syms([]string{"Compute"}, nil, nil, nil)})
	wantRejections(t, vs[0], RejectUnverifiableBaseline)
}

func TestU161_002_RejectsHeadNotGreen(t *testing.T) {
	v := evalOne(t, genuineSrc, ResultFail, ResultFail, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v, RejectHeadNotGreen)
}

func TestU161_002_UnanalyzableSourceIsRejected(t *testing.T) {
	nt := NewTest{Package: "p", File: "p/p_test.go", Name: "TestX"}
	s := syms([]string{"Compute"}, nil, nil, nil)
	for name, srcs := range map[string]map[string][]byte{
		"missing source":  {},
		"unparseable":     {nt.File: []byte("package p\nfunc TestX(")},
		"function absent": {nt.File: []byte("package p\nfunc TestOther(t *testing.T) {}\n")},
	} {
		vs := Evaluate(EvalInput{Tests: []NewTest{nt}, Sources: srcs,
			Baseline: map[string]Result{nt.Key(): ResultFail}, Head: map[string]Result{nt.Key(): ResultPass}, Symbols: s})
		if len(vs) != 1 || vs[0].Honest() {
			t.Errorf("%s: verdict = %+v", name, vs)
			continue
		}
		found := false
		for _, r := range vs[0].Rejections {
			found = found || r == RejectUnanalyzable
		}
		if !found {
			t.Errorf("%s: rejections = %v, want %s", name, vs[0].Rejections, RejectUnanalyzable)
		}
	}
}

func TestU161_002_EmptyProductionDeltaSkipsStructuralRulesButKeepsBaselineRule(t *testing.T) {
	// Fault-injected baseline / no production delta: structural rules cannot
	// apply, but a test that already passes is still rejected.
	src := `package p
import "testing"
func TestX(t *testing.T) { if Unrelated() != 1 { t.Fatal("x") } }
`
	wantRejections(t, evalOne(t, src, ResultFail, ResultPass, Symbols{}))
	wantRejections(t, evalOne(t, src, ResultPass, ResultPass, Symbols{}), RejectAlreadyPasses)
}

func TestU161_002_MultipleRejectionsAreDeterministicallyOrdered(t *testing.T) {
	src := `package p
import "testing"
func TestX(t *testing.T) {
	want := Compute(2)
	if Compute(2) != want { t.Fatal("x") }
}
`
	v := evalOne(t, src, ResultPass, ResultFail, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v, RejectAlreadyPasses, RejectBuggyPathExpected, RejectHeadNotGreen)
}

func TestU161_002_VerdictsFollowInputOrder(t *testing.T) {
	a := NewTest{Package: "p", File: "p/p_test.go", Name: "TestA"}
	b := NewTest{Package: "p", File: "p/p_test.go", Name: "TestB"}
	src := "package p\nimport \"testing\"\nfunc TestA(t *testing.T){ Compute(1) }\nfunc TestB(t *testing.T){ Compute(1) }\n"
	vs := Evaluate(EvalInput{Tests: []NewTest{b, a}, Sources: map[string][]byte{a.File: []byte(src)},
		Baseline: map[string]Result{a.Key(): ResultFail, b.Key(): ResultFail},
		Head:     map[string]Result{a.Key(): ResultPass, b.Key(): ResultPass},
		Symbols:  syms([]string{"Compute"}, nil, nil, nil)})
	if len(vs) != 2 || vs[0].Test.Name != "TestB" || vs[1].Test.Name != "TestA" {
		t.Fatalf("order = %+v", vs)
	}
}
