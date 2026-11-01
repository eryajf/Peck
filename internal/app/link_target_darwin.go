//go:build darwin

package app

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eryajf/Peck/internal/core"
)

type linkCandidate struct {
	path string
	rank int
}

// findLinkTargets uses the local application index as a hint resolver. It
// never follows a link-provided path and does not modify or launch a result.
func findLinkTargets(parent context.Context, link core.LinkRequest) []string {
	if link.BundleID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 2500*time.Millisecond)
	defer cancel()

	// ParseLink already restricts bundle IDs to this character set. Keep the
	// check here too because this helper is also used directly by tests.
	if strings.Trim(link.BundleID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-") != "" {
		return nil
	}
	query := `kMDItemContentType == "com.apple.application-bundle" && kMDItemCFBundleIdentifier == "` + link.BundleID + `"`
	output, err := runMetadataCommand(ctx, "/usr/bin/mdfind", "-0", query)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	candidates := make([]linkCandidate, 0, 4)
	for _, line := range strings.Split(string(output), "\x00") {
		path := filepath.Clean(line)
		if path == "." || seen[path] || !filepath.IsAbs(path) || !isApplicationPath(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		if !bundleMatchesLink(ctx, path, link) {
			continue
		}
		seen[path] = true
		candidates = append(candidates, linkCandidate{path: path, rank: linkCandidateRank(path)})
	}
	// A partial lookup must not turn an unexamined duplicate into a unique match.
	if ctx.Err() != nil {
		return nil
	}
	return selectBestLinkTargets(candidates)
}

func runMetadataCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C"}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd.Output()
}

func bundleMatchesLink(ctx context.Context, path string, link core.LinkRequest) bool {
	plist := filepath.Join(path, "Contents", "Info.plist")
	// Spotlight may contain an old identity after an application was replaced.
	output, err := runMetadataCommand(ctx, "/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", plist)
	if err != nil || strings.TrimSuffix(string(output), "\n") != link.BundleID {
		return false
	}
	if link.Release == "" {
		return true
	}
	for _, key := range []string{"CFBundleShortVersionString", "CFBundleVersion"} {
		output, err := runMetadataCommand(ctx, "/usr/bin/plutil", "-extract", key, "raw", "-o", "-", plist)
		if err == nil && strings.TrimSuffix(string(output), "\n") == link.Release {
			return true
		}
	}
	return false
}

func linkCandidateRank(path string) int {
	path = filepath.Clean(path)
	if path == "/Applications" || strings.HasPrefix(path, "/Applications/") {
		return 0
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		for rank, directory := range []string{
			filepath.Join(home, "Applications"),
			filepath.Join(home, "Downloads"),
			filepath.Join(home, "Desktop"),
		} {
			if path == directory || strings.HasPrefix(path, directory+string(os.PathSeparator)) {
				return rank + 1
			}
		}
	}
	return 10
}

func selectBestLinkTargets(candidates []linkCandidate) []string {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank < candidates[j].rank
		}
		return candidates[i].path < candidates[j].path
	})
	if len(candidates) == 0 {
		return nil
	}
	bestRank := candidates[0].rank
	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.rank != bestRank {
			break
		}
		paths = append(paths, candidate.path)
	}
	return paths
}
