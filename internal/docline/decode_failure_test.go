package docline

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyDecodeFailure is the classifyDecodeFailure half of 146.018-T
// (U8)'s two-helper split: a table over the three real decodeDoc error
// shapes (containment, frontmatter, and a wrapped *fs.PathError standing in
// for read/I-O), asserting the policy-neutral kind and that the cause is
// returned unchanged.
func TestClassifyDecodeFailure(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind decodeFailureKind
	}{
		{
			name:     "containment",
			err:      fmt.Errorf("docline.decodeDoc: %s: %w", "../escape.md", ErrPathEscapesWorkspace),
			wantKind: decodeFailureContainment,
		},
		{
			name:     "frontmatter",
			err:      fmt.Errorf("docline.decodeDoc: decode %s: %w", "docs/decisions/broken.md", ErrFrontmatterDecode),
			wantKind: decodeFailureFrontmatter,
		},
		{
			name: "read_io_wrapped_path_error",
			err: fmt.Errorf("docline.decodeDoc: read %s: %w", "docs/decisions/x.md",
				&fs.PathError{Op: "open", Path: `C:\Source\GitHub\backlogit\docs\decisions\x.md`, Err: errors.New("access is denied")}),
			wantKind: decodeFailureRead,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, cause := classifyDecodeFailure(tt.err)
			assert.Equal(t, tt.wantKind, kind)
			assert.Same(t, tt.err, cause, "classifyDecodeFailure must return the cause unchanged")
		})
	}
}

// TestClassifyDecodeFailure_NilInput is the fail-closed guard: a nil input is
// classified as decodeFailureRead (the propagate default) with a synthesized,
// non-nil error rather than panicking or being treated as "no failure".
func TestClassifyDecodeFailure_NilInput(t *testing.T) {
	kind, cause := classifyDecodeFailure(nil)
	assert.Equal(t, decodeFailureRead, kind)
	require.Error(t, cause)
}

// TestApplyDecodeFailure_ContainmentStaysFatal is scenario A of 146.018-T
// (U8)'s applyDecodeFailure guard: a containment failure must never become a
// Finding.
func TestApplyDecodeFailure_ContainmentStaysFatal(t *testing.T) {
	err := fmt.Errorf("docline.collectInScopeDocs: %s: %w", "../escape", ErrPathEscapesWorkspace)

	findings, fatal := applyDecodeFailure(err, "../escape")
	assert.Nil(t, findings, "a containment failure must never become a finding")
	require.Error(t, fatal)
	assert.True(t, errors.Is(fatal, ErrPathEscapesWorkspace))
}

// TestApplyDecodeFailure_WrappedPathErrorStaysFatal is scenario B: this
// closes the fail-open gap where a naive U8 that turned EVERY decodeDoc
// error into a finding (leaking the *fs.PathError's absolute host path into
// Fix) would still pass every U7b scenario, because U7b never drives a real
// read/I-O failure through LintTree end-to-end. A wrapped *fs.PathError read
// failure must stay fatal, never become a finding.
func TestApplyDecodeFailure_WrappedPathErrorStaysFatal(t *testing.T) {
	pathErr := &fs.PathError{Op: "open", Path: `C:\Source\GitHub\backlogit\docs\decisions\x.md`, Err: errors.New("access is denied")}
	err := fmt.Errorf("docline.decodeDoc: read %s: %w", "docs/decisions/x.md", pathErr)

	findings, fatal := applyDecodeFailure(err, "docs/decisions/x.md")
	assert.Nil(t, findings, "a wrapped *fs.PathError read failure must never become a finding")
	require.Error(t, fatal)
	var asPathErr *fs.PathError
	assert.True(t, errors.As(fatal, &asPathErr), "the fatal error must still unwrap to the original *fs.PathError")
	assert.False(t, errors.Is(fatal, ErrFrontmatterDecode))
}

// TestApplyDecodeFailure_FrontmatterBecomesFinding is scenario C: the only
// classification applyDecodeFailure converts into a Finding.
func TestApplyDecodeFailure_FrontmatterBecomesFinding(t *testing.T) {
	err := fmt.Errorf("docline.decodeDoc: decode %s: %w", "docs/decisions/broken.md", ErrFrontmatterDecode)

	findings, fatal := applyDecodeFailure(err, "docs/decisions/broken.md")
	require.NoError(t, fatal)
	require.Len(t, findings, 1)
	assert.Equal(t, "docs/decisions/broken.md", findings[0].File)
	assert.Equal(t, RuleDecodeError, findings[0].Rule)
	assert.Equal(t, SeverityError, findings[0].Severity)
}
