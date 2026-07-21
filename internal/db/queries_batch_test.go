package db_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestGetItemsByIDs_ChunkBoundaries exercises the 900-item chunk boundary and the
// multi-chunk resolution path in GetItemsByIDs. The colocated size-composition
// tests only pass a handful of IDs, so a chunking off-by-one (a single chunk that
// silently drops the 901st ID, or a boundary that duplicates a row across chunks)
// would otherwise pass the suite. This seeds 901 rows and asserts the resolver
// returns every requested indexed ID across the chunk split, while de-duplicating
// inputs and omitting misses.
func TestGetItemsByIDs_ChunkBoundaries(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	const total = 901 // one full 900 chunk + 1, forcing a second chunk
	ids := make([]string, total)
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("B%04d-T", i)
		ids[i] = id
		require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{
			ID: id, Title: "batch " + id, Status: models.StatusActive, ArtifactType: "task",
		}))
	}

	cases := []struct {
		name    string
		input   []string
		wantLen int
		// spot-checked IDs that must be present in the result.
		wantPresent []string
		// IDs that must be absent from the result.
		wantAbsent []string
	}{
		{
			name:        "single full chunk (900)",
			input:       ids[:900],
			wantLen:     900,
			wantPresent: []string{"B0000-T", "B0899-T"},
			wantAbsent:  []string{"B0900-T"},
		},
		{
			name:        "crosses chunk boundary (901)",
			input:       ids,
			wantLen:     901,
			wantPresent: []string{"B0000-T", "B0899-T", "B0900-T"},
		},
		{
			name: "duplicates and missing across boundary",
			// 901 unique real IDs, plus duplicates of a first-chunk and a
			// second-chunk ID, plus two IDs with no indexed row and an empty.
			input: append(append([]string{}, ids...),
				"B0001-T", "B0900-T", "Z9998-T", "Z9999-T", ""),
			wantLen:     901,
			wantPresent: []string{"B0001-T", "B0900-T"},
			wantAbsent:  []string{"Z9998-T", "Z9999-T", ""},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := db.GetItemsByIDs(ctx, database, tc.input)
			require.NoError(t, err)
			assert.Len(t, resolved, tc.wantLen)
			for _, id := range tc.wantPresent {
				assert.Contains(t, resolved, id, "expected %s present", id)
			}
			for _, id := range tc.wantAbsent {
				assert.NotContains(t, resolved, id, "expected %s absent", id)
			}
		})
	}
}
