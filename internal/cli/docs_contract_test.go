package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocsLintCommand_ContractText_DecodeErrorAndRetainedExit is scenario 3
// of 146.024-T (U9c): the CONSTRUCTED docs lint Cobra Short, Long, and
// Example strings must name the decode_error rule value and state that the
// non-zero exit is retained on a corpus containing one. Assertions read the
// constructed command, never a package-level literal, so text that is
// written but never wired into the command cannot produce a false green.
// This is 146.018-T (U8b)'s own red harness, authored and observed failing
// before U8b, exactly as U3d (146.010-T) is the red harness for U4b's tool
// description.
func TestDocsLintCommand_ContractText_DecodeErrorAndRetainedExit(t *testing.T) {
	var cwd string
	cmd := newDocsLintCommand(&cwd)

	combined := strings.ToLower(cmd.Short + "\n" + cmd.Long + "\n" + cmd.Example)

	assert.Contains(t, combined, "decode_error", "the constructed docs lint command must name the decode_error rule value")
	require.NotEmpty(t, cmd.Long, "146.018-T (U8b) must add a Long description")
	require.NotEmpty(t, cmd.Example, "146.018-T (U8b) must add an Example")
	assert.True(t,
		strings.Contains(combined, "non-zero") || strings.Contains(combined, "nonzero"),
		"the constructed command must state that the non-zero exit is retained on a corpus containing a decode_error",
	)
}
