// Package release looks up and compares published backlogit releases.
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// DefaultLatestURL is the GitHub Releases endpoint for the latest backlogit release.
	DefaultLatestURL = "https://api.github.com/repos/softwaresalt/backlogit/releases/latest"

	// UpdateCheckOK means the latest version was fetched and compared successfully.
	UpdateCheckOK = "ok"
	// UpdateCheckUncomparable means the latest version was fetched but comparison was not possible.
	UpdateCheckUncomparable = "uncomparable"

	defaultUserAgent = "backlogit"
	maxResponseBytes = 1 << 20
)

// Client queries the latest backlogit release.
type Client struct {
	HTTPClient *http.Client
	LatestURL  string
	Token      string
	UserAgent  string
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

// Latest returns the latest release tag from GitHub.
func (c Client) Latest(ctx context.Context) (tag string, returnErr error) {
	url := c.LatestURL
	if url == "" {
		url = DefaultLatestURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create latest release request: %w", err)
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

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() {
		if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes)); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("drain latest release response: %w", err))
		}
		if err := resp.Body.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close latest release response: %w", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch latest release: unexpected status %s", resp.Status)
	}

	var payload latestReleaseResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := dec.Decode(&payload); err != nil {
		return "", fmt.Errorf("decode latest release response: %w", err)
	}
	tag = strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", errors.New("decode latest release response: missing tag_name")
	}
	return tag, nil
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
