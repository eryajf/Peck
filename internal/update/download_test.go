package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDownloadVerifiesChecksumAndRenamesPart(t *testing.T) {
	payload := []byte("verified update zip")
	sum := sha256.Sum256(payload)
	checksum := hex.EncodeToString(sum[:])
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := checksum + "  Peck-0.2.0-darwin-arm64.zip\n"
		if strings.HasSuffix(request.URL.Path, "SHA256SUMS") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(payload)), Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
	})}
	result := Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", Comparison: ComparisonUpdateAvailable, Release: Release{Asset: &Asset{Name: "Peck-0.2.0-darwin-arm64.zip", DownloadURL: "https://evil.example/asset", ChecksumURL: "https://evil.example/SHA256SUMS", Size: int64(len(payload))}}}
	path, err := Download(context.Background(), result, t.TempDir(), client, nil)
	if err == nil || path != "" {
		t.Fatalf("unexpected host validation result: path=%q err=%v", path, err)
	}

	result.Release.Asset.DownloadURL = "https://github.com/eryajf/Peck/releases/download/v0.2.0/Peck-0.2.0-darwin-arm64.zip"
	result.Release.Asset.ChecksumURL = "https://github.com/eryajf/Peck/releases/download/v0.2.0/SHA256SUMS"
	path, err = Download(context.Background(), result, t.TempDir(), client, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != result.Release.Asset.Name {
		t.Fatalf("download path = %q", path)
	}
}

func TestDownloadDoesNotFollowFixedPartSymlink(t *testing.T) {
	payload := []byte("safe update")
	sum := sha256.Sum256(payload)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "SHA256SUMS") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(hex.EncodeToString(sum[:]) + "  Peck.zip\n"))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(payload)), Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
	})}
	result := Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", Comparison: ComparisonUpdateAvailable, Release: Release{Asset: &Asset{Name: "Peck.zip", DownloadURL: "https://github.com/eryajf/Peck/a.zip", ChecksumURL: "https://github.com/eryajf/Peck/SHA256SUMS", Size: int64(len(payload))}}}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(target, []byte("must remain unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	partPath := filepath.Join(dir, "Peck.zip.part")
	if err := os.Symlink(target, partPath); err != nil {
		t.Fatal(err)
	}
	if _, err := Download(context.Background(), result, dir, client, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "must remain unchanged" {
		t.Fatalf("fixed part symlink target was modified: %q", got)
	}
}

func TestDownloadRejectsOversizedPayloadAndRemovesPart(t *testing.T) {
	payload := []byte("too large")
	sum := sha256.Sum256(payload)
	checksum := hex.EncodeToString(sum[:])
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "SHA256SUMS") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(checksum + "  Peck.zip\n"))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
	})}
	result := Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", Comparison: ComparisonUpdateAvailable, Release: Release{Asset: &Asset{Name: "Peck.zip", DownloadURL: "https://github.com/eryajf/Peck/a.zip", ChecksumURL: "https://github.com/eryajf/Peck/SHA256SUMS", Size: 3}}}
	dir := t.TempDir()
	if _, err := Download(context.Background(), result, dir, client, nil); err == nil || !strings.Contains(err.Error(), "exceeds declared size") {
		t.Fatalf("expected oversized payload error, got %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(matches) != 0 {
		t.Fatalf("partial files remain: %v", matches)
	}
}

func TestDownloadRejectsInvalidAssetSizes(t *testing.T) {
	for _, size := range []int64{0, -1, maxAssetSize + 1} {
		t.Run(strconv.FormatInt(size, 10), func(t *testing.T) {
			result := Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", Comparison: ComparisonUpdateAvailable, Release: Release{Asset: &Asset{Name: "Peck.zip", DownloadURL: "https://github.com/eryajf/Peck/a.zip", ChecksumURL: "https://github.com/eryajf/Peck/SHA256SUMS", Size: size}}}
			if _, err := Download(context.Background(), result, t.TempDir(), nil, nil); err == nil || !strings.Contains(err.Error(), "invalid update package size") {
				t.Fatalf("expected invalid size error, got %v", err)
			}
		})
	}
}

func TestUpdateRedirectHosts(t *testing.T) {
	client := safeDownloadClient(&http.Client{})
	for _, tt := range []struct {
		address string
		want    bool
	}{
		{"https://release-assets.githubusercontent.com/file", true},
		{"https://objects.githubusercontent.com/file", true},
		{"http://release-assets.githubusercontent.com/file", false},
		{"https://evil.example/file", false},
	} {
		req, err := http.NewRequest(http.MethodGet, tt.address, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = client.CheckRedirect(req, nil)
		if (err == nil) != tt.want {
			t.Errorf("redirect %s: %v", tt.address, err)
		}
	}
}

func TestRedirectCallbackCannotBypassHostValidation(t *testing.T) {
	base := &http.Client{CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		req.URL, _ = url.Parse("https://evil.example/file")
		return nil
	}}
	req, _ := http.NewRequest(http.MethodGet, "https://release-assets.githubusercontent.com/file", nil)
	if err := safeDownloadClient(base).CheckRedirect(req, nil); err == nil {
		t.Fatal("redirect callback bypassed validation")
	}
}

func TestDownloadCancellationRemovesPart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "SHA256SUMS") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(strings.Repeat("0", 64) + "  Peck.zip\n"))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: &cancelReader{cancel: cancel, payload: []byte("payload")}}, nil
	})}
	result := Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", Comparison: ComparisonUpdateAvailable, Release: Release{Asset: &Asset{Name: "Peck.zip", DownloadURL: "https://github.com/eryajf/Peck/a.zip", ChecksumURL: "https://github.com/eryajf/Peck/SHA256SUMS", Size: 7}}}
	dir := t.TempDir()
	if _, err := Download(ctx, result, dir, client, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if entries, err := os.ReadDir(dir); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("temporary download files remain: %v", entries)
	}
}

type cancelReader struct {
	cancel  context.CancelFunc
	payload []byte
	read    bool
}

func (reader *cancelReader) Read(target []byte) (int, error) {
	if reader.read {
		return 0, context.Canceled
	}
	reader.read = true
	copy(target, reader.payload)
	reader.cancel()
	return len(reader.payload), context.Canceled
}

func (reader *cancelReader) Close() error { return nil }

func TestDownloadRemovesPartOnChecksumMismatch(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "SHA256SUMS") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(strings.Repeat("0", 64) + "  Peck.zip\n"))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("bad"))}, nil
	})}
	result := Result{CurrentVersion: "0.1.0", LatestVersion: "0.2.0", Comparison: ComparisonUpdateAvailable, Release: Release{Asset: &Asset{Name: "Peck.zip", DownloadURL: "https://github.com/eryajf/Peck/a.zip", ChecksumURL: "https://github.com/eryajf/Peck/SHA256SUMS", Size: 3}}}
	dir := t.TempDir()
	if _, err := Download(context.Background(), result, dir, client, nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(matches) != 0 {
		t.Fatalf("partial files remain: %v", matches)
	}
}
