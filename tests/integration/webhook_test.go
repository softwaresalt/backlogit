package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/hooks"
)

func TestWebhookIntegration_CreateArtifactFiresWebhook(t *testing.T) {
	// Track received webhook payloads.
	var mu sync.Mutex
	var received []hooks.WebhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload hooks.WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Logf("webhook decode error: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set up workspace with webhook endpoint configured.
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	// Write hooks.yaml with the test server as an endpoint.
	hooksCfg := config.DefaultHooksConfig()
	hooksCfg.Notifications.Endpoints = []config.WebhookEndpoint{
		{
			URL:         server.URL,
			TimeoutSecs: 5,
		},
	}
	hooksData, err := yaml.Marshal(hooksCfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(backlogitDir, "hooks.yaml"), hooksData, 0o644))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	// Create a level-1 artifact (feature — doesn't require parent_id).
	_, err = core.CreateArtifact(ctx, ws, "Test Feature for Webhook", "feature")
	require.NoError(t, err)

	// Close workspace to drain in-flight webhooks.
	require.NoError(t, ws.Close())

	// Give async dispatches time to complete (Shutdown should have drained them).
	time.Sleep(100 * time.Millisecond)

	// Verify the webhook was received.
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, received, "webhook endpoint should have received at least one POST")

	payload := received[0]
	assert.Equal(t, 1, payload.SchemaVersion)
	assert.Equal(t, "create_artifact", payload.EventType)
	assert.NotEmpty(t, payload.ItemID)
	assert.Equal(t, "feature", payload.ArtifactType)
	assert.False(t, payload.Timestamp.IsZero())
}

func TestWebhookIntegration_NoEndpoints_NoWebhook(t *testing.T) {
	// Set up workspace WITHOUT webhook endpoints.
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	// Default hooks.yaml has no endpoints, so no webhook should fire.

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close() //nolint:errcheck

	// HookRunner should exist but no webhook notifier.
	require.NotNil(t, ws.HookRunner)

	// Creating an artifact should succeed without any webhook infrastructure.
	_, err = core.CreateArtifact(ctx, ws, "Test Feature No Webhook", "feature")
	require.NoError(t, err)
}
