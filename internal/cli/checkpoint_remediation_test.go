package cli_test

// U16 — CLI-boundary remediation command rendering (147-F / 147.039-T). The
// CLI is the only surface allowed to render an operator-runnable disposition
// command, because it is the only layer that knows the resolved workspace.
// No cross-shell paste-safety claim is made; the approval step is a human
// step by construction.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestU16_RenderedBlockCarriesCwdFilenameAndMandatoryLines asserts a
// rendered block for a valid-but-non-conforming target contains an absolute
// --cwd bound to the resolved workspace, the bare filename (never a path or
// concatenation) rendered on the intent's own Verb subcommand, and all
// three of the A4c approval, preimage, and no-clobber destination lines.
func TestU16_RenderedBlockCarriesCwdFilenameAndMandatoryLines(t *testing.T) {
	workspaceRoot := t.TempDir()
	intent := &events.RemediationIntent{
		Verb:             "quarantine",
		TargetFilename:   "checkpoint-20260824-100000.json",
		RequiresApproval: true,
		ApprovalClass:    "A4c",
		Reason:           "non_conforming",
	}

	var buf bytes.Buffer
	cli.RenderCheckpointRemediationBlock(&buf, intent, workspaceRoot)
	out := buf.String()

	require.NotEmpty(t, out)
	assert.Contains(t, out, "--cwd "+workspaceRoot, "must carry an explicit --cwd bound to the resolved workspace")
	assert.Contains(t, out, intent.TargetFilename, "must name the bare filename")
	// The bare filename must appear as the command argument, never
	// concatenated into a larger path segment immediately before it.
	assert.NotContains(t, out, "/"+intent.TargetFilename+intent.TargetFilename)
	assert.Contains(t, out, "A4c", "must carry the A4c approval line")
	assert.Contains(t, out, "REQUIRED", "must carry an explicit approval-required line")
	assert.Contains(t, out, "byte copy", "must carry the preimage line")
	assert.Contains(t, out, "no-clobber", "must carry the no-clobber destination line")
	assert.Contains(t, out, "ABSENT", "the no-clobber line must state the destination must be absent")

	commandLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "backlogit --cwd") {
			commandLine = line
			break
		}
	}
	require.NotEmpty(t, commandLine, "must render a command line")
	assert.Contains(t, commandLine, "checkpoint "+intent.Verb,
		"the command must use the intent's own Verb as the checkpoint subcommand, not a hardcoded one")
}

// TestU16_ShellSpecialCharacterSuppressesCommandLine asserts a target whose
// filename contains a shell metacharacter renders the block with the
// "command not rendered" line and no command line at all — refusing to
// render is the safe failure mode, emitting a half-quoted command is not.
func TestU16_ShellSpecialCharacterSuppressesCommandLine(t *testing.T) {
	workspaceRoot := t.TempDir()
	intent := &events.RemediationIntent{
		Verb:             "quarantine",
		TargetFilename:   "checkpoint-$(evil).json",
		RequiresApproval: true,
		ApprovalClass:    "A4c",
		Reason:           "non_conforming",
	}

	var buf bytes.Buffer
	cli.RenderCheckpointRemediationBlock(&buf, intent, workspaceRoot)
	out := buf.String()

	require.NotEmpty(t, out)
	assert.Contains(t, out, "command not rendered: workspace or filename requires manual quoting")
	assert.NotContains(t, out, "backlogit --cwd", "no command line may be emitted when manual quoting is required")
	// The mandatory lines still appear; only the command line is suppressed.
	assert.Contains(t, out, "A4c")
}

// TestU16Guard_ConformingPathUnchanged asserts the renderer emits nothing at
// all when RemediationIntent is nil, so a conforming file's output is
// unchanged (green on landing, committed with the implementation).
func TestU16Guard_ConformingPathUnchanged(t *testing.T) {
	workspaceRoot := t.TempDir()

	var buf bytes.Buffer
	cli.RenderCheckpointRemediationBlock(&buf, nil, workspaceRoot)

	assert.Empty(t, buf.String(), "a nil RemediationIntent must render nothing")
}

