package update

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct {
		current, latest, want string
	}{
		{"0.1.0", "0.2.0", ComparisonUpdateAvailable},
		{"0.2.0", "0.2.0", ComparisonUpToDate},
		{"0.3.0", "0.2.0", ComparisonLocalNewer},
		{"dev", "0.2.0", ComparisonUncomparable},
	} {
		if got := compare(test.current, test.latest); got != test.want {
			t.Errorf("compare(%q, %q) = %q, want %q", test.current, test.latest, got, test.want)
		}
	}
}

func TestMatchAssetForBothArchitectures(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "Peck-0.2.0-darwin-arm64.zip", BrowserDownloadURL: "https://github.com/eryajf/Peck/releases/download/v0.2.0/arm64.zip", Size: 10},
		{Name: "Peck-0.2.0-darwin-amd64.zip", BrowserDownloadURL: "https://github.com/eryajf/Peck/releases/download/v0.2.0/amd64.zip", Size: 12},
	}
	for _, arch := range []string{"arm64", "amd64"} {
		asset := matchAsset(assets, "darwin", arch)
		if asset == nil || !strings.Contains(asset.Name, arch) {
			t.Fatalf("%s: %#v", arch, asset)
		}
	}
}

func TestRewriteURL(t *testing.T) {
	github := "https://github.com/eryajf/Peck/releases/tag/v0.2.0"
	cnb := "https://cnb.cool/znb/Peck/-/releases/tag/v0.2.0"
	if got := RewriteURL(github, SourceCNB); got != cnb {
		t.Fatalf("CNB URL = %q, want %q", got, cnb)
	}
	if got := RewriteURL(cnb, SourceGitHub); got != github {
		t.Fatalf("GitHub URL = %q, want %q", got, github)
	}
}

func TestCheckUsesReleaseMetadataAndSelectedSource(t *testing.T) {
	oldClient := httpClient
	httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"tag_name":"v0.2.0","html_url":"https://github.com/eryajf/Peck/releases/tag/v0.2.0","body":"notes","published_at":"2026-05-01T00:00:00Z","assets":[{"name":"Peck-0.2.0-darwin-arm64.zip","browser_download_url":"https://github.com/eryajf/Peck/releases/download/v0.2.0/Peck-0.2.0-darwin-arm64.zip","size":42}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	t.Cleanup(func() { httpClient = oldClient })

	result, err := Check(context.Background(), "0.1.0", SourceCNB, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Comparison != ComparisonUpdateAvailable || result.Release.URL != "https://cnb.cool/znb/Peck/-/releases/tag/v0.2.0" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if result.Release.Asset == nil || !strings.HasPrefix(result.Release.Asset.DownloadURL, "https://cnb.cool/znb/Peck/-/") {
			t.Fatalf("unexpected asset: %#v", result.Release.Asset)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
