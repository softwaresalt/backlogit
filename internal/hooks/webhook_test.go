package hooks_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeHC is a helper that builds a minimal HookContext for webhook tests.
func makeHC(itemID string, topLevel bool) hooks.HookContext {
	return hooks.HookContext{
		ItemID:       itemID,
		ArtifactType: "task",
		TopLevel:     topLevel,
		OldValues:    map[string]any{},
		NewValues:    map[string]any{},
	}
}

// TestWebhookNotifier_DispatchSuccess verifies that a top-level dispatch sends
// an HTTP POST to the configured endpoint with a correctly serialised payload.
func TestWebhookNotifier_DispatchSuccess(t *testing.T) {
	var mu sync.Mutex
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedBody = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{{URL: srv.URL}},
		10, nil,
	)

	fn := n.DispatchWithEventType("create_artifact")
	hc := hooks.HookContext{
		ItemID:       "001-T",
		ArtifactType: "task",
		TopLevel:     true,
		OldValues:    map[string]any{},
		NewValues:    map[string]any{"title": "My Task"},
	}
	require.NoError(t, fn(context.Background(), hc))
	require.NoError(t, n.Shutdown(context.Background()))

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, receivedBody, "server should have received a POST body")

	var payload hooks.WebhookPayload
	require.NoError(t, json.Unmarshal(receivedBody, &payload))
	assert.Equal(t, 1, payload.SchemaVersion)
	assert.Equal(t, "create_artifact", payload.EventType)
	assert.Equal(t, "001-T", payload.ItemID)
	assert.Equal(t, "task", payload.ArtifactType)
	assert.Equal(t, "My Task", payload.Title)
	assert.False(t, payload.Timestamp.IsZero())
}

// TestWebhookNotifier_SkipsNonTopLevel verifies that no HTTP request is made
// when TopLevel is false, preventing duplicate notifications from nested ops.
func TestWebhookNotifier_SkipsNonTopLevel(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{{URL: srv.URL}},
		10, nil,
	)

	fn := n.DispatchWithEventType("create_artifact")
	require.NoError(t, fn(context.Background(), makeHC("001-T", false)))
	require.NoError(t, n.Shutdown(context.Background()))

	assert.Zero(t, callCount.Load(), "server should NOT have been called for non-top-level dispatch")
}

// TestWebhookNotifier_EventFilter verifies that endpoint event filters are
// respected: a non-matching event type is dropped; a matching one is sent.
func TestWebhookNotifier_EventFilter(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	endpoints := []hooks.WebhookEndpointConfig{
		{
			URL: srv.URL,
			EventFilter: map[string]struct{}{
				"create_artifact": {},
			},
		},
	}
	n := hooks.NewWebhookNotifier(endpoints, 10, nil)

	// "update_artifact" does not match the filter — server must NOT be called.
	fnUpdate := n.DispatchWithEventType("update_artifact")
	require.NoError(t, fnUpdate(context.Background(), makeHC("001-T", true)))
	require.NoError(t, n.Shutdown(context.Background()))
	assert.Zero(t, callCount.Load(), "update_artifact should be filtered out")

	// "create_artifact" matches the filter — server MUST be called.
	fnCreate := n.DispatchWithEventType("create_artifact")
	require.NoError(t, fnCreate(context.Background(), makeHC("001-T", true)))
	require.NoError(t, n.Shutdown(context.Background()))
	assert.Equal(t, int32(1), callCount.Load(), "create_artifact should reach the server")
}

// TestWebhookNotifier_EnvVarExpansion verifies that $VAR_NAME placeholders in
// the endpoint URL are resolved via os.ExpandEnv at construction time.
func TestWebhookNotifier_EnvVarExpansion(t *testing.T) {
	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("BACKLOGIT_TEST_WEBHOOK_URL", srv.URL)

	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{{URL: "$BACKLOGIT_TEST_WEBHOOK_URL"}},
		10, nil,
	)

	fn := n.DispatchWithEventType("create_artifact")
	require.NoError(t, fn(context.Background(), makeHC("001-T", true)))
	require.NoError(t, n.Shutdown(context.Background()))

	assert.True(t, called.Load(), "server should have been reached via env-var-expanded URL")
}

