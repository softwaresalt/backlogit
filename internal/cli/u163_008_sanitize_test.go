package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/policyengine"
)

func TestU163_008_PrintPolicyTextStripsControlCharacters(t *testing.T) {
	d := policyengine.Decision{
		TaskID: "001-T", HeadCommit: "0123456789abcdef", Mode: "enforce", Surface: "cli",
		Disposition: "block", Reason: "bad\x1b[31mred\x07",
		Errors:          []policyengine.Violation{{Code: "c", Node: "n", Reason: "evil\x1b]0;title\x07"}},
		OverrideDenials: []policyengine.OverrideDenial{{Index: 1, Reason: "deny\x1b[2J"}},
	}
	var buf bytes.Buffer
	printPolicyText(&buf, d)
	out := buf.String()
	assert.NotContains(t, out, "\x1b")
	assert.NotContains(t, out, "\x07")
	assert.Contains(t, out, "evil")
}

func TestU163_008_SafeTextStripsEscapes(t *testing.T) {
	assert.NotContains(t, safeText("a\u009b31mb"), "\u009b")
	assert.NotContains(t, safeText("a\x1b[31mb\x00c\x7f"), "\x1b")
	assert.NotContains(t, safeText("a\x1b[31mb\x00c\x7f"), "\x7f")
}
