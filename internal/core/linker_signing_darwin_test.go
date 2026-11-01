package core

import (
	"context"
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func linkerFixture(t *testing.T) string {
	t.Helper()
	app := fixture(t)
	cmd := exec.Command("/usr/bin/clang", "-o", filepath.Join(app, "Contents/MacOS/fixture"), filepath.Join(filepath.Dir(app), "main.c"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("linker fixture: %v: %s", err, out)
	}
	resources := filepath.Join(app, "Contents/Resources")
	if err := os.MkdirAll(resources, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "example.txt"), []byte("resource"), 0644); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestLinkerSignatureRepair(t *testing.T) {
	ctx := context.Background()
	app := linkerFixture(t)
	if err := unix.Setxattr(app, QuarantineAttribute, []byte("0081;12345678;PeckTest;"), 0); err != nil {
		t.Fatal(err)
	}
	i, err := Inspect(ctx, app)
	if err != nil || i.SignatureStatus != "adhoc-invalid" || !i.CanSign || i.BlockedReason != "" {
		out, checkErr := systemCommand(ctx, "/usr/sbin/spctl", "--assess", "--type", "execute", "--verbose=4", app)
		t.Logf("assessment: %q %v", out, checkErr)
		t.Fatalf("inspection: %+v %v", i, err)
	}
	r := Process(ctx, i, Options{}, nil)
	if r.Error != nil || !r.CanOpen || r.Changed < 1 {
		t.Fatalf("quarantine-only processing should not require repair signing: %+v", r)
	}
	if r.Signature != "未请求" {
		t.Fatalf("normal processing unexpectedly signed the application: %+v", r)
	}
	if state, reason := signature(ctx, app); state != "adhoc-invalid" || reason != "" {
		t.Fatalf("quarantine-only processing changed the signature: %s %s", state, reason)
	}
	// A caller that explicitly requests signing may still use the restricted
	// repair path on a fresh fixture.
	app = linkerFixture(t)
	if err := unix.Setxattr(app, QuarantineAttribute, []byte("0081;12345678;PeckTest;"), 0); err != nil {
		t.Fatal(err)
	}
	i, err = Inspect(ctx, app)
	if err != nil {
		t.Fatal(err)
	}
	r = Process(ctx, i, Options{Sign: true}, nil)
	if r.Error != nil || !r.CanOpen || r.Changed < 1 {
		t.Fatalf("repair: %+v", r)
	}
	if state, reason := signature(ctx, app); state != "adhoc" || reason != "" {
		t.Fatalf("final signature: %s %s", state, reason)
	}
	i, err = Inspect(ctx, app)
	if err != nil || i.CanSign {
		t.Fatalf("completed signature must be preserved: %+v %v", i, err)
	}
}

func TestLinkerSignatureModifiedCodeRejected(t *testing.T) {
	app := linkerFixture(t)
	exe := filepath.Join(app, "Contents/MacOS/fixture")
	f, err := macho.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	section := f.Section("__text")
	if section == nil {
		t.Fatal("missing text section")
	}
	offset := section.Offset
	f.Close()
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	data[offset] ^= 1
	if err := os.WriteFile(exe, data, 0755); err != nil {
		t.Fatal(err)
	}
	if state, reason := signature(context.Background(), app); state != "invalid" || reason == "" {
		t.Fatalf("modified code accepted: %s %s", state, reason)
	}
}

func TestLinkerAssessmentOnlyDefersKnownFailure(t *testing.T) {
	for _, msg := range []string{"rejected", "internal error in code signing subsystem"} {
		code := 3
		if strings.HasPrefix(msg, "internal") {
			code = 1
		}
		if _, next, err := classifyAssessment(msg, "adhoc-invalid", code); err != nil || next {
			t.Fatalf("known linker limitation: %v", err)
		}
		if _, _, err := classifyAssessment(msg+"\nmalware", "adhoc-invalid", code); err == nil {
			t.Fatal("security signal ignored")
		}
		if _, _, err := classifyAssessment(msg, "invalid", code); err == nil {
			t.Fatal("invalid developer signature accepted")
		}
	}
}
