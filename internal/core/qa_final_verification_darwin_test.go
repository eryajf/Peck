package core

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// finalVerificationFixture is a self-built harmless application with 12 bundle
// entries, old timestamps, nested directories and unrelated empty attributes.
// Its executable returns immediately; it never uses an installed application.
func finalVerificationFixture(t *testing.T) (string, []string) {
	t.Helper()
	app := fixture(t)
	resources := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(filepath.Join(resources, "Empty"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.txt", "two.txt", "three.txt"} {
		if err := os.WriteFile(filepath.Join(resources, name), []byte("original"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := systemCommand(context.Background(), "/usr/bin/codesign", "--sign", "-", "--timestamp=none", app); err != nil {
		t.Fatalf("sign harmless application: %v: %s", err, out)
	}
	var paths []string
	if err := filepath.WalkDir(app, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		if err := unix.Setxattr(path, QuarantineAttribute, []byte("0081;12345678;PeckFinalVerification;"), 0); err != nil {
			return err
		}
		if err := unix.Setxattr(path, "org.example.peck.empty", []byte{}, 0); err != nil {
			return err
		}
		if err := unix.Setxattr(path, "org.example.peck.keep", []byte("unchanged"), 0); err != nil {
			return err
		}
		old := time.Unix(1700000000, 123456000)
		return os.Chtimes(path, old, old)
	}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 12 {
		t.Fatalf("fixture has %d entries, expected 12", len(paths))
	}
	return app, paths
}

func TestProcessFinalVerificationKeepsCompletedRemovalSeparate(t *testing.T) {
	for _, change := range []string{"none", "attribute", "content", "identity", "mtime", "cancellation"} {
		t.Run(change, func(t *testing.T) {
			app, paths := finalVerificationFixture(t)
			i, err := Inspect(context.Background(), app)
			if err != nil || i == nil || i.BlockedReason != "" || i.QuarantineCount != 12 {
				t.Fatalf("inspection: %+v %v", i, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			target := filepath.Join(app, "Contents", "Resources", "one.txt")
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			result := Process(ctx, i, Options{}, func(p Progress) {
				if p.Stage != "finish" {
					return
				}
				switch change {
				case "attribute":
					err = unix.Setxattr(target, "org.example.peck.concurrent", []byte("changed"), 0)
				case "content":
					err = os.WriteFile(target, []byte("modified"), 0644)
					if err == nil {
						err = os.Chtimes(target, info.ModTime(), info.ModTime())
					}
				case "identity":
					err = os.Remove(target)
					if err == nil {
						err = os.WriteFile(target, []byte("original"), 0644)
					}
				case "mtime":
					err = os.Chtimes(target, time.Now(), time.Now())
				case "cancellation":
					cancel()
				}
				if err != nil {
					t.Fatalf("inject %s: %v", change, err)
				}
			})
			if result.Changed != 12 || result.Signature != "未请求" || strings.Contains(result.Quarantine, "未全部完成") {
				t.Fatalf("completed removal misreported: %+v", result)
			}
			for _, path := range paths {
				if _, err := unix.Getxattr(path, QuarantineAttribute, nil); !errors.Is(err, unix.ENOATTR) {
					t.Fatalf("quarantine remains: %s: %v", path, err)
				}
			}
			if change == "none" {
				if result.Error != nil || result.Quarantine != "已移除" || result.Verification != "已通过" || !result.CanOpen || result.post == nil {
					t.Fatalf("stable application did not complete: %+v", result)
				}
				if _, err := verifySnapshot(context.Background(), result.post); err != nil {
					t.Fatalf("result did not keep the final real snapshot: %v", err)
				}
				return
			}
			if result.Error == nil || result.CanOpen || result.post != nil || !strings.Contains(result.Error.Error(), "最终核验未通过") {
				t.Fatalf("changed application was accepted: %+v", result)
			}
			if change == "cancellation" {
				if !result.Cancelled || result.Verification != "已停止" {
					t.Fatalf("final cancellation lost its stage: %+v", result)
				}
			} else if result.Verification != "未通过" {
				t.Fatalf("final failure lost its stage: %+v", result)
			}
			if change == "attribute" && !strings.Contains(result.Error.Error(), "org.example.peck.concurrent") {
				t.Fatalf("attribute mismatch was not diagnosed: %v", result.Error)
			}
			if change == "content" && !strings.Contains(result.Error.Error(), "内容") {
				t.Fatalf("content mismatch was not diagnosed: %v", result.Error)
			}
			if err := Open(context.Background(), &result); err == nil {
				t.Fatal("failed verification was allowed to open")
			}
		})
	}
}

func TestProcessReselectWithoutQuarantineUsesNewSnapshotAndCanOpen(t *testing.T) {
	app, _ := finalVerificationFixture(t)
	i, err := Inspect(context.Background(), app)
	if err != nil || i == nil || i.BlockedReason != "" {
		t.Fatalf("initial inspection: %+v %v", i, err)
	}
	target := filepath.Join(app, "Contents", "Resources", "one.txt")
	first := Process(context.Background(), i, Options{}, func(p Progress) {
		if p.Stage == "finish" {
			if err := unix.Setxattr(target, "org.example.peck.concurrent", []byte("new state"), 0); err != nil {
				t.Fatal(err)
			}
		}
	})
	if first.Error == nil || first.Changed != 12 || first.CanOpen {
		t.Fatalf("first result must retain a real final mismatch: %+v", first)
	}
	selectedAgain, err := Inspect(context.Background(), app)
	if err != nil || selectedAgain == nil || selectedAgain.BlockedReason != "" || selectedAgain.QuarantineCount != 0 {
		t.Fatalf("reselection: %+v %v", selectedAgain, err)
	}
	result := Process(context.Background(), selectedAgain, Options{}, nil)
	if result.Error != nil || result.Changed != 0 || result.Quarantine != "无需处理" || result.Verification != "已通过" || !result.CanOpen || result.post == nil {
		t.Fatalf("fresh confirmation could not finish: %+v", result)
	}
	if _, err := verifySnapshot(context.Background(), result.post); err != nil {
		t.Fatalf("final snapshot did not match the reselected application: %v", err)
	}
	// The only launched fixture is the tiny C application built by this test.
	if err := Open(context.Background(), &result); err != nil || result.Launch != "打开请求已提交" {
		t.Fatalf("open reselected harmless application: %v (%+v)", err, result)
	}
}

func TestProcessPartialRemovalReportsFailureAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "write precondition changed"
		if cancelled {
			name = "cancel after first removal"
		}
		t.Run(name, func(t *testing.T) {
			app, _ := finalVerificationFixture(t)
			i, err := Inspect(context.Background(), app)
			if err != nil || i == nil || i.BlockedReason != "" {
				t.Fatalf("inspection: %+v %v", i, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := Process(ctx, i, Options{}, func(p Progress) {
				if p.Stage != "quarantine" || p.Completed != 1 {
					return
				}
				if cancelled {
					cancel()
				} else if err := unix.Setxattr(filepath.Join(app, "Contents"), "org.example.peck.concurrent", []byte("changed"), 0); err != nil {
					t.Fatal(err)
				}
			})
			if result.Error == nil || result.CanOpen || result.Changed != 1 || strings.Contains(result.Quarantine, "未全部完成") {
				t.Fatalf("partial result: %+v", result)
			}
			if cancelled {
				if !result.Cancelled || result.Quarantine != "已停止；已执行 1 项移除" {
					t.Fatalf("partial cancellation: %+v", result)
				}
			} else if result.Quarantine != "已执行 1 项移除；后续操作未完成" {
				t.Fatalf("partial failure: %+v", result)
			}
		})
	}
}
