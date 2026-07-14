package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// zoneOffsetRECLI matches a trailing numeric zone offset such as "-07:00".
var zoneOffsetRECLI = regexp.MustCompile(`[+-]\d{2}:\d{2}$`)

// TestUpdateCommand_SectionWrite_EmitsUTCUpdatedAt proves the CLI update
// --section path serializes updated_at in canonical UTC (trailing "Z") even
// when the process runs in a non-UTC timezone (site: cli/update.go section
// write). internal/cli runs t.Parallel tests, so a process-global time.Local
// override is unsafe; instead this drives a hermetic subprocess with
// TZ=America/Los_Angeles (a fixed non-UTC zone) and asserts the emitted
// frontmatter timestamp ends with exactly "Z".
func TestUpdateCommand_SectionWrite_EmitsUTCUpdatedAt(t *testing.T) {
	t.Parallel()

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperUpdateSectionUTCChild$", "-test.v=true") //nolint:gosec // re-exec of the test binary is the standard hermetic-subprocess pattern
	cmd.Env = append(os.Environ(), "BACKLOGIT_UTC_HELPER=1", "TZ=America/Los_Angeles")
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "hermetic subprocess failed:\n%s", out)

	var value string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "UTC_UPDATED_AT=") {
			value = strings.TrimPrefix(line, "UTC_UPDATED_AT=")
			break
		}
	}
	require.NotEmptyf(t, value, "helper did not emit UTC_UPDATED_AT marker; output:\n%s", out)
	assert.Truef(t, strings.HasSuffix(value, "Z"),
		"updated_at must end with exactly Z, got %q", value)
	assert.Falsef(t, zoneOffsetRECLI.MatchString(value),
		"updated_at must not carry a numeric zone offset, got %q", value)
}

// TestHelperUpdateSectionUTCChild is the hermetic child of
// TestUpdateCommand_SectionWrite_EmitsUTCUpdatedAt. It runs only when
// re-executed with BACKLOGIT_UTC_HELPER=1 (and TZ set to a non-UTC zone); a
// normal test run skips it. It performs a real `update --section` write and
// prints the on-disk updated_at value on a UTC_UPDATED_AT= marker line.
func TestHelperUpdateSectionUTCChild(t *testing.T) {
	if os.Getenv("BACKLOGIT_UTC_HELPER") != "1" {
		t.Skip("helper process; only runs when re-executed by the parent test")
	}

	// Guard: the hermetic zone must actually be non-UTC, otherwise a local-offset
	// regression could pass silently. TZ=America/Los_Angeles yields a nonzero
	// offset; if time.Local is UTC here the TZ override did not take effect.
	if _, offset := time.Now().Zone(); offset == 0 {
		t.Fatal("TZ override did not take effect; time.Local is UTC, hermetic red would be meaningless")
	}

	root := setupCLIWorkspace(t)
	ctx := context.Background()

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Helper feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Helper task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", task.ID, "--section", "description=UTC helper body"})
	require.NoErrorf(t, cmd.Execute(), "update --section failed: %s", buf.String())

	ws2, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	filePath, err := core.FindArtifactPath(ctx, ws2, task.ID)
	require.NoError(t, err)
	ws2.Close()

	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)

	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "updated_at:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "updated_at:"))
			fmt.Printf("UTC_UPDATED_AT=%s\n", value)
			return
		}
	}
	t.Fatalf("updated_at not found in frontmatter:\n%s", raw)
}
