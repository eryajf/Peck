package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func qaBundle(t *testing.T) (string, string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(base, "中文 空格 ' Sample.app")
	macos := filepath.Join(bundle, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleExecutable</key><string>sample</string><key>CFBundleIdentifier</key><string>org.example.qa</string><key>CFBundleName</key><string>QA fixture</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(plist), 0600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(macos, "sample")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return bundle, exe
}

func qaAttribute(t *testing.T, path, name string, value []byte) {
	t.Helper()
	if err := unix.Setxattr(path, name, value, 0); err != nil {
		t.Fatal(err)
	}
}

func TestQACaptureDoesNotMutateAndDetectsConfirmedChanges(t *testing.T) {
	for _, change := range []string{"content", "replacement", "new_file", "attribute"} {
		t.Run(change, func(t *testing.T) {
			bundle, exe := qaBundle(t)
			qaAttribute(t, exe, QuarantineAttribute, []byte("0081;00000000;Peck-QA;"))
			s, err := capture(context.Background(), bundle)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := unix.Getxattr(exe, QuarantineAttribute, nil); err != nil {
				t.Fatalf("inspection changed quarantine: %v", err)
			}
			switch change {
			case "content":
				err = os.WriteFile(exe, []byte("#!/bin/sh\nexit 1\n"), 0700)
			case "replacement":
				if err := os.Rename(exe, exe+".old"); err != nil {
					t.Fatal(err)
				}
				err = os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0700)
			case "new_file":
				err = os.WriteFile(filepath.Join(bundle, "new.txt"), []byte("new"), 0600)
			case "attribute":
				qaAttribute(t, exe, "org.example.peck.qa", []byte("new"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifySnapshot(context.Background(), s); err == nil {
				t.Fatal("post-confirmation change was accepted")
			}
		})
	}
}

func TestQAFileLinksCannotEscapeBundle(t *testing.T) {
	for _, hard := range []bool{false, true} {
		name := "symbolic"
		if hard {
			name = "hard"
		}
		t.Run(name, func(t *testing.T) {
			bundle, _ := qaBundle(t)
			outside := filepath.Join(filepath.Dir(bundle), "outside.txt")
			if err := os.WriteFile(outside, []byte("must stay unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			qaAttribute(t, outside, QuarantineAttribute, []byte("fixture"))
			link := filepath.Join(bundle, "Contents", "outside")
			var err error
			if hard {
				err = os.Link(outside, link)
			} else {
				err = os.Symlink(outside, link)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = capture(context.Background(), bundle); err == nil {
				t.Fatal("bundle containing external link was accepted")
			}
			if _, err = unix.Getxattr(outside, QuarantineAttribute, nil); err != nil {
				t.Fatal("external attribute changed")
			}
		})
	}
}

func TestQARemovalPreservesOtherAttributesAndCancellation(t *testing.T) {
	bundle, exe := qaBundle(t)
	qaAttribute(t, exe, QuarantineAttribute, []byte("fixture"))
	qaAttribute(t, exe, "org.example.peck.qa", []byte("keep me"))
	s, err := capture(context.Background(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := s.at("Contents/MacOS/sample")
	if !ok {
		t.Fatal("fixture executable missing")
	}
	root, err := openAbsolute(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := removeOne(ctx, root, e); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled removal: %v", err)
	}
	if _, err := unix.Getxattr(exe, QuarantineAttribute, nil); err != nil {
		t.Fatal("cancelled removal modified file")
	}
	if _, err := removeOne(context.Background(), root, e); err != nil {
		t.Fatal(err)
	}
	if _, err := unix.Getxattr(exe, QuarantineAttribute, nil); !errors.Is(err, unix.ENOATTR) {
		t.Fatalf("quarantine remains: %v", err)
	}
	buf := make([]byte, 32)
	n, err := unix.Getxattr(exe, "org.example.peck.qa", buf)
	if err != nil || string(buf[:n]) != "keep me" {
		t.Fatalf("unrelated metadata changed: %q %v", buf[:n], err)
	}
}

func TestQAReplacementBetweenPlanAndWriteIsRejected(t *testing.T) {
	bundle, exe := qaBundle(t)
	qaAttribute(t, exe, QuarantineAttribute, []byte("fixture"))
	s, err := capture(context.Background(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := s.at("Contents/MacOS/sample")
	root, err := openAbsolute(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(exe, exe+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("replacement"), 0700); err != nil {
		t.Fatal(err)
	}
	qaAttribute(t, exe, QuarantineAttribute, []byte("new fixture"))
	if _, err := removeOne(context.Background(), root, e); err == nil {
		t.Fatal("replacement accepted")
	}
	if _, err := unix.Getxattr(exe, QuarantineAttribute, nil); err != nil {
		t.Fatal("replacement was modified")
	}
}
