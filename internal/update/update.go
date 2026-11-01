// Package update checks Peck's published releases without changing the local
// installation. Downloading and applying an update are separate operations.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/eryajf/Peck/internal/version"
)

const (
	SourceGitHub = "github"
	SourceCNB    = "cnb"
)

const (
	ComparisonUpdateAvailable = "update_available"
	ComparisonUpToDate        = "up_to_date"
	ComparisonLocalNewer      = "local_newer"
	ComparisonUncomparable    = "uncomparable"
)

const (
	githubLatestReleaseURL = "https://api.github.com/repos/eryajf/Peck/releases/latest"
	githubReleasePrefix    = "https://github.com/eryajf/Peck/"
	cnbReleasePrefix       = "https://cnb.cool/znb/Peck/-/"
	maxReleaseBytes        = 1 << 20
	maxNotesBytes          = 16 << 10
	checkTimeout           = 5 * time.Second
)

var httpClient = http.DefaultClient

// Release is the latest published release and the optional package for this
// machine.
type Release struct {
	Version     string
	URL         string
	Notes       string
	PublishedAt string
	Asset       *Asset
}

type Asset struct {
	Name        string
	DownloadURL string
	ChecksumURL string
	Size        int64
}

type Result struct {
	CurrentVersion string
	LatestVersion  string
	Comparison     string
	Release        Release
	CheckedAt      time.Time
}

func NormalizeSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SourceCNB:
		return SourceCNB
	default:
		return SourceGitHub
	}
}

func Check(ctx context.Context, currentVersion, source string, force bool) (Result, error) {
	_ = force // reserved for a future process-wide cache; every call is currently a network check.
	current := version.Normalize(currentVersion)
	if current == "" || strings.EqualFold(current, "dev") {
		return Result{CurrentVersion: current, Comparison: ComparisonUncomparable, CheckedAt: time.Now()}, nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "peck-updater/"+current)
	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("request latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("latest release returned %s", resp.Status)
	}

	var payload githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseBytes)).Decode(&payload); err != nil {
		return Result{}, fmt.Errorf("decode latest release: %w", err)
	}
	if payload.Draft || payload.Prerelease {
		return Result{}, errors.New("latest release is not a stable release")
	}
	latest := version.Normalize(payload.TagName)
	if !validVersion(latest) {
		return Result{}, fmt.Errorf("invalid latest release version %q", payload.TagName)
	}
	if !strings.HasPrefix(payload.HTMLURL, "https://") {
		return Result{}, errors.New("latest release URL is not HTTPS")
	}

	result := Result{
		CurrentVersion: current,
		LatestVersion:  latest,
		Release: Release{
			Version:     latest,
			URL:         payload.HTMLURL,
			Notes:       cutNotes(payload.Body),
			PublishedAt: strings.TrimSpace(payload.PublishedAt),
			Asset:       matchAsset(payload.Assets, runtime.GOOS, runtime.GOARCH),
		},
		CheckedAt: time.Now(),
	}
	result.Comparison = compare(current, latest)
	result.Release.URL = RewriteURL(result.Release.URL, source)
	if result.Release.Asset != nil {
		asset := *result.Release.Asset
		asset.DownloadURL = RewriteURL(asset.DownloadURL, source)
		asset.ChecksumURL = RewriteURL(asset.ChecksumURL, source)
		result.Release.Asset = &asset
	}
	return result, nil
}

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	HTMLURL     string               `json:"html_url"`
	Body        string               `json:"body"`
	PublishedAt string               `json:"published_at"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CompareVersions compares stable release versions; malformed versions are not comparable.
func CompareVersions(current, latest string) string { return compare(current, latest) }

func compare(current, latest string) string {
	local, localOK := parseVersion(current)
	remote, remoteOK := parseVersion(latest)
	if !localOK || !remoteOK {
		return ComparisonUncomparable
	}
	for index := range local {
		if remote[index] > local[index] {
			return ComparisonUpdateAvailable
		}
		if remote[index] < local[index] {
			return ComparisonLocalNewer
		}
	}
	return ComparisonUpToDate
}

func validVersion(value string) bool {
	_, ok := parseVersion(value)
	return ok
}

func parseVersion(value string) ([3]int, bool) {
	var parsed [3]int
	parts := strings.Split(version.Normalize(value), ".")
	if len(parts) != len(parsed) {
		return parsed, false
	}
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return parsed, false
		}
		parsed[index] = number
	}
	return parsed, true
}

func matchAsset(assets []githubReleaseAsset, goos, goarch string) *Asset {
	if goos != SupportedGOOS || (goarch != "arm64" && goarch != "amd64") {
		return nil
	}
	archTokens := map[string][]string{
		"arm64": {"arm64", "aarch64", "apple-silicon"},
		"amd64": {"amd64", "x86_64", "x64", "intel"},
	}
	for _, candidate := range assets {
		name := strings.ToLower(strings.TrimSpace(candidate.Name))
		if !strings.HasSuffix(name, SupportedAsset) || !strings.Contains(name, "darwin") && !strings.Contains(name, "macos") {
			continue
		}
		if !slices.ContainsFunc(archTokens[goarch], func(token string) bool { return strings.Contains(name, token) }) {
			continue
		}
		if !strings.HasPrefix(candidate.BrowserDownloadURL, "https://") {
			continue
		}
		checksumURL := ""
		for _, checksum := range assets {
			checksumName := strings.ToLower(strings.TrimSpace(checksum.Name))
			if checksumName == "sha256sums" || checksumName == "sha256sums.txt" || checksumName == "checksums.txt" {
				checksumURL = checksum.BrowserDownloadURL
				break
			}
		}
		return &Asset{Name: candidate.Name, DownloadURL: candidate.BrowserDownloadURL, ChecksumURL: checksumURL, Size: candidate.Size}
	}
	return nil
}

func RewriteURL(rawURL, source string) string {
	switch NormalizeSource(source) {
	case SourceCNB:
		if strings.HasPrefix(rawURL, githubReleasePrefix) {
			return cnbReleasePrefix + strings.TrimPrefix(rawURL, githubReleasePrefix)
		}
	case SourceGitHub:
		if strings.HasPrefix(rawURL, cnbReleasePrefix) {
			return githubReleasePrefix + strings.TrimPrefix(rawURL, cnbReleasePrefix)
		}
	}
	return rawURL
}

func cutNotes(notes string) string {
	if len(notes) <= maxNotesBytes {
		return notes
	}
	return notes[:maxNotesBytes]
}
