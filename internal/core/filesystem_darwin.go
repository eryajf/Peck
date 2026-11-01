package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	maxEntries          = 100000
	provenanceAttribute = "com.apple.provenance"
)

var errUnstableEntry = errors.New("读取时应用发生变化，请重新选择")

// captureStable absorbs short-lived macOS metadata updates, which are common
// when a downloaded app is first opened in Finder. Every retry captures the
// whole bundle again; the caller still receives an error if the bundle keeps
// changing, so this never turns an unstable read into an authorization.
func captureStable(ctx context.Context, path string) (*snapshot, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		snapshot, err := capture(ctx, path)
		if err == nil {
			return snapshot, nil
		}
		last = err
		if !errors.Is(err, errUnstableEntry) || attempt == 2 {
			return nil, err
		}
		delay := time.Duration(attempt+1) * 45 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, last
}

type entry struct {
	Path       string
	Dev        int32
	Ino        uint64
	Mode       uint16
	UID        uint32
	Nlink      uint16
	Size       int64
	Mtime      unix.Timespec
	Hash       [32]byte
	LinkTarget string
	Attrs      map[string][]byte
}

type snapshot struct {
	Path    string
	Entries []entry
}

// openAbsolute walks from / with O_NOFOLLOW for every component. It does not
// trust a previously resolved string to remain free of symbolic links.
func openAbsolute(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("应用路径无效")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if e != nil {
			return nil, fmt.Errorf("无法安全读取应用目录：%w", e)
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), path), nil
}

func openRelative(root *os.File, path string, writable bool) (*os.File, error) {
	if path != "." && (!filepath.IsLocal(path) || filepath.Clean(path) != path) {
		return nil, errors.New("包内路径无效")
	}
	fd, err := unix.Dup(int(root.Fd()))
	if err != nil {
		return nil, err
	}
	parts := strings.Split(path, string(filepath.Separator))
	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		} else if writable {
			flags = unix.O_RDWR | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
		}
		next, e := unix.Openat(fd, part, flags, 0)
		unix.Close(fd)
		if e != nil {
			return nil, e
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), path), nil
}

// openRelativeLink opens the final component itself when it is a symbolic
// link. Every parent is still opened with O_NOFOLLOW, so this never follows a
// link while walking from the checked bundle root.
func openRelativeLink(root *os.File, path string) (*os.File, error) {
	if path == "." || !filepath.IsLocal(path) || filepath.Clean(path) != path {
		return nil, errors.New("包内符号链接路径无效")
	}
	fd, err := unix.Dup(int(root.Fd()))
	if err != nil {
		return nil, err
	}
	parts := strings.Split(path, string(filepath.Separator))
	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY | unix.O_NOFOLLOW
		} else {
			flags |= unix.O_SYMLINK
		}
		next, e := unix.Openat(fd, part, flags, 0)
		unix.Close(fd)
		if e != nil {
			return nil, e
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), path), nil
}

func sameLinkStat(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Uid == b.Uid &&
		a.Gid == b.Gid && a.Nlink == b.Nlink && a.Size == b.Size && a.Mtim == b.Mtim && a.Ctim == b.Ctim
}

func readLinkTarget(root *os.File, path string, expected unix.Stat_t) (string, error) {
	parent := filepath.Dir(path)
	name := filepath.Base(path)
	dir, err := openRelative(root, parent, false)
	if err != nil {
		return "", err
	}
	defer dir.Close()
	check := func() error {
		var current unix.Stat_t
		if err := unix.Fstatat(int(dir.Fd()), name, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return err
		}
		if !sameLinkStat(expected, current) {
			return changedError()
		}
		return nil
	}
	if err := check(); err != nil {
		return "", err
	}
	buf := make([]byte, 4096)
	for len(buf) <= 1<<20 {
		n, err := unix.Readlinkat(int(dir.Fd()), name, buf)
		if err != nil {
			return "", err
		}
		if n < len(buf) {
			if err := check(); err != nil {
				return "", err
			}
			return string(buf[:n]), nil
		}
		buf = append(buf, make([]byte, len(buf))...)
	}
	return "", errors.New("符号链接目标过长")
}

