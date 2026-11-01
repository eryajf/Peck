package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	downloadBufferSize = 128 * 1024
	maxAssetSize       = 1 << 30
	downloadTimeout    = 5 * time.Minute
	maxChecksumBytes   = 1 << 20
)

type Progress struct {
	Received int64
	Total    int64
	Speed    int64
}

func Download(ctx context.Context, result Result, dir string, client *http.Client, report func(Progress)) (string, error) {
	asset := result.Release.Asset
	if result.Comparison != ComparisonUpdateAvailable || asset == nil {
		return "", errors.New("no downloadable update is available")
	}
	if !validVersion(result.CurrentVersion) || !validVersion(result.LatestVersion) || compare(result.CurrentVersion, result.LatestVersion) != ComparisonUpdateAvailable {
		return "", errors.New("update result is stale or has an invalid version")
	}
	if err := validateDownloadURL(asset.DownloadURL); err != nil {
		return "", err
	}
	if asset.ChecksumURL == "" {
		return "", errors.New("release has no checksum manifest")
	}
	if err := validateDownloadURL(asset.ChecksumURL); err != nil {
		return "", err
	}
	if filepath.Base(asset.Name) != asset.Name || !strings.HasSuffix(strings.ToLower(asset.Name), SupportedAsset) {
		return "", errors.New("release has an invalid update package name")
	}
	if asset.Size <= 0 || asset.Size > maxAssetSize {
		return "", fmt.Errorf("release has an invalid update package size: %d", asset.Size)
	}
	if client == nil {
		client = http.DefaultClient
	}
	client = safeDownloadClient(client)
	downloadCtx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	checksum, err := fetchChecksum(downloadCtx, client, asset.ChecksumURL, asset.Name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	finalPath := filepath.Join(dir, asset.Name)
	tempDir, err := os.MkdirTemp(dir, ".download-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	request, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, asset.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "peck-updater/"+result.CurrentVersion)
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("update download returned %s", response.Status)
	}
	total := asset.Size
	if response.ContentLength > 0 && response.ContentLength <= asset.Size {
		total = response.ContentLength
	}
	file, err := os.CreateTemp(tempDir, "asset-*")
	if err != nil {
		return "", err
	}
	tempPath := file.Name()
	defer file.Close()

	hash := sha256.New()
	buffer := make([]byte, downloadBufferSize)
	var received int64
	lastAt := time.Now()
	lastBytes := int64(0)
	limitedBody := io.LimitReader(response.Body, asset.Size+1)
	for {
		count, readErr := limitedBody.Read(buffer)
		if count > 0 {
			if received+int64(count) > asset.Size {
				return "", fmt.Errorf("update package exceeds declared size of %d bytes", asset.Size)
			}
			if _, err := file.Write(buffer[:count]); err != nil {
				return "", err
			}
			_, _ = hash.Write(buffer[:count])
			received += int64(count)
			now := time.Now()
			if report != nil && now.Sub(lastAt) >= 300*time.Millisecond {
				report(Progress{Received: received, Total: total, Speed: int64(float64(received-lastBytes) / now.Sub(lastAt).Seconds())})
				lastAt, lastBytes = now, received
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", readErr
		}
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if received != asset.Size {
		return "", fmt.Errorf("update package size is %d bytes, want %d", received, asset.Size)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != checksum {
		return "", fmt.Errorf("update checksum mismatch: got %s", got)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", err
	}
	if report != nil {
		report(Progress{Received: received, Total: total})
	}
	return finalPath, nil
}

func fetchChecksum(ctx context.Context, client *http.Client, rawURL, assetName string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "peck-updater")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("checksum manifest returned %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxChecksumBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxChecksumBytes {
		return "", errors.New("checksum manifest is too large")
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || filepath.Base(strings.TrimPrefix(fields[1], "*")) != assetName {
			continue
		}
		value := strings.ToLower(fields[0])
		if len(value) == sha256.Size*2 {
			if _, err := hex.DecodeString(value); err == nil {
				return value, nil
			}
		}
	}
	return "", fmt.Errorf("checksum for %s was not found", assetName)
}

func safeDownloadClient(client *http.Client) *http.Client {
	copy := *client
	redirect := client.CheckRedirect
	copy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many update redirects")
		}
		if err := validateRedirectURL(request.URL); err != nil {
			return fmt.Errorf("unsafe update redirect: %w", err)
		}
		if redirect != nil {
			if err := redirect(request, via); err != nil {
				return err
			}
		}
		return validateRedirectURL(request.URL)
	}
	return &copy
}

func validateRedirectURL(parsed *url.URL) error {
	if parsed.Scheme != "https" || parsed.User != nil {
		return errors.New("update redirect must use HTTPS")
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "github.com", "cnb.cool", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return nil
	default:
		return fmt.Errorf("update redirect host is not allowed: %q", parsed.Hostname())
	}
}

func validateDownloadURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return fmt.Errorf("update URL must use HTTPS: %q", rawURL)
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "github.com", "cnb.cool":
		return nil
	default:
		return fmt.Errorf("update URL host is not allowed: %q", parsed.Hostname())
	}
}
