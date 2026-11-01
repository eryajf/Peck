//go:build darwin

package update

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunApplyRestoresOldBundleWhenLaunchFails(t *testing.T) {
	dir := t.TempDir()
	target, staged := filepath.Join(dir, "Peck.app"), filepath.Join(dir, "staging", "Peck.app")
	for _, p := range []string{target, staged} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "marker"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "marker"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	oldWait, oldCopy, oldRestart := applyWaitForExit, applyCopyBundle, applyRestart
	defer func() { applyWaitForExit, applyCopyBundle, applyRestart = oldWait, oldCopy, oldRestart }()
	applyWaitForExit = func(context.Context, int) error { return nil }
	applyCopyBundle = func(src, dst string) error {
		if err := os.MkdirAll(dst, 0700); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(src, "marker"))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, "marker"), data, 0600)
	}
	calls := 0
	applyRestart = func(string) error {
		calls++
		if calls == 1 {
			return errors.New("launch failed")
		}
		return nil
	}
	pkg := filepath.Join(dir, "update.zip")
	err := runApply([]string{"--pid", fmt.Sprint(os.Getpid()), "--staged", staged, "--target", target, "--package", pkg})
	if err == nil || !strings.Contains(err.Error(), "launch failed") {
		t.Fatalf("error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "marker"))
	if err != nil || string(data) != "old" {
		t.Fatalf("rollback marker = %q: %v", data, err)
	}
	if calls != 2 {
		t.Fatalf("restart calls = %d", calls)
	}
	if _, err := os.Stat(pkg + applyErrorSuffix); err != nil {
		t.Fatalf("missing failure record: %v", err)
	}
}

func TestWaitForStartupRequiresConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "started")
	if err := waitForStartup(path, 10*time.Millisecond); err == nil {
		t.Fatal("missing confirmation accepted")
	}
	if err := os.WriteFile(path, []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := waitForStartup(path, 10*time.Millisecond); err == nil {
		t.Fatal("invalid confirmation accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmStartup(strings.TrimSuffix(path, ".started"), "0.2.0", "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("old version confirmed startup")
	}
	pkg := filepath.Join(t.TempDir(), "update.zip")
	if err := ConfirmStartup(pkg, "0.2.0", "0.2.0"); err != nil {
		t.Fatal(err)
	}
	if err := waitForStartup(pkg+applyStartupSuffix, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestStartupTimeoutKeepsBothBundlesWithoutRelaunch(t *testing.T) {
	dir := t.TempDir()
	target, staged := filepath.Join(dir, "Peck.app"), filepath.Join(dir, "staging", "Peck.app")
	for _, p := range []string{target, staged} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "marker"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "marker"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	oldWait, oldCopy, oldRestart, oldStartup := applyWaitForExit, applyCopyBundle, applyRestart, applyWaitStartup
	defer func() {
		applyWaitForExit, applyCopyBundle, applyRestart, applyWaitStartup = oldWait, oldCopy, oldRestart, oldStartup
	}()
	applyWaitForExit = func(context.Context, int) error { return nil }
	applyCopyBundle = func(src, dst string) error {
		if err := os.MkdirAll(dst, 0700); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(src, "marker"))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, "marker"), data, 0600)
	}
	calls := 0
	applyRestart = func(string) error { calls++; return nil }
	startupConfirmed := false
	applyWaitStartup = func(string, time.Duration) error {
		if !startupConfirmed {
			return errors.New("startup unconfirmed")
		}
		return nil
	}
	pkg := filepath.Join(dir, "update.zip")
	err := runApply([]string{"--pid", fmt.Sprint(os.Getpid()), "--staged", staged, "--target", target, "--package", pkg})
	if err == nil || !strings.Contains(err.Error(), "startup unconfirmed") {
		t.Fatalf("error = %v", err)
	}
	for _, entry := range []struct{ path, want string }{{target, "new"}, {filepath.Join(dir, ".Peck.app.old"), "old"}} {
		data, err := os.ReadFile(filepath.Join(entry.path, "marker"))
		if err != nil || string(data) != entry.want {
			t.Fatalf("%s = %q, %v", entry.path, data, err)
		}
	}
	if calls != 1 {
		t.Fatalf("unexpected extra launch: %d", calls)
	}

	// The process may confirm after the helper timed out. That confirmation
	// is the only evidence that makes removing the preserved backup safe.
	if err := os.WriteFile(pkg+applyStartupSuffix, []byte(applyReady), 0600); err != nil {
		t.Fatal(err)
	}
	startupConfirmed = true
	// A later release has a different ZIP path; recovery must consult the
	// previous transaction, not the new package's marker.
	nextPackage := filepath.Join(dir, "next-version.zip")
	if err := runApply([]string{"--pid", fmt.Sprint(os.Getpid()), "--staged", staged, "--target", target, "--package", nextPackage}); err != nil {
		t.Fatalf("retry after confirmed startup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".Peck.app.old")); !os.IsNotExist(err) {
		t.Fatalf("confirmed backup still exists: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "marker"))
	if err != nil || string(data) != "new" {
		t.Fatalf("retried target = %q, %v", data, err)
	}
}

func TestWaitForReady(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	go func() { _, _ = writer.WriteString(applyReady) }()
	if err := waitForReady(reader, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForReadyTimesOut(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if err := waitForReady(reader, 10*time.Millisecond); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("waitForReady error = %v", err)
	}
}

func TestExtractBundleRejectsTraversal(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "update.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("../outside.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("unsafe"))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractBundle(archivePath, filepath.Join(t.TempDir(), "stage")); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestPersistApplyErrorUsesAtomicFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.error")
	want := errors.New("replacement failed")
	if err := persistApplyError(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want.Error()+"\n" {
		t.Fatalf("persisted error = %q", data)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary error file still exists: %v", err)
	}
}
