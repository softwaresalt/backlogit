package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// WebhookEndpointConfig holds the resolved configuration for a single webhook target.
type WebhookEndpointConfig struct {
	URL         string
	EventFilter map[string]struct{} // empty means all events
	Headers     map[string]string
	Timeout     time.Duration
}

// WebhookPayload is the JSON body sent to webhook endpoints.
// It mirrors the compact HookEventPayload schema and MUST NOT include
// full old/new value maps or artifact body content.
type WebhookPayload struct {
	SchemaVersion int       `json:"schema_version"`
	EventType     string    `json:"event_type"`
	ItemID        string    `json:"item_id"`
	ArtifactType  string    `json:"artifact_type,omitempty"`
	Title         string    `json:"title,omitempty"`
	Status        string    `json:"status,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	ChangedFields []string  `json:"changed_fields,omitempty"`
}

// WebhookNotifier dispatches HTTP POST notifications to configured endpoints.
// Dispatch is async: goroutines handle individual POST requests. A shared
// rate limiter caps aggregate outbound traffic. Shutdown drains in-flight
// goroutines before the process exits.
type WebhookNotifier struct {
	endpoints   []WebhookEndpointConfig
	client      *http.Client
	rateLimiter *rate.Limiter
	logger      *slog.Logger
	wg          sync.WaitGroup
}

// NewWebhookNotifier creates a notifier for the given endpoints.
// ratePerSecond sets the shared token bucket rate (default 10 if <= 0).
// Endpoint URLs undergo os.ExpandEnv for environment variable resolution.
// Header values also undergo os.ExpandEnv.
func NewWebhookNotifier(endpoints []WebhookEndpointConfig, ratePerSecond int, logger *slog.Logger) *WebhookNotifier {
	if ratePerSecond <= 0 {
		ratePerSecond = 10
	}
	if logger == nil {
		logger = slog.Default()
	}

	resolved := make([]WebhookEndpointConfig, len(endpoints))
	for i, ep := range endpoints {
		resolved[i] = WebhookEndpointConfig{
			URL:         os.ExpandEnv(ep.URL),
			EventFilter: ep.EventFilter,
			Timeout:     ep.Timeout,
			Headers:     make(map[string]string, len(ep.Headers)),
		}
		for k, v := range ep.Headers {
			resolved[i].Headers[k] = os.ExpandEnv(v)
		}
		if resolved[i].Timeout <= 0 {
			resolved[i].Timeout = 10 * time.Second
		}
	}

	return &WebhookNotifier{
		endpoints:   resolved,
		client:      &http.Client{},
		rateLimiter: rate.NewLimiter(rate.Limit(ratePerSecond), ratePerSecond),
		logger:      logger,
	}
}

// DispatchWithEventType returns a HookFunc with a specific event type baked in
// via closure. Registered per hook point by RegisterWebhookNotifier so the event
// type is always known at dispatch time. The returned function skips non-top-level
// calls to prevent duplicate notifications from nested operations, always returns
// nil, and runs HTTP POSTs in goroutines tracked by the internal WaitGroup.
func (n *WebhookNotifier) DispatchWithEventType(eventType string) HookFunc {
	return func(ctx context.Context, hc HookContext) error {
		if !hc.TopLevel {
			return nil
		}

		payload := WebhookPayload{
			SchemaVersion: 1,
			EventType:     eventType,
			ItemID:        hc.ItemID,
			ArtifactType:  hc.ArtifactType,
			Timestamp:     time.Now().UTC(),
		}

		if title, ok := hc.NewValues["title"].(string); ok {
			payload.Title = title
		}
		if status, ok := hc.NewValues["status"].(string); ok {
			payload.Status = status
		}
		for k := range hc.NewValues {
			payload.ChangedFields = append(payload.ChangedFields, k)
		}

		for _, ep := range n.endpoints {
			if !n.matchesFilter(ep, eventType) {
				continue
			}
			n.dispatchToEndpoint(ctx, ep, payload)
		}

		return nil
	}
}

func (n *WebhookNotifier) matchesFilter(ep WebhookEndpointConfig, eventType string) bool {
	if len(ep.EventFilter) == 0 {
		return true
	}
	_, ok := ep.EventFilter[eventType]
	return ok
}

func (n *WebhookNotifier) dispatchToEndpoint(_ context.Context, ep WebhookEndpointConfig, payload WebhookPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		n.logger.Warn("webhook marshal error", "url", ep.URL, "error", err)
		return
	}

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()

		// Rate limit — blocks until a token is available.
		// Uses context.Background() so rate limiting outlives the parent hook
		// context and runs entirely in the goroutine to avoid blocking FirePost.
		if err := n.rateLimiter.Wait(context.Background()); err != nil {
			n.logger.Warn("webhook rate limit wait cancelled", "url", ep.URL, "error", err)
			return
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), ep.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, ep.URL, bytes.NewReader(body))
		if err != nil {
			n.logger.Warn("webhook request creation error", "url", ep.URL, "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range ep.Headers {
			req.Header.Set(k, v)
		}

		resp, err := n.client.Do(req)
		if err != nil {
			n.logger.Warn("webhook dispatch error", "url", ep.URL, "error", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			n.logger.Warn("webhook endpoint returned error",
				"url", ep.URL,
				"status_code", resp.StatusCode,
			)
		} else {
			n.logger.Info("webhook dispatched",
				"url", ep.URL,
				"status_code", resp.StatusCode,
				"event_type", payload.EventType,
			)
		}
	}()
}

// Shutdown drains all in-flight webhook dispatches and releases HTTP
// connections before returning. Called from Workspace.Close() to prevent
// goroutine and connection leaks in short-lived CLI processes.
func (n *WebhookNotifier) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		n.client.CloseIdleConnections()
		return nil
	case <-ctx.Done():
		n.client.CloseIdleConnections()
		return fmt.Errorf("webhook shutdown timed out: %w", ctx.Err())
	}
}

// RegisterWebhookNotifier registers the notifier's DispatchWithEventType as a
// post-hook on all hook points at priority 80.
func RegisterWebhookNotifier(runner *HookRunner, notifier *WebhookNotifier) {
	for _, point := range allHookPoints {
		p := point
		runner.Register(p, PhasePost, HookRegistration{
			Name:     "webhook_notifier",
			Priority: 80,
			Fn:       notifier.DispatchWithEventType(string(p)),
		})
	}
}
