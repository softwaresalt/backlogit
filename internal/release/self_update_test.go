package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientReleaseByTagAndDownloadAsset(t *testing.T) {
	t.Parallel()

	payload := []byte("new-binary")
	sum := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/tags/v1.2.3":
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			mustFprint(t, w, fmt.Sprintf(`{
				"tag_name":"v1.2.3",
				"assets":[
					{"name":"backlogit-windows-amd64.exe","browser_download_url":"%s/assets/backlogit-windows-amd64.exe"},
					{"name":"SHA256SUMS","browser_download_url":"%s/assets/SHA256SUMS"}
				]
			}`, serverURL(r), serverURL(r)))
		case "/assets/backlogit-windows-amd64.exe":
			assert.Empty(t, r.Header.Get("Authorization"))
			_, err := w.Write(payload)
			require.NoError(t, err)
		case "/assets/SHA256SUMS":
			assert.Empty(t, r.Header.Get("Authorization"))
			mustFprint(t, w, fmt.Sprintf("%s  backlogit-windows-amd64.exe\n", hex.EncodeToString(sum[:])))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := Client{HTTPClient: server.Client(), BaseURL: server.URL, Token: "test-token"}
	rel, err := client.ReleaseByTag(context.Background(), "v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", rel.TagName)

	asset, ok := FindAsset(rel.Assets, "backlogit-windows-amd64.exe")
	require.True(t, ok)
	var got bytes.Buffer
	n, err := client.DownloadAsset(context.Background(), asset, &got, int64(len(payload)))
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), n)
	assert.Equal(t, payload, got.Bytes())

	sumsAsset, ok := FindAsset(rel.Assets, "SHA256SUMS")
	require.True(t, ok)
	var sums bytes.Buffer
	_, err = client.DownloadAsset(context.Background(), sumsAsset, &sums, 1<<20)
	require.NoError(t, err)
	parsed, err := ParseSHA256SUMS(sums.String())
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(sum[:]), parsed["backlogit-windows-amd64.exe"])
}

func TestClientLatestReleaseUsesBaseURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/releases/latest", r.URL.Path)
		mustFprint(t, w, `{"tag_name":"v1.2.3","assets":[]}`)
	}))
	t.Cleanup(server.Close)

	client := Client{HTTPClient: server.Client(), BaseURL: server.URL}
	rel, err := client.LatestRelease(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", rel.TagName)
}

func TestParseSHA256SUMSRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{name: "missing filename", data: "abc123\n"},
		{name: "short hash", data: "abc123  backlogit-linux-amd64\n"},
		{name: "bad hex", data: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz  backlogit-linux-amd64\n"},
		{name: "path traversal", data: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  ../backlogit-linux-amd64\n"},
		{name: "windows separator", data: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  nested\\backlogit.exe\n"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := ParseSHA256SUMS(tt.data)

			require.Error(t, err)
			assert.Empty(t, parsed)
		})
	}
}

func TestParseSHA256SUMSAllowsDotSlashBasename(t *testing.T) {
	t.Parallel()

	parsed, err := ParseSHA256SUMS("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  ./backlogit-linux-amd64\n")

	require.NoError(t, err)
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", parsed["backlogit-linux-amd64"])
}

func TestDownloadAssetEnforcesBoundedSize(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("too-large"))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := Client{HTTPClient: server.Client()}
	var got bytes.Buffer
	n, err := client.DownloadAsset(context.Background(), Asset{
		Name:               "backlogit-linux-amd64",
		BrowserDownloadURL: server.URL,
	}, &got, 3)

	require.Error(t, err)
	assert.Zero(t, n)
	assert.Empty(t, got.Bytes())
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