// TestWebhookNotifier_EndpointTimeout verifies that a slow server does not
// cause the dispatch goroutine to hang beyond the configured per-endpoint
// timeout.
func TestWebhookNotifier_EndpointTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{
			{URL: srv.URL, Timeout: 100 * time.Millisecond},
		},
		10, nil,
	)

	fn := n.DispatchWithEventType("create_artifact")
	require.NoError(t, fn(context.Background(), makeHC("001-T", true)))

	// Shutdown should complete well before the 2 s server delay because the
	// goroutine's HTTP request times out at 100 ms.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(shutdownCtx))
}

// TestWebhookNotifier_ShutdownDrains verifies that Shutdown waits for all
// in-flight goroutines before returning.
func TestWebhookNotifier_ShutdownDrains(t *testing.T) {
	var handlerCompleted atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a brief in-flight request.
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		handlerCompleted.Store(true)
	}))
	defer srv.Close()

	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{{URL: srv.URL}},
		10, nil,
	)

	fn := n.DispatchWithEventType("create_artifact")
	require.NoError(t, fn(context.Background(), makeHC("001-T", true)))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(shutdownCtx))

	// By the time Shutdown returns, the handler must have finished.
	assert.True(t, handlerCompleted.Load(), "handler should have completed before Shutdown returned")
}

// TestWebhookNotifier_ErrorLogged verifies that a 5xx response from the server
// is logged (not propagated) and DispatchWithEventType still returns nil.
func TestWebhookNotifier_ErrorLogged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{{URL: srv.URL}},
		10, nil,
	)

	fn := n.DispatchWithEventType("create_artifact")
	// Must return nil regardless of the 500 response.
	require.NoError(t, fn(context.Background(), makeHC("001-T", true)))
	require.NoError(t, n.Shutdown(context.Background()))
}

// TestWebhookNotifier_Headers verifies that custom headers configured on an
// endpoint are forwarded in the outbound HTTP request.
func TestWebhookNotifier_Headers(t *testing.T) {
	var mu sync.Mutex
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotHeaders = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{
			{
				URL: srv.URL,
				Headers: map[string]string{
					"X-Api-Key": "secret-key",
					"X-Custom":  "my-value",
				},
			},
		},
		10, nil,
	)

	fn := n.DispatchWithEventType("create_artifact")
	require.NoError(t, fn(context.Background(), makeHC("001-T", true)))
	require.NoError(t, n.Shutdown(context.Background()))

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, gotHeaders, "server should have received request headers")
	assert.Equal(t, "secret-key", gotHeaders.Get("X-Api-Key"))
	assert.Equal(t, "my-value", gotHeaders.Get("X-Custom"))
	assert.Equal(t, "application/json", gotHeaders.Get("Content-Type"))
}

// TestRegisterWebhookNotifier verifies that RegisterWebhookNotifier wires the
// notifier into the runner on all six hook points and the handler is invoked
// when FirePost is called.
func TestRegisterWebhookNotifier_AllPoints(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runner := hooks.NewHookRunner()
	n := hooks.NewWebhookNotifier(
		[]hooks.WebhookEndpointConfig{{URL: srv.URL}},
		100, nil,
	)
	hooks.RegisterWebhookNotifier(runner, n)

	hc := hooks.HookContext{
		ItemID:    "001-T",
		TopLevel:  true,
		OldValues: map[string]any{},
		NewValues: map[string]any{},
	}

	runner.FirePost(context.Background(), hooks.HookCreateArtifact, hc)
	runner.FirePost(context.Background(), hooks.HookUpdateArtifact, hc)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(shutdownCtx))

	assert.Equal(t, int32(2), callCount.Load(), "both hook points should have dispatched to the server")
}
