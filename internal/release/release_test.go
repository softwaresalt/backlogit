package release

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientLatest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		mustFprint(t, w, `{"tag_name":"v1.5.0"}`)
	}))
	t.Cleanup(server.Close)

	client := Client{
		HTTPClient: server.Client(),
		LatestURL:  server.URL,
		Token:      "test-token",
	}

	tag, err := client.Latest(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "v1.5.0", tag)
}

func TestClientLatestFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "non-200",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "rate limited", http.StatusTooManyRequests)
			},
		},
		{
			name: "malformed-json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				mustFprint(t, w, `{`)
			},
		},
		{
			name: "missing-tag",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				mustFprint(t, w, `{"name":"v1.5.0"}`)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tt.handler)
			t.Cleanup(server.Close)

			client := Client{HTTPClient: server.Client(), LatestURL: server.URL}
			tag, err := client.Latest(context.Background())

			require.Error(t, err)
			assert.Empty(t, tag)
		})
	}
}

func TestClientLatestRespectsContextTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		mustFprint(t, w, `{"tag_name":"v1.5.0"}`)
	}))
	t.Cleanup(server.Close)

	client := Client{HTTPClient: server.Client(), LatestURL: server.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	t.Cleanup(cancel)

	start := time.Now()
	tag, err := client.Latest(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Empty(t, tag)
	assert.Less(t, elapsed, 150*time.Millisecond, "latest lookup should honor caller timeout")
}

func TestClientLatestClosesBodyAfterReadError(t *testing.T) {
	t.Parallel()

	body := &errorReadCloser{readErr: errors.New("read failed")}
	client := Client{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       body,
				Header:     make(http.Header),
			}, nil
		})},
		LatestURL: "https://example.invalid/latest",
	}

	tag, err := client.Latest(context.Background())

	require.Error(t, err)
	assert.Empty(t, tag)
	assert.True(t, body.closed, "response body must be closed even when reads fail")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReadCloser struct {
	readErr error
	closed  bool
}

func (r *errorReadCloser) Read([]byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	return 0, io.EOF
}

func (r *errorReadCloser) Close() error {
	r.closed = true
	return nil
}

func mustFprint(t *testing.T, w io.Writer, value string) {
	t.Helper()

	_, err := fmt.Fprint(w, value)
	require.NoError(t, err)
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		latest  string
		want    int
		wantErr bool
	}{
		{name: "v-prefix-newer", current: "v1.4.1", latest: "1.4.2", want: -1},
		{name: "equal-with-v-prefix", current: "1.4.2", latest: "v1.4.2", want: 0},
		{name: "pre-release-before-release", current: "1.5.0-beta.1", latest: "1.5.0", want: -1},
		{name: "release-after-pre-release", current: "1.5.0", latest: "1.5.0-beta.1", want: 1},
		{name: "pseudo-version-before-release", current: "1.4.2-0.20260710210001-ac688fa42dba", latest: "1.4.2", want: -1},
		{name: "invalid-current", current: "dev", latest: "1.4.2", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CompareVersions(tt.current, tt.latest)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUpdateAvailable(t *testing.T) {
	t.Parallel()

	assert.True(t, UpdateAvailable("1.4.1", "v1.5.0"))
	assert.False(t, UpdateAvailable("v1.5.0", "1.5.0"))
	assert.False(t, UpdateAvailable("dev", "1.5.0"), "uncomparable dev builds should not claim an update")
}
