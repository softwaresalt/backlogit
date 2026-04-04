package stash_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/stash"
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