// validateInternalSymlinks resolves aliases entirely from the captured entries.
// No filesystem lookup follows a link, including intermediate links that leave
// the bundle and happen to lead back into it. Resolve '..' after each alias so
// nested framework/version links retain their actual filesystem meaning.
func validateInternalSymlinks(ctx context.Context, s *snapshot) error {
	entries := make(map[string]entry, len(s.Entries))
	for _, e := range s.Entries {
		entries[e.Path] = e
	}
	for _, link := range s.Entries {
		if link.Mode&unix.S_IFMT != unix.S_IFLNK {
			continue
		}
		pending := strings.Split(link.Path, "/")
		var resolved []string
		hops := 0
		for len(pending) > 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			part := pending[0]
			pending = pending[1:]
			if part == "" || part == "." {
				continue
			}
			if part == ".." {
				if len(resolved) == 0 {
					return fmt.Errorf("应用内链接 %s 指向应用包外，无法处理", link.Path)
				}
				resolved = resolved[:len(resolved)-1]
				continue
			}
			path := strings.Join(append(append([]string(nil), resolved...), part), "/")
			e, ok := entries[path]
			if !ok {
				return fmt.Errorf("应用内链接 %s 的目标缺失，请重新解压或下载完整应用", link.Path)
			}
			if e.Mode&unix.S_IFMT == unix.S_IFLNK {
				hops++
				if hops > 32 {
					return fmt.Errorf("应用内链接 %s 存在循环或层级过多", link.Path)
				}
				target := e.LinkTarget
				if target == "" {
					return fmt.Errorf("应用内链接 %s 的目标为空", link.Path)
				}
				if filepath.IsAbs(target) {
					if target != s.Path && !strings.HasPrefix(target, s.Path+"/") {
						return fmt.Errorf("应用内链接 %s 指向应用包外，无法处理", link.Path)
					}
					target = strings.TrimPrefix(strings.TrimPrefix(target, s.Path), "/")
					resolved = nil
				}
				pending = append(strings.Split(target, "/"), pending...)
				continue
			}
			if len(pending) > 0 && e.Mode&unix.S_IFMT != unix.S_IFDIR {
				return fmt.Errorf("应用内链接 %s 的目标路径无效，请重新解压或下载完整应用", link.Path)
			}
			resolved = append(resolved, part)
		}
	}
	return nil
}

func readLinkEntry(ctx context.Context, root, f *os.File, relative string) (entry, error) {
	if err := ctx.Err(); err != nil {
		return entry{}, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return entry{}, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFLNK {
		return entry{}, errors.New("应用在检查时发生变化，请重新选择")
	}
	if st.Nlink != 1 {
		return entry{}, errors.New("不处理具有多个硬链接的文件")
	}
	target, err := readLinkTarget(root, relative, st)
	if err != nil {
		return entry{}, err
	}
	attrs, err := readAttrs(int(f.Fd()))
	if err != nil {
		return entry{}, err
	}
	var after unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &after); err != nil {
		return entry{}, err
	}
	if !sameLinkStat(st, after) {
		return entry{}, errUnstableEntry
	}
	return entry{
		Path: relative, Dev: st.Dev, Ino: st.Ino, Mode: st.Mode, UID: st.Uid,
		Nlink: st.Nlink, Size: st.Size, Mtime: st.Mtim, LinkTarget: target, Attrs: attrs,
	}, nil
}

