//go:build darwin

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/eryajf/Peck/internal/core"
)

func TestBundleMatchesLinkChecksCurrentIdentityAndVersion(t *testing.T) {
	const identifier = "org.example.app"
	const metadata = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>org.example.app</string>
<key>CFBundleShortVersionString</key><string>1.2.3</string>
<key>CFBundleVersion</key><string>42</string>
<key>ExtraData</key><data>AQIDBA==</data>
</dict></plist>`
	for _, binary := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "Example.app")
		if err := os.MkdirAll(filepath.Join(path, "Contents"), 0700); err != nil {
			t.Fatal(err)
		}
		plist := filepath.Join(path, "Contents", "Info.plist")
		if err := os.WriteFile(plist, []byte(metadata), 0600); err != nil {
			t.Fatal(err)
		}
		if binary {
			if _, err := runMetadataCommand(context.Background(), "/usr/bin/plutil", "-convert", "binary1", plist); err != nil {
				t.Fatal(err)
			}
		}
		for _, test := range []struct {
			name, bundleID, release string
			want                    bool
		}{
			{"identity only", identifier, "", true},
			{"release version", identifier, "1.2.3", true},
			{"build version", identifier, "42", true},
			{"different release", identifier, "1.2.4", false},
			{"stale indexed identity", "org.example.previous", "1.2.3", false},
			{"stale identity without release", "org.example.previous", "", false},
		} {
			link := core.LinkRequest{BundleID: test.bundleID, Release: test.release}
			if got := bundleMatchesLink(context.Background(), path, link); got != test.want {
				t.Errorf("%s (binary %t): got %t, want %t", test.name, binary, got, test.want)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if bundleMatchesLink(ctx, path, core.LinkRequest{BundleID: identifier}) {
			t.Fatal("cancelled lookup matched a candidate")
		}
		if err := os.WriteFile(plist, []byte("invalid plist"), 0600); err != nil {
			t.Fatal(err)
		}
		if bundleMatchesLink(context.Background(), path, core.LinkRequest{BundleID: identifier}) {
			t.Fatal("invalid metadata matched a candidate")
		}
	}
}

func TestSelectBestLinkTargetsPrefersInstalledApplications(t *testing.T) {
	got := selectBestLinkTargets([]linkCandidate{
		{path: "/tmp/Peck/build/小清.app", rank: linkCandidateRank("/tmp/Peck/build/小清.app")},
		{path: "/Applications/小清.app", rank: linkCandidateRank("/Applications/小清.app")},
	})
	if len(got) != 1 || got[0] != "/Applications/小清.app" {
		t.Fatalf("got %v, want the installed application", got)
	}
}

func TestSelectBestLinkTargetsKeepsAmbiguousEqualRankMatches(t *testing.T) {
	got := selectBestLinkTargets([]linkCandidate{
		{path: "/Applications/One/小清.app", rank: linkCandidateRank("/Applications/One/小清.app")},
		{path: "/Applications/Two/小清.app", rank: linkCandidateRank("/Applications/Two/小清.app")},
		{path: "/tmp/小清.app", rank: linkCandidateRank("/tmp/小清.app")},
	})
	if len(got) != 2 || got[0] != "/Applications/One/小清.app" || got[1] != "/Applications/Two/小清.app" {
		t.Fatalf("got %v, want both equally preferred applications", got)
	}
}
