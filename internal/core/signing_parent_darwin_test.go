package core

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestTrustedSigningParent(t *testing.T) {
	admin := func() (uint32, bool) { return 80, true }
	cases := []struct {
		name, path string
		uid, gid   uint32
		mode       uint16
		lookup     func() (uint32, bool)
		want       bool
	}{
		{"standard Applications", "/Applications", 0, 80, 0775, admin, true},
		{"other group", "/Applications", 0, 81, 0775, admin, false},
		{"world writable", "/Applications", 0, 80, 0777, admin, false},
		{"admin lookup failure", "/Applications", 0, 80, 0775, func() (uint32, bool) { return 0, false }, false},
		{"other group writable directory", "/shared", 0, 80, 0775, admin, false},
		{"user owned private", "/private/user", uint32(os.Geteuid()), 20, 0755, admin, true},
		{"root sticky directory", "/private/tmp", 0, 0, 01777, admin, true},
		{"other owner", "/other", uint32(os.Geteuid()) + 1, 80, 0755, admin, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := unix.Stat_t{Uid: tc.uid, Gid: tc.gid, Mode: tc.mode}
			if got := trustedSigningParent(tc.path, st, tc.lookup); got != tc.want {
				t.Fatalf("trustedSigningParent(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
