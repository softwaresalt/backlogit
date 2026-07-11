// Package release looks up and compares published backlogit releases.
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	// DefaultBaseURL is the GitHub Releases API base for backlogit.
	DefaultBaseURL = "https://api.github.com/repos/softwaresalt/backlogit"
	// DefaultLatestURL is the GitHub Releases endpoint for the latest backlogit release.
	DefaultLatestURL = DefaultBaseURL + "/releases/latest"

	// UpdateCheckOK means the latest version was fetched and compared successfully.
	UpdateCheckOK = "ok"
	// UpdateCheckUncomparable means the latest version was fetched but comparison was not possible.
	UpdateCheckUncomparable = "uncomparable"

	defaultUserAgent = "backlogit"
	maxResponseBytes = 1 << 20
)

// Client queries backlogit releases and release assets.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	LatestURL  string
	Token      string
	UserAgent  string
}

// Asset describes a downloadable GitHub release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Release describes a GitHub release and its downloadable assets.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Latest returns the latest release tag from GitHub.
func (c Client) Latest(ctx context.Context) (string, error) {
	release, err := c.LatestRelease(ctx)
	if err != nil {
		return "", err
	}
	return release.TagName, nil
}

// LatestRelease returns the latest release from GitHub.
func (c Client) LatestRelease(ctx context.Context) (Release, error) {
	url := c.LatestURL
	if url == "" {
		url = DefaultLatestURL
	}
	release, err := c.fetchRelease(ctx, url, "latest release")
	if err != nil {
		return Release{}, err
	}
	return release, nil
}

// ReleaseByTag returns a specific release selected by tag.
func (c Client) ReleaseByTag(ctx context.Context, tag string) (Release, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return Release{}, errors.New("release tag is required")
	}
	release, err := c.fetchRelease(ctx, c.releaseURL("/releases/tags/"+url.PathEscape(tag)), "release by tag")
	if err != nil {
		return Release{}, err
	}
	return release, nil
}

// FindAsset returns the asset with the exact name.
func FindAsset(assets []Asset, name string) (Asset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

// DownloadAsset downloads a release asset into w while enforcing maxBytes.
func (c Client) DownloadAsset(ctx context.Context, asset Asset, w io.Writer, maxBytes int64) (written int64, returnErr error) {
	if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
		return 0, fmt.Errorf("download asset %q: missing browser_download_url", asset.Name)
	}
	if maxBytes <= 0 {
		return 0, errors.New("download asset: maxBytes must be positive")
	}
	var buf strings.Builder
	counting := &limitBufferWriter{limit: maxBytes, dst: &buf}
	req, err := c.newRequest(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return 0, fmt.Errorf("create asset download request: %w", err)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, fmt.Errorf("download asset %q: %w", asset.Name, err)
	}
	defer func() {
		if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes)); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("drain asset response: %w", err))
		}
		if err := resp.Body.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close asset response: %w", err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download asset %q: unexpected status %s", asset.Name, resp.Status)
	}
	if _, err := io.Copy(counting, resp.Body); err != nil {
		return 0, fmt.Errorf("download asset %q: %w", asset.Name, err)
	}
	if _, err := io.Copy(w, strings.NewReader(buf.String())); err != nil {
		return 0, fmt.Errorf("write asset %q: %w", asset.Name, err)
	}
	return counting.written, nil
}

// ParseSHA256SUMS parses a SHA256SUMS file into filename-to-hex-digest entries.
func ParseSHA256SUMS(raw string) (map[string]string, error) {
	entries := map[string]string{}
	for lineNo, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("parse SHA256SUMS line %d: expected hash and filename", lineNo+1)
		}
		hash := strings.ToLower(fields[0])
		if len(hash) != 64 {
			return nil, fmt.Errorf("parse SHA256SUMS line %d: SHA256 hash must be 64 hex characters", lineNo+1)
		}
		for _, r := range hash {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				return nil, fmt.Errorf("parse SHA256SUMS line %d: SHA256 hash contains non-hex characters", lineNo+1)
			}
		}
		name := strings.TrimPrefix(fields[1], "*")
		name = strings.TrimPrefix(name, "./")
		if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
			return nil, fmt.Errorf("parse SHA256SUMS line %d: invalid filename %q", lineNo+1, fields[1])
		}
		if _, exists := entries[name]; exists {
			return nil, fmt.Errorf("parse SHA256SUMS line %d: duplicate filename %q", lineNo+1, name)
		}
		entries[name] = hash
	}
	if len(entries) == 0 {
		return nil, errors.New("parse SHA256SUMS: no entries")
	}
	return entries, nil
}

func (c Client) fetchRelease(ctx context.Context, rawURL, label string) (release Release, returnErr error) {
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return Release{}, fmt.Errorf("create %s request: %w", label, err)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetch %s: %w", label, err)
	}
	defer func() {
		if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes)); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("drain %s response: %w", label, err))
		}
		if err := resp.Body.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close %s response: %w", label, err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("fetch %s: unexpected status %s", label, resp.Status)
	}

	var payload Release
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := dec.Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("decode %s response: %w", label, err)
	}
	payload.TagName = strings.TrimSpace(payload.TagName)
	if payload.TagName == "" {
		return Release{}, fmt.Errorf("decode %s response: missing tag_name", label)
	}
	return payload, nil
}

func (c Client) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	userAgent := c.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c Client) releaseURL(path string) string {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	return base + path
}

type limitBufferWriter struct {
	limit   int64
	written int64
	dst     *strings.Builder
}

