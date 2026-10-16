package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU160003_ImpossibleTest_FlagsContradictoryEquals(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Tests = []TestSpec{{Name: "TestX", Assertions: []TestAssertion{
		{Subject: "status", Op: OpEqual, Value: "done"},
		{Subject: "status", Op: OpEqual, Value: "queued"},
	}}}
	got := findingsFor(t, "impossible-test", u)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "TestX")
}

func TestU160003_ImpossibleTest_FlagsEqualAndNotEqual(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Tests = []TestSpec{{Name: "TestY", Assertions: []TestAssertion{
		{Subject: "n", Op: OpEqual, Value: "1"},
		{Subject: "n", Op: OpNotEqual, Value: "1"},
	}}}
	assert.Len(t, findingsFor(t, "impossible-test", u), 1)
}

func TestU160003_ImpossibleTest_UnknownOperatorFailsClosed(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Tests = []TestSpec{{Name: "TestZ", Assertions: []TestAssertion{{Subject: "n", Op: "~", Value: "1"}}}}
	assert.Len(t, findingsFor(t, "impossible-test", u), 1)
}

func TestU160003_ImpossibleTest_SatisfiableTestsPass(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Tests = []TestSpec{
		{Name: "TestOk", Assertions: []TestAssertion{
			{Subject: "status", Op: OpEqual, Value: "done"},
			{Subject: "status", Op: OpEqual, Value: "done"},
			{Subject: "status", Op: OpNotEqual, Value: "queued"},
			{Subject: "other", Op: OpEqual, Value: "x"},
		}},
		{Name: "TestOther", Assertions: []TestAssertion{{Subject: "status", Op: OpEqual, Value: "queued"}}},
	}
	assert.Empty(t, findingsFor(t, "impossible-test", u))
}

func TestCopilotR1_ImpossibleTest_CaseAndWhitespaceSensitive(t *testing.T) {
	cases := []struct {
		name    string
		a, b    TestAssertion
		flagged bool
	}{
		{"case differs eq/eq", TestAssertion{Subject: "status", Op: OpEqual, Value: "READY"}, TestAssertion{Subject: "status", Op: OpEqual, Value: "ready"}, true},
		{"case differs eq/ne", TestAssertion{Subject: "status", Op: OpEqual, Value: "READY"}, TestAssertion{Subject: "status", Op: OpNotEqual, Value: "ready"}, false},
		{"inner whitespace eq/ne", TestAssertion{Subject: "s", Op: OpEqual, Value: "a  b"}, TestAssertion{Subject: "s", Op: OpNotEqual, Value: "a b"}, false},
		{"same literal eq/ne", TestAssertion{Subject: "s", Op: OpEqual, Value: "x"}, TestAssertion{Subject: "s", Op: OpNotEqual, Value: "x"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := soundUnit("1-T", "A")
			u.Tests = []TestSpec{{Name: "T", Assertions: []TestAssertion{tc.a, tc.b}}}
			got := findingsFor(t, "impossible-test", u)
			if tc.flagged {
				assert.Len(t, got, 1)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}