func capture(ctx context.Context, path string) (*snapshot, error) {
	root, err := openAbsolute(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	s := &snapshot{Path: path}
	var total int64
	appendEntry := func(e entry) error {
		if len(s.Entries) >= maxEntries {
			return errors.New("应用文件数量超过本版本的检查范围")
		}
		if len(s.Entries) > 0 && e.Dev != s.Entries[0].Dev {
			return errors.New("应用包含其他卷的内容，无法处理")
		}
		total += e.Size
		if total > 16<<30 {
			return errors.New("应用超过本版本的 16 GB 检查范围")
		}
		s.Entries = append(s.Entries, e)
		return nil
	}
	var walk func(*os.File, string) error
	walk = func(f *os.File, relative string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		e, err := readEntry(ctx, f, relative)
		if err != nil {
			return fmt.Errorf("%s：%w", relative, err)
		}
		if err := appendEntry(e); err != nil {
			return err
		}
		if e.Mode&unix.S_IFMT != unix.S_IFDIR {
			return nil
		}
		names, err := f.Readdirnames(-1)
		if err != nil {
			return err
		}
		sort.Strings(names)
		for _, name := range names {
			var st unix.Stat_t
			if err := unix.Fstatat(int(f.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				return err
			}
			if st.Mode&unix.S_IFMT == unix.S_IFLNK {
				linkPath := filepath.Join(relative, name)
				linkFile, err := openRelativeLink(root, linkPath)
				if err != nil {
					return err
				}
				link, err := readLinkEntry(ctx, root, linkFile, linkPath)
				linkFile.Close()
				if err != nil {
					return fmt.Errorf("%s：%w", linkPath, err)
				}
				if err := appendEntry(link); err != nil {
					return err
				}
				continue
			}
			if st.Mode&unix.S_IFMT != unix.S_IFREG && st.Mode&unix.S_IFMT != unix.S_IFDIR {
				return errors.New("应用包含不支持的特殊文件")
			}
			fd, err := unix.Openat(int(f.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if err != nil {
				return err
			}
			child := os.NewFile(uintptr(fd), name)
			err = walk(child, filepath.Join(relative, name))
			child.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err = walk(root, "."); err != nil {
		return nil, err
	}
	if err := validateInternalSymlinks(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func readEntry(ctx context.Context, f *os.File, relative string) (entry, error) {
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return entry{}, err
	}
	kind := st.Mode & unix.S_IFMT
	if kind != unix.S_IFDIR && kind != unix.S_IFREG || kind == unix.S_IFREG && st.Nlink != 1 {
		return entry{}, errors.New("只支持普通文件和目录，不处理硬链接或特殊文件")
	}
	e := entry{Path: relative, Dev: st.Dev, Ino: st.Ino, Mode: st.Mode, UID: st.Uid, Nlink: st.Nlink, Size: st.Size, Mtime: st.Mtim}
	attrs, err := readAttrs(int(f.Fd()))
	if err != nil {
		return e, err
	}
	e.Attrs = attrs
	if kind == unix.S_IFREG {
		h := sha256.New()
		buf := make([]byte, 128*1024)
		for off := int64(0); ; {
			if err := ctx.Err(); err != nil {
				return e, err
			}
			n, err := f.ReadAt(buf, off)
			h.Write(buf[:n])
			off += int64(n)
			if err == io.EOF {
				break
			}
			if err != nil {
				return e, err
			}
		}
		copy(e.Hash[:], h.Sum(nil))
	}
	var after unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &after); err != nil {
		return e, err
	}
	if st.Ino != after.Ino || st.Size != after.Size || st.Mtim != after.Mtim || st.Ctim != after.Ctim || st.Mode != after.Mode || st.Nlink != after.Nlink {
		return e, errUnstableEntry
	}
	return e, nil
}

func readAttrs(fd int) (map[string][]byte, error) {
	n, err := unix.Flistxattr(fd, nil)
	if err != nil {
		return nil, err
	}
	if n > 1<<20 {
		return nil, errors.New("扩展属性清单过大")
	}
	buf := make([]byte, n)
	n, err = unix.Flistxattr(fd, buf)
	if err != nil {
		return nil, err
	}
	attrs := map[string][]byte{}
	for _, name := range bytes.Split(buf[:n], []byte{0}) {
		if len(name) == 0 {
			continue
		}
		sz, err := unix.Fgetxattr(fd, string(name), nil)
		if err != nil || sz > 8<<20 {
			return nil, fmt.Errorf("无法读取扩展属性 %s", name)
		}
		v := make([]byte, sz)
		sz, err = unix.Fgetxattr(fd, string(name), v)
		if err != nil {
			return nil, err
		}
		attrs[string(name)] = v[:sz]
	}
	return attrs, nil
}

func sameSnapshot(a, b *snapshot) bool {
	return a != nil && b != nil && a.Path == b.Path && reflect.DeepEqual(a.Entries, b.Entries)
}

// sameSnapshotAllowProvenance keeps the confirmation boundary strict while
// accepting updates to macOS's provenance tracking metadata. This attribute
// is preserved on disk, and is never used to authorize processing. Every other
// attribute (including quarantine), file identity and content remains strict.
func sameSnapshotAllowProvenance(a, b *snapshot) bool {
	if a == nil || b == nil || a.Path != b.Path || len(a.Entries) != len(b.Entries) {
		return false
	}
	for idx := range a.Entries {
		if !sameEntryAllowProvenance(a.Entries[idx], b.Entries[idx]) {
			return false
		}
	}
	return true
}

func sameEntryAllowProvenance(a, b entry) bool {
	attrsA, attrsB := a.Attrs, b.Attrs
	a.Attrs, b.Attrs = nil, nil
	return reflect.DeepEqual(a, b) && sameAttributesAllowProvenance(attrsA, attrsB)
}

func sameAttributesAllowProvenance(a, b map[string][]byte) bool {
	for name, value := range a {
		if name == provenanceAttribute {
			continue
		}
		other, ok := b[name]
		if !ok || !bytes.Equal(value, other) {
			return false
		}
	}
	for name := range b {
		if name != provenanceAttribute {
			if _, ok := a[name]; !ok {
				return false
			}
		}
	}
	return true
}

func (s *snapshot) at(path string) (entry, bool) {
	for _, e := range s.Entries {
		if e.Path == path {
			return e, true
		}
	}
	return entry{}, false
}

func changedError() error {
	return errors.New("应用在检查后发生变化，请重新选择并确认")
}

func verifySnapshot(ctx context.Context, expected *snapshot) (*snapshot, error) {
	return verifySnapshotWithPolicy(ctx, expected, false)
}

func verifySnapshotAllowProvenance(ctx context.Context, expected *snapshot) (*snapshot, error) {
	return verifySnapshotWithPolicy(ctx, expected, true)
}

func verifySnapshotWithPolicy(ctx context.Context, expected *snapshot, allowProvenance bool) (*snapshot, error) {
	current, err := capture(ctx, expected.Path)
	if err != nil && ctx.Err() == nil && (errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) || allowProvenance && errors.Is(err, errUnstableEntry)) {
		// Retry an interrupted/unavailable read once. Never retry a completed
		// snapshot mismatch or replace the originally authorized expectation.
		// A provenance update can also change ctime during a final read. Read
		// a stable snapshot once more, still checking all other fields against
		// the original plan instead of adopting an unverified new baseline.
		current, err = capture(ctx, expected.Path)
	}
	if err != nil {
		return nil, err
	}
	if !sameSnapshot(expected, current) && (!allowProvenance || !sameSnapshotAllowProvenance(expected, current)) {
		return nil, snapshotChangeError(expected, current, allowProvenance)
	}
	return current, nil
}

// snapshotChangeError describes the first changed entry without exposing
// attribute values or weakening any of the exact snapshot comparisons.
func snapshotChangeError(expected, current *snapshot, allowProvenance bool) error {
	detail := func(path, field string) error {
		return fmt.Errorf("应用在检查后发生变化（%s：%s），请重新选择并确认", path, field)
	}
	if expected.Path != current.Path {
		return detail("应用", "路径")
	}
	actual := make(map[string]entry, len(current.Entries))
	for _, e := range current.Entries {
		actual[e.Path] = e
	}
	for _, want := range expected.Entries {
		got, ok := actual[want.Path]
		if !ok {
			return detail(want.Path, "文件或目录缺失")
		}
		delete(actual, want.Path)
		var fields []string
		if want.Dev != got.Dev || want.Ino != got.Ino {
			fields = append(fields, "文件身份")
		}
		if want.Mode != got.Mode {
			fields = append(fields, "类型或权限")
		}
		if want.UID != got.UID {
			fields = append(fields, "所属用户")
		}
		if want.Nlink != got.Nlink {
			fields = append(fields, "链接数量")
		}
		if want.Size != got.Size {
			fields = append(fields, "大小")
		}
		if want.Mtime != got.Mtime {
			fields = append(fields, "修改时间")
		}
		if want.Hash != got.Hash {
			fields = append(fields, "内容")
		}
		if want.LinkTarget != got.LinkTarget {
			fields = append(fields, "链接目标")
		}
		if !reflect.DeepEqual(want.Attrs, got.Attrs) {
			names := make(map[string]bool, len(want.Attrs)+len(got.Attrs))
			for name := range want.Attrs {
				names[name] = true
			}
			for name := range got.Attrs {
				names[name] = true
			}
			var changed []string
			for name := range names {
				if allowProvenance && name == provenanceAttribute {
					continue
				}
				before, beforeOK := want.Attrs[name]
				after, afterOK := got.Attrs[name]
				if beforeOK != afterOK || !reflect.DeepEqual(before, after) {
					changed = append(changed, name)
				}
			}
			sort.Strings(changed)
			if len(changed) > 0 {
				fields = append(fields, "扩展属性 "+strings.Join(changed, ", "))
			}
		}
		if len(fields) > 0 {
			return detail(want.Path, strings.Join(fields, "、"))
		}
	}
	for _, e := range current.Entries {
		if _, ok := actual[e.Path]; ok {
			return detail(e.Path, "新增文件或目录")
		}
	}
	return changedError()
}
