package redhonesty

import "testing"

func TestU161_002_ExpectedValueViaSameFileHelperIsCaught(t *testing.T) {
	src := `package p
import "testing"
func expectedFor(n int) int { return Compute(n) }
func wrap(n int) int { return expectedFor(n) }
func TestX(t *testing.T) {
	want := wrap(2)
	if Compute(2) != want { t.Fatal("x") }
}
`
	s := syms([]string{"Compute"}, nil, nil, nil)
	wantRejections(t, evalOne(t, src, ResultFail, ResultPass, s), RejectBuggyPathExpected)

	unrelated := `package p
import "testing"
func literal(n int) int { return n * 3 }
func TestX(t *testing.T) {
	want := literal(1)
	if Compute(2) != want { t.Fatal("x") }
}
`
	wantRejections(t, evalOne(t, unrelated, ResultFail, ResultPass, s))
}
