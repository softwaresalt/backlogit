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
