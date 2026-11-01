package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// These fixtures are built in t.TempDir. They never inspect or mutate installed
// applications. The tiny executable returns immediately without doing work.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	app := filepath.Join(dir, "PeckFixture.app")
	macos := filepath.Join(app, "Contents/MacOS")
	if err := os.MkdirAll(macos, 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "main.c")
	if err := os.WriteFile(src, []byte("int main(void) { return 0; }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/usr/bin/clang", "-Wl,-no_adhoc_codesign", "-o", filepath.Join(macos, "fixture"), src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile harmless fixture: %v %s", err, out)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleExecutable</key><string>fixture</string><key>CFBundleIdentifier</key><string>org.example.peck.fixture</string><key>CFBundleName</key><string>Peck Fixture</string><key>CFBundleDisplayName</key><string>Peck Fixture</string><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleShortVersionString</key><string>1.0</string><key>CFBundleVersion</key><string>1</string><key>LSMinimumSystemVersion</key><string>12.0</string><key>CFBundleSupportedPlatforms</key><array><string>MacOSX</string></array></dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents/Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestInspectAndQuarantineOnly(t *testing.T) {
	app := fixture(t)
	main := filepath.Join(app, "Contents/MacOS/fixture")
	for _, p := range []string{app, main} {
		if err := unix.Setxattr(p, QuarantineAttribute, []byte("0081;12345678;PeckTest;"), 0); err != nil {
			t.Fatal(err)
		}
		if err := unix.Setxattr(p, "org.example.peck.keep", []byte("keep-me"), 0); err != nil {
			t.Fatal(err)
		}
	}
	i, err := Inspect(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	out, assessErr := systemCommand(context.Background(), "/usr/sbin/spctl", "--assess", "--type", "execute", "--verbose=4", app)
	t.Logf("spctl fixture diagnostic: %s (%v)", out, assessErr)
	if i.SignatureStatus != "unsigned" || i.QuarantineCount != 2 || i.BlockedReason != "" {
		t.Fatalf("inspection: %+v", i)
	}
	r := Process(context.Background(), i, Options{}, nil)
	if r.Error != nil || r.Changed != 2 || !r.CanOpen || r.Signature != "未请求" {
		t.Fatalf("result: %+v", r)
	}
	for _, p := range []string{app, main} {
		if _, err := unix.Getxattr(p, QuarantineAttribute, nil); !errors.Is(err, unix.ENOATTR) {
			t.Fatalf("quarantine remains: %v", err)
		}
		v := make([]byte, 100)
		n, err := unix.Getxattr(p, "org.example.peck.keep", v)
		if err != nil || string(v[:n]) != "keep-me" {
			t.Fatalf("unrelated xattr changed: %v", err)
		}
	}
	if status, _ := signature(context.Background(), app); status != "unsigned" {
		t.Fatal("quarantine-only operation signed the app")
	}
}

func TestChangedTargetAndCancellation(t *testing.T) {
	app := fixture(t)
	if err := unix.Setxattr(app, QuarantineAttribute, []byte("0081;12345678;PeckTest;"), 0); err != nil {
		t.Fatal(err)
	}
	i, err := Inspect(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents/new"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	r := Process(context.Background(), i, Options{}, nil)
	if r.Error == nil || r.Changed != 0 || r.CanOpen {
		t.Fatalf("changed app processed: %+v", r)
	}
	i, err = Inspect(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = Process(ctx, i, Options{}, nil)
	if !r.Cancelled || r.Changed != 0 || r.CanOpen {
		t.Fatalf("cancelled app processed: %+v", r)
	}
}

func TestLinksAndHardlinksRejected(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			app := fixture(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("untouched"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := unix.Setxattr(outside, QuarantineAttribute, []byte("test"), 0); err != nil {
				t.Fatal(err)
			}
			inside := filepath.Join(app, "Contents/link")
			var err error
			if kind == "symlink" {
				err = os.Symlink(outside, inside)
			} else {
				err = os.Link(outside, inside)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Inspect(context.Background(), app); err == nil {
				t.Fatal("link accepted")
			}
			if _, err := unix.Getxattr(outside, QuarantineAttribute, nil); err != nil {
				t.Fatal("outside file modified")
			}
		})
	}
}

func TestAdhocSigningAndNoOverwrite(t *testing.T) {
	app := fixture(t)
	if err := unix.Setxattr(app, QuarantineAttribute, []byte("0081;12345678;PeckTest;"), 0); err != nil {
		t.Fatal(err)
	}
	i, err := Inspect(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	if !i.CanSign {
		out, e := systemCommand(context.Background(), "/usr/sbin/spctl", "--assess", "--type", "execute", "--verbose=4", app)
		t.Logf("spctl diagnostic: %s (%v)", out, e)
		t.Fatalf("simple fixture cannot sign: %s (%s)", i.SignReason, i.BlockedReason)
	}
	r := Process(context.Background(), i, Options{Sign: true}, nil)
	if r.Error != nil || r.Changed < 1 || !r.CanOpen {
		t.Fatalf("sign failed: %+v", r)
	}
	post, err := capture(context.Background(), r.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range post.Entries {
		if _, ok := e.Attrs[QuarantineAttribute]; ok {
			t.Fatalf("quarantine retained on %s", e.Path)
		}
	}
	if status, reason := signature(context.Background(), app); status != "adhoc" || reason != "" {
		t.Fatalf("signature %s %s", status, reason)
	}
	i, err = Inspect(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	if i.CanSign {
		t.Fatal("already signed app allows overwrite")
	}
	r = Process(context.Background(), i, Options{Sign: true}, nil)
	if r.Error == nil || r.CanOpen {
		t.Fatalf("already signed app overwritten: %+v", r)
	}
}

func TestProcessSnapshotCannotBeRedirected(t *testing.T) {
	app := fixture(t)
	if err := unix.Setxattr(app, QuarantineAttribute, []byte("0081;12345678;PeckTest;"), 0); err != nil {
		t.Fatal(err)
	}
	i, err := Inspect(context.Background(), app)
	if err != nil {
		t.Fatal(err)
	}
	i.Path = "/Applications/Unrelated.app"
	i.BlockedReason = ""
	r := Process(context.Background(), i, Options{}, nil)
	expectedPath, _ := filepath.EvalSymlinks(app)
	if r.Error != nil || r.Path != expectedPath || r.Changed != 1 {
		t.Fatalf("snapshot redirected: %+v", r)
	}
	i.created = time.Now().Add(-11 * time.Minute)
	r = Process(context.Background(), i, Options{}, nil)
	if r.Error == nil || !strings.Contains(r.Error.Error(), "过期") {
		t.Fatalf("expired accepted: %+v", r)
	}
}
