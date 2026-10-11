package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

const (
	// benchMemberCount is the release-scope size profiled by the U6 spike.
	benchMemberCount = 45
	// benchProgressEventsPerMember is the number of progress comments written to
	// each member log before its gate-pass evidence, approximating real task history.
	benchProgressEventsPerMember = 6
)

// newBenchMemberGateFixture builds a temporary, non-git workspace whose release
// scope holds benchMemberCount completed task members. Each member log carries
// creation, activation, progress comments, and one passing gate evidence event,
// the shape validateMemberGateEvidence reads at ship time.
func newBenchMemberGateFixture(b *testing.B) (*Workspace, []string) {
	b.Helper()
	ctx := context.Background()
	silenceBenchLogs(b)

	root := b.TempDir()
	backlogDir := filepath.Join(root, ".backlogit")
	require.NoError(b, os.MkdirAll(backlogDir, 0o755))
	require.NoError(b, config.WriteDefaults(backlogDir))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(b, err)
	b.Cleanup(func() { _ = ws.Close() })
	// Keep the fixture off the executable gate broker; members are completed
	// through the ungated path in seedBenchMember.
	ws.GateBroker = nil

	feat, err := CreateArtifact(ctx, ws, "U6 benchmark release feature", "feature")
	require.NoError(b, err)

	scope := make([]string, 0, benchMemberCount)
	for i := range benchMemberCount {
		scope = append(scope, seedBenchMember(ctx, b, ws, feat.ID, i))
	}
	return ws, scope
}

// silenceBenchLogs discards slog output for the rest of the benchmark. Fixture
// logs would otherwise share a line with the benchmark name and break the ns/op
// line that the U6 completion gate parses.
func silenceBenchLogs(b *testing.B) {
	b.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	b.Cleanup(func() { slog.SetDefault(prev) })
}

// seedBenchMember creates one task under parentID, drives it through its real
// lifecycle with progress comments, and appends a passing gate evidence event.
// It returns the member ID.
func seedBenchMember(ctx context.Context, b *testing.B, ws *Workspace, parentID string, index int) string {
	b.Helper()
	task, err := CreateArtifact(ctx, ws, fmt.Sprintf("U6 benchmark member %02d", index), "task", WithParent(parentID))
	require.NoError(b, err)
	_, err = UpdateArtifact(ctx, ws, task.ID, map[string]any{"status": "active"})
	require.NoError(b, err)
	for j := range benchProgressEventsPerMember {
		require.NoError(b, AppendComment(ctx, ws, nil, task.ID, "ship", fmt.Sprintf("progress note %d", j), ""))
	}
	_, err = updateArtifactUngated(ctx, ws, task.ID, map[string]any{"status": "done"})
	require.NoError(b, err)
	require.NoError(b, appendItemEventErr(ctx, ws, task.ID, EventGatePassed, map[string]any{
		"outcome": "passed",
		"ran":     true,
	}))
	return task.ID
}

// BenchmarkValidateMemberGateEvidence profiles ship-time member validation over a
// 45-member release scope. The shipment head is empty, so the per-member git
// lineage check is skipped and the timed loop isolates artifact lookup and
// event-log reads.
func BenchmarkValidateMemberGateEvidence(b *testing.B) {
	ctx := context.Background()
	ws, scope := newBenchMemberGateFixture(b)

	b.ResetTimer()
	for range b.N {
		if err := validateMemberGateEvidence(ctx, ws, scope, ""); err != nil {
			b.Fatalf("validate member gate evidence: %v", err)
		}
	}
}
