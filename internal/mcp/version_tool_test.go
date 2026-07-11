package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/version"
)

// TestHandleGetVersion_ReturnsVersionFields asserts that handleGetVersion returns
// a non-error result containing version, commit, build_date, and go_version.
func TestHandleGetVersion_ReturnsVersionFields(t *testing.T) {
	t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "1")
	root := t.TempDir()
	s := NewServerForRoot(root)

	result, err := s.handleGetVersion(context.Background(), mcplib.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "backlogit_get_version must not return an error result")
	require.NotEmpty(t, result.Content, "result must have at least one content item")

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "result content must be TextContent")

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data), "result text must be valid JSON")
	assert.NotEmpty(t, data["version"], "version field must be present and non-empty")
	assert.NotEmpty(t, data["current"], "current field must be present and non-empty")
	assert.Contains(t, data, "latest", "latest field must be present")
	assert.Contains(t, data, "update_available", "update_available field must be present")
	assert.Equal(t, false, data["update_available"], "skipped update check should not report an update")
	assert.Equal(t, "skipped", data["update_check"], "env skip should be reported")
	assert.Contains(t, data, "commit", "commit field must be present")
	assert.Contains(t, data, "build_date", "build_date field must be present")
	assert.NotEmpty(t, data["go_version"], "go_version field must be present and non-empty")
}

// TestHandleGetVersion_WorksWithoutWorkspace asserts that handleGetVersion succeeds
// even when the .backlogit directory has not been initialized.
func TestHandleGetVersion_WorksWithoutWorkspace(t *testing.T) {
	t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "1")
	emptyDir := t.TempDir()
	_, err := os.Stat(filepath.Join(emptyDir, ".backlogit"))
	require.True(t, os.IsNotExist(err), "pre-condition: .backlogit should not exist")

	s := NewServerForRoot(emptyDir)
	result, err := s.handleGetVersion(context.Background(), mcplib.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "tool must succeed without a workspace")
}

func TestHandleGetVersion_LatestLookupStates(t *testing.T) {
	tests := []struct {
		name            string
		current         string
		lookup          func(context.Context) (string, error)
		wantLatest      string
		wantAvailable   bool
		wantUpdateCheck string
	}{
		{
			name:    "successful lookup reports newer latest",
			current: "1.0.0",
			lookup: func(context.Context) (string, error) {
				return "v1.2.0", nil
			},
			wantLatest:      "v1.2.0",
			wantAvailable:   true,
			wantUpdateCheck: "ok",
		},
		{
			name:    "lookup failure degrades gracefully",
			current: "1.0.0",
			lookup: func(context.Context) (string, error) {
				return "", errors.New("offline")
			},
			wantLatest:      "",
			wantAvailable:   false,
			wantUpdateCheck: "unavailable",
		},
		{
			name:    "uncomparable current still reports latest",
			current: version.DevVersion,
			lookup: func(context.Context) (string, error) {
				return "v1.2.0", nil
			},
			wantLatest:      "v1.2.0",
			wantAvailable:   false,
			wantUpdateCheck: "uncomparable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "")
			restoreVersion := setVersionForTest(t, tt.current)
			defer restoreVersion()
			s := NewServerForRoot(t.TempDir())
			s.LatestVersionLookup = tt.lookup

			data := callGetVersionForTest(t, s)

			assert.Equal(t, tt.current, data["current"])
			assert.Equal(t, tt.wantLatest, data["latest"])
			assert.Equal(t, tt.wantAvailable, data["update_available"])
			assert.Equal(t, tt.wantUpdateCheck, data["update_check"])
		})
	}
}

func TestHandleGetVersion_NoUpdateCheckArgumentSkipsLookup(t *testing.T) {
	t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "")
	called := false
	s := NewServerForRoot(t.TempDir())
	s.LatestVersionLookup = func(context.Context) (string, error) {
		called = true
		return "v9.9.9", nil
	}
	request := mcplib.CallToolRequest{}
	request.Params.Arguments = map[string]any{"no_update_check": true}

	data := callGetVersionRequestForTest(t, s, request)

	assert.False(t, called, "no_update_check argument should skip the lookup")
	assert.Equal(t, "skipped", data["update_check"])
}

func TestHandleGetVersion_ServerNoUpdateCheckSkipsLookup(t *testing.T) {
	t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "")
	called := false
	s := NewServerForRoot(t.TempDir())
	s.NoUpdateCheck = true
	s.LatestVersionLookup = func(context.Context) (string, error) {
		called = true
		return "v9.9.9", nil
	}

	data := callGetVersionForTest(t, s)

	assert.False(t, called, "server NoUpdateCheck should skip the lookup")
	assert.Equal(t, "skipped", data["update_check"])
}

func callGetVersionForTest(t *testing.T, s *Server) map[string]any {
	t.Helper()
	return callGetVersionRequestForTest(t, s, mcplib.CallToolRequest{})
}

func callGetVersionRequestForTest(t *testing.T, s *Server, request mcplib.CallToolRequest) map[string]any {
	t.Helper()
	result, err := s.handleGetVersion(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "result content must be TextContent")

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data), "result text must be valid JSON")
	return data
}

func setVersionForTest(t *testing.T, value string) func() {
	t.Helper()
	original := version.Version
	version.Version = value
	return func() {
		version.Version = original
	}
}
