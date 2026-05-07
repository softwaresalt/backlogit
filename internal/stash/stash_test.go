package stash_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/stash"
)

func TestParseContent_ReadsEntriesWithIDs(t *testing.T) {
	content := `---
title: Stash
description: Ideas
---

## Stash

- [ ] [A1B2C3D4] [priority:high] [deliberation:DL001] feature: Build reporting
- [ ] [DEADBEEF] task: Tighten validation
`

	_, entries, err := stash.ParseContent(content)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "A1B2C3D4", entries[0].ID)
	assert.Equal(t, "high", entries[0].Priority)
	assert.Equal(t, "DL001", entries[0].DeliberationID)
	assert.Equal(t, "feature", entries[0].Kind)
	assert.Equal(t, "Build reporting", entries[0].Text)
	assert.Equal(t, stash.DefaultPriority, entries[1].Priority)
}

func TestGenerateID_ReturnsEightChars(t *testing.T) {
	id, err := stash.GenerateID(map[string]struct{}{})
	require.NoError(t, err)
	assert.Len(t, id, 8)
}

func TestFormatEntry_IncludesPriority(t *testing.T) {
	line := stash.FormatEntry(stash.Entry{
		ID:             "A1B2C3D4",
		Priority:       "critical",
		DeliberationID: "DL001",
		Kind:           "bug",
		Text:           "Fix broken harness",
	})

	assert.Equal(t, "- [ ] [A1B2C3D4] [priority:critical] [deliberation:DL001] bug: Fix broken harness", line)
}

func TestEntry_CreatedAt_JSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	entry := stash.Entry{
		ID:        "AABBCCDD",
		Priority:  "high",
		Kind:      "feature",
		Text:      "Round-trip test",
		CreatedAt: &now,
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.Contains(t, string(data), "created_at")

	var decoded stash.Entry
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, entry.ID, decoded.ID)
	require.NotNil(t, decoded.CreatedAt)
	assert.Equal(t, entry.CreatedAt.Unix(), decoded.CreatedAt.Unix())
}

func TestEntry_CreatedAt_OmittedWhenZero(t *testing.T) {
	entry := stash.Entry{
		ID:       "DEADBEEF",
		Priority: "low",
		Kind:     "task",
		Text:     "No timestamp",
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "created_at")
}

func TestNormalizeKind_ExtendedBaseSet(t *testing.T) {
	for _, kind := range []string{"spike", "subtask", "deliberation", "review", "shipment"} {
		got, err := stash.NormalizeKind(kind)
		require.NoErrorf(t, err, "NormalizeKind(%q) should succeed", kind)
		assert.Equal(t, kind, got)
	}
}

func TestNormalizeKindWithExtras_AcceptsExtraKinds(t *testing.T) {
	got, err := stash.NormalizeKindWithExtras("custom", []string{"custom", "other"})
	require.NoError(t, err)
	assert.Equal(t, "custom", got)
}

func TestNormalizeKindWithExtras_RejectsUnknownKind(t *testing.T) {
	_, err := stash.NormalizeKindWithExtras("notakind", []string{"custom"})
	require.Error(t, err)
}

func TestNormalizeKindWithExtras_BaseKindsAlwaysAccepted(t *testing.T) {
	// Extra list is empty — base kinds still work.
	got, err := stash.NormalizeKindWithExtras("feature", nil)
	require.NoError(t, err)
	assert.Equal(t, "feature", got)
}

func TestAllowedKindsWithExtras_MergesAndDeduplicates(t *testing.T) {
	extras := []string{"custom", "feature"} // "feature" is already in base
	kinds := stash.AllowedKindsWithExtras(extras)
	assert.Contains(t, kinds, "custom")
	assert.Contains(t, kinds, "feature")
	assert.Contains(t, kinds, "spike")
	// No duplicates
	seen := make(map[string]int)
	for _, k := range kinds {
		seen[k]++
	}
	for k, count := range seen {
		assert.Equal(t, 1, count, "kind %q appears more than once", k)
	}
}

func TestAllowedKinds_IncludesNewKinds(t *testing.T) {
	kinds := stash.AllowedKinds()
	for _, want := range []string{"spike", "subtask", "deliberation", "review", "shipment"} {
		assert.Contains(t, kinds, want, "expected AllowedKinds to include %q", want)
	}
}
