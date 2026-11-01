package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestQAExistingAdhocAppCanRemoveQuarantineWithoutResigning(t *testing.T) {
	ctx := context.Background()
	app := fixture(t)
	main := filepath.Join(app, "Contents", "MacOS", "fixture")
	resources := filepath.Join(app, "Contents", "_CodeSignature", "CodeResources")
	if out, err := systemCommand(ctx, "/usr/bin/codesign", "--sign", "-", "--timestamp=none", app); err != nil {
		t.Fatalf("sign harmless fixture: %v: %s", err, out)
	}
	beforeMain, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	beforeResources, err := os.ReadFile(resources)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{app, main} {
		if err := unix.Setxattr(path, QuarantineAttribute, []byte("0081;12345678;PeckQATest;"), 0); err != nil {
			t.Fatal(err)
		}
		if err := unix.Setxattr(path, "org.example.qa.keep", []byte("unchanged"), 0); err != nil {
			t.Fatal(err)
		}
	}
	i, err := Inspect(ctx, app)
	if err != nil {
		t.Fatal(err)
	}
	if i.SignatureStatus != "adhoc" || i.BlockedReason != "" || i.CanSign || i.QuarantineCount != 2 {
		t.Fatalf("existing ad-hoc fixture must remain eligible for isolation-only processing: %+v", i)
	}
	if i.Assessment == "" || strings.Contains(i.Assessment, "无法确认") {
		t.Fatalf("assessment must explain the real system outcome: %q", i.Assessment)
	}
	t.Logf("existing ad-hoc assessment: %s", i.Assessment)
	result := Process(ctx, i, Options{}, nil)
	if result.Error != nil || result.Signature != "未请求" || result.Quarantine != "已移除" || result.Changed != 2 || !result.CanOpen || result.Launch != "未请求" {
		t.Fatalf("isolation-only result: %+v", result)
	}
	for _, path := range []string{app, main} {
		if _, err := unix.Getxattr(path, QuarantineAttribute, nil); !errors.Is(err, unix.ENOATTR) {
			t.Fatalf("quarantine remains: %v", err)
		}
		buf := make([]byte, 32)
		n, err := unix.Getxattr(path, "org.example.qa.keep", buf)
		if err != nil || string(buf[:n]) != "unchanged" {
			t.Fatalf("unrelated metadata changed: %v", err)
		}
	}
	afterMain, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	afterResources, err := os.ReadFile(resources)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeMain, afterMain) || !bytes.Equal(beforeResources, afterResources) {
		t.Fatal("quarantine-only processing rewrote executable or signature resources")
	}
	if status, reason := signature(ctx, app); status != "adhoc" || reason != "" {
		t.Fatalf("existing signature changed: %s %s", status, reason)
	}
}
