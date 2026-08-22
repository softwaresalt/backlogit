package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// findToolDescription returns the registered description string for the
// named MCP tool, so contract text is always read from the exported server
// handle rather than a package-level literal that could go unregistered.
func findToolDescription(t *testing.T, s *Server, name string) string {
	t.Helper()
	for _, info := range s.DescribeTools() {
		if info.Name == name {
			return info.Description
		}
	}
	t.Fatalf("tool %q is not registered", name)
	return ""
}

// reflectedLegalCheckpointKeys derives the legal CheckpointV1 top-level and
// CheckpointProgress nested keys by reflection over the exported json tags,
// mirroring the derivation 146.011-T (U4) uses at the create boundary. This
// is a TEST-local derivation so a future modeled field cannot silently
// desynchronize the documented contract from the enforced one; it does not
// call any production-derived key set.
func reflectedLegalCheckpointKeys(t *testing.T) []string {
	t.Helper()
	var keys []string
	for _, typ := range []reflect.Type{
		reflect.TypeOf(events.CheckpointV1{}),
		reflect.TypeOf(events.CheckpointProgress{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.PkgPath != "" {
				continue
			}
			tag := f.Tag.Get("json")
			if tag == "-" || tag == "" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" || name == "context" {
				continue
			}
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	return keys
}

// TestHandleCreateCheckpoint_TwoUnknownTopLevelKeys_ValidationFailed is
// scenario 1 of 146.010-T (U3d): dispatched through the registered
// handleCreateCheckpoint, a dump with two unknown top-level keys must return
// a validation_failed result — not internal — whose unknown_fields array is
// read with a .([]any) type assertion and contains both keys, sorted (R12).
func TestHandleCreateCheckpoint_TwoUnknownTopLevelKeys_ValidationFailed(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_create_checkpoint"
	request.Params.Arguments = map[string]any{
		"state_dump": `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","zeta_key":"x","alpha_key":"y"}`,
	}

	result, err := s.handleCreateCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError, "a dump with unknown top-level keys must fail the tool call as validation_failed")

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))

	assert.Equal(t, "validation_failed", resp["error"])
	rawFields, ok := resp["unknown_fields"].([]any)
	require.True(t, ok, "unknown_fields must be present and decode as a JSON array")
	got := make([]string, 0, len(rawFields))
	for _, f := range rawFields {
		fieldName, ok := f.(string)
		require.True(t, ok)
		got = append(got, fieldName)
	}
	sort.Strings(got)
	assert.Equal(t, []string{"alpha_key", "zeta_key"}, got)
}

// TestBacklogitCreateCheckpoint_DescriptionEnumeratesLegalKeys is scenario 2
// of 146.010-T (U3d): the registered backlogit_create_checkpoint tool
// description contains the legal-key enumeration, the "arbitrary keys belong
// inside context" sentence, and the context_keys sentence, so the
// agent-facing half of R11 cannot ship stale the way the CLI half nearly
// did.
func TestBacklogitCreateCheckpoint_DescriptionEnumeratesLegalKeys(t *testing.T) {
	s, _ := setupBugFixServer(t)
	desc := findToolDescription(t, s, "backlogit_create_checkpoint")

	assert.Contains(t, strings.ToLower(desc), "context_keys",
		"the description must name context_keys")
	assert.True(t,
		strings.Contains(desc, "context") && (strings.Contains(strings.ToLower(desc), "arbitrary") || strings.Contains(strings.ToLower(desc), "any other key")),
		"the description must state that arbitrary/unmodeled keys belong inside context",
	)
}

// TestBacklogitCreateCheckpoint_DescriptionNamesEveryReflectedKey is scenario
// 3 of 146.010-T (U3d): the legal-key enumeration in the registered
// description is derived by reflection over CheckpointV1 and
// CheckpointProgress in this test, and every derived key must appear
// somewhere in the description string, so a future modeled field cannot
// silently desynchronize the documented contract from the enforced one. The
// reflection-derived set necessarily includes the disposition_* fields,
// which are administrative/reserved rather than caller-supplied create
// inputs; this scenario only asserts each key appears SOMEWHERE in the
// description, not that it appears in the caller-supplied group.
func TestBacklogitCreateCheckpoint_DescriptionNamesEveryReflectedKey(t *testing.T) {
	s, _ := setupBugFixServer(t)
	desc := findToolDescription(t, s, "backlogit_create_checkpoint")

	keys := reflectedLegalCheckpointKeys(t)
	require.NotEmpty(t, keys)
	for _, k := range keys {
		assert.Contains(t, desc, k, "the registered description must enumerate reflection-derived legal key %q", k)
	}
}
