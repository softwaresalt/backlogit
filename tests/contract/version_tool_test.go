package contract_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/version"
)

// TestGetVersion_ToolRegistered asserts that backlogit_get_version is registered
// on the MCP server and returns a structured response with the required fields.
func TestGetVersion_ToolRegistered(t *testing.T) {
	t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "1")
	s := setupRealMCPServer(t)

	data := callToolAndParseJSON(t, s, "backlogit_get_version", map[string]any{})
	require.NotNil(t, data)
	assert.NotEmpty(t, data["version"], "version field must be present and non-empty")
	assert.NotEmpty(t, data["current"], "current field must be present and non-empty")
	assert.Contains(t, data, "latest", "latest field must be present")
	assert.Contains(t, data, "update_available", "update_available field must be present")
	assert.Contains(t, data, "commit", "commit field must be present")
	assert.Contains(t, data, "build_date", "build_date field must be present")
	assert.NotEmpty(t, data["go_version"], "go_version field must be present and non-empty")
}

func TestGetVersion_LatestLookupContract(t *testing.T) {
	tests := []struct {
		name            string
		current         string
		lookup          func(context.Context) (string, error)
		wantLatest      string
		wantAvailable   bool
		wantUpdateCheck string
	}{
		{
			name:    "successful lookup",
			current: "1.0.0",
			lookup: func(context.Context) (string, error) {
				return "v1.1.0", nil
			},
			wantLatest:      "v1.1.0",
			wantAvailable:   true,
			wantUpdateCheck: "ok",
		},
		{
			name:    "unavailable lookup",
			current: "1.0.0",
			lookup: func(context.Context) (string, error) {
				return "", errors.New("offline")
			},
			wantLatest:      "",
			wantAvailable:   false,
			wantUpdateCheck: "unavailable",
		},
		{
			name:    "uncomparable current",
			current: version.DevVersion,
			lookup: func(context.Context) (string, error) {
				return "v1.1.0", nil
			},
			wantLatest:      "v1.1.0",
			wantAvailable:   false,
			wantUpdateCheck: "uncomparable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "")
			restoreVersion := setVersionForContractTest(t, tt.current)
			defer restoreVersion()
			s := setupRealMCPServer(t)
			s.LatestVersionLookup = tt.lookup

			data := callToolAndParseJSON(t, s, "backlogit_get_version", map[string]any{})

			assert.Equal(t, tt.current, data["current"])
			assert.Equal(t, tt.wantLatest, data["latest"])
			assert.Equal(t, tt.wantAvailable, data["update_available"])
			assert.Equal(t, tt.wantUpdateCheck, data["update_check"])
		})
	}
}

func TestGetVersion_NoUpdateCheckArgumentContract(t *testing.T) {
	t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "")
	called := false
	s := setupRealMCPServer(t)
	s.LatestVersionLookup = func(context.Context) (string, error) {
		called = true
		return "v9.9.9", nil
	}

	data := callToolAndParseJSON(t, s, "backlogit_get_version", map[string]any{"no_update_check": true})

	assert.False(t, called, "no_update_check should skip the latest-release lookup")
	assert.Equal(t, "skipped", data["update_check"])
	assert.Equal(t, false, data["update_available"])
}

func setVersionForContractTest(t *testing.T, value string) func() {
	t.Helper()
	original := version.Version
	version.Version = value
	return func() {
		version.Version = original
	}
}