func (w *limitBufferWriter) Write(p []byte) (int, error) {
	if w.written+int64(len(p)) > w.limit {
		return 0, fmt.Errorf("response exceeds maximum size %d bytes", w.limit)
	}
	n, err := w.dst.Write(p)
	w.written += int64(n)
	if err != nil {
		return n, fmt.Errorf("buffer response: %w", err)
	}
	return n, nil
}

type semVersion struct {
	major string
	minor string
	patch string
	pre   []string
}

// CompareVersions compares two semantic versions after accepting an optional v-prefix.
func CompareVersions(a, b string) (int, error) {
	left, err := parseSemVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := parseSemVersion(b)
	if err != nil {
		return 0, err
	}
	return compareSemVersions(left, right), nil
}

// UpdateAvailable reports whether latest is newer than current.
func UpdateAvailable(current, latest string) bool {
	available, _ := UpdateAvailability(current, latest)
	return available
}

// UpdateAvailability compares current and latest and returns a status for callers.
func UpdateAvailability(current, latest string) (bool, string) {
	cmp, err := CompareVersions(current, latest)
	if err != nil {
		return false, UpdateCheckUncomparable
	}
	return cmp < 0, UpdateCheckOK
}

func parseSemVersion(raw string) (semVersion, error) {
	v := strings.TrimSpace(raw)
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		v = v[1:]
	}
	coreAndPre, build, hasBuild := strings.Cut(v, "+")
	if hasBuild {
		if err := validateBuildMetadata(raw, build); err != nil {
			return semVersion{}, err
		}
	}
	core, pre, hasPre := strings.Cut(coreAndPre, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semVersion{}, fmt.Errorf("parse semantic version %q: expected major.minor.patch", raw)
	}
	nums := make([]string, len(parts))
	for i, part := range parts {
		if part == "" {
			return semVersion{}, fmt.Errorf("parse semantic version %q: empty numeric component", raw)
		}
		if len(part) > 1 && part[0] == '0' {
			return semVersion{}, fmt.Errorf("parse semantic version %q: numeric component %q has leading zero", raw, part)
		}
		normalized, ok := normalizeNumericIdentifier(part)
		if !ok {
			return semVersion{}, fmt.Errorf("parse semantic version %q: numeric component %q is not numeric", raw, part)
		}
		nums[i] = normalized
	}
	parsed := semVersion{major: nums[0], minor: nums[1], patch: nums[2]}
	if hasPre {
		if pre == "" {
			return semVersion{}, fmt.Errorf("parse semantic version %q: empty prerelease component", raw)
		}
		parsed.pre = strings.Split(pre, ".")
		for _, identifier := range parsed.pre {
			if err := validatePrereleaseIdentifier(raw, identifier); err != nil {
				return semVersion{}, err
			}
		}
	}
	return parsed, nil
}

func validateBuildMetadata(raw, build string) error {
	if build == "" {
		return fmt.Errorf("parse semantic version %q: empty build metadata", raw)
	}
	for _, identifier := range strings.Split(build, ".") {
		if identifier == "" {
			return fmt.Errorf("parse semantic version %q: empty build metadata identifier", raw)
		}
		for _, r := range identifier {
			if !isPrereleaseIdentifierChar(r) {
				return fmt.Errorf("parse semantic version %q: invalid build metadata identifier %q", raw, identifier)
			}
		}
	}
	return nil
}

func validatePrereleaseIdentifier(raw, identifier string) error {
	if identifier == "" {
		return fmt.Errorf("parse semantic version %q: empty prerelease identifier", raw)
	}
	for _, r := range identifier {
		if !isPrereleaseIdentifierChar(r) {
			return fmt.Errorf("parse semantic version %q: invalid prerelease identifier %q", raw, identifier)
		}
	}
	if _, numeric := normalizeNumericIdentifier(identifier); numeric && len(identifier) > 1 && identifier[0] == '0' {
		return fmt.Errorf("parse semantic version %q: numeric prerelease identifier %q has leading zero", raw, identifier)
	}
	return nil
}

func isPrereleaseIdentifierChar(r rune) bool {
	return r == '-' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func compareSemVersions(a, b semVersion) int {
	if cmp := compareNumericStrings(a.major, b.major); cmp != 0 {
		return cmp
	}
	if cmp := compareNumericStrings(a.minor, b.minor); cmp != 0 {
		return cmp
	}
	if cmp := compareNumericStrings(a.patch, b.patch); cmp != 0 {
		return cmp
	}
	return comparePrerelease(a.pre, b.pre)
}

func comparePrerelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if cmp := comparePrereleaseIdentifier(a[i], b[i]); cmp != 0 {
			return cmp
		}
	}
	return compareInt(len(a), len(b))
}

func comparePrereleaseIdentifier(a, b string) int {
	aNum, aIsNum := normalizeNumericIdentifier(a)
	bNum, bIsNum := normalizeNumericIdentifier(b)
	switch {
	case aIsNum && bIsNum:
		return compareNumericStrings(aNum, bNum)
	case aIsNum:
		return -1
	case bIsNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func normalizeNumericIdentifier(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	trimmed := strings.TrimLeft(s, "0")
	if trimmed == "" {
		trimmed = "0"
	}
	return trimmed, true
}

func compareNumericStrings(a, b string) int {
	if cmp := compareInt(len(a), len(b)); cmp != 0 {
		return cmp
	}
	return strings.Compare(a, b)
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
