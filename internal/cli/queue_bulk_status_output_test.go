package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestU172_WriteBulkStatusResult_RendersFailedDetails(t *testing.T) {
	tests := []struct {
		name   string
		result *core.BulkUpdateResult
		want   string
	}{
		{
			name:   "all succeeded",
			result: &core.BulkUpdateResult{Succeeded: 2},
			want:   "Updated 2 items to status \"done\"\n",
		},
		{
			name: "ordinary failure has no detail line",
			result: &core.BulkUpdateResult{
				Succeeded: 1,
				Failed:    []string{"002-T"},
			},
			want: "Updated 1 items to status \"done\"\n" +
				"Failed to update 1 items: 002-T\n",
		},
		{
			name: "conflict renders detail with error",
			result: &core.BulkUpdateResult{
				Failed: []string{"001-T", "002-T"},
				FailedDetails: []core.BulkUpdateConflict{{
					ID:         "002-T",
					Err:        fmt.Errorf("archived_status changed: %w", blerrors.ErrShipmentConflict),
					FromStatus: "queued",
					ToStatus:   "done",
				}},
			},
			want: "Updated 0 items to status \"done\"\n" +
				"Failed to update 2 items: 001-T, 002-T\n" +
				"Failed detail 002-T: queued -> done: archived_status changed: " + blerrors.ErrShipmentConflict.Error() + "\n",
		},
		{
			name: "conflict without error omits suffix",
			result: &core.BulkUpdateResult{
				Failed:        []string{"003-T"},
				FailedDetails: []core.BulkUpdateConflict{{ID: "003-T", FromStatus: "active", ToStatus: "done"}},
			},
			want: "Updated 0 items to status \"done\"\n" +
				"Failed to update 1 items: 003-T\n" +
				"Failed detail 003-T: active -> done\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeBulkStatusResult(&buf, "done", tt.result)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
