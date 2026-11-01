package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// codeContent normalizes only the fields codesign is allowed to change in a
// thin, 64-bit Mach-O: the signature load command, header command counters,
// __LINKEDIT size, and the signature blob appended at EOF. Every other byte,
// including header padding and executable code, participates in the digest.
func codeContent(data []byte) ([32]byte, error) {
	var zero [32]byte
	if len(data) < 32 || binary.LittleEndian.Uint32(data) != 0xfeedfacf {
		return zero, errors.New("补签仅支持单一 64 位 Mach-O 主程序")
	}
	order := binary.LittleEndian
	ncmds, size := int(order.Uint32(data[16:])), int(order.Uint32(data[20:]))
	if ncmds > 4096 || size < 0 || size > len(data)-32 {
		return zero, errors.New("Mach-O 加载命令无效")
	}
	header := append([]byte(nil), data[:32]...)
	clear(header[16:24])
	parts := [][]byte{header}
	off, end := 32, len(data)
	for count := 0; count < ncmds; count++ {
		if off+8 > 32+size {
			return zero, errors.New("Mach-O 加载命令不完整")
		}
		cmd, n := order.Uint32(data[off:]), int(order.Uint32(data[off+4:]))
		if n < 8 || off+n > 32+size {
			return zero, errors.New("Mach-O 加载命令越界")
		}
		if cmd == 0x1d { // LC_CODE_SIGNATURE
			if n != 16 || end != len(data) {
				return zero, errors.New("代码签名段无效")
			}
			sigOff, sigSize := int(order.Uint32(data[off+8:])), int(order.Uint32(data[off+12:]))
			if sigOff < 32+size || sigOff+sigSize != len(data) {
				return zero, errors.New("代码签名段不位于文件末尾")
			}
			end = sigOff
		} else {
			part := append([]byte(nil), data[off:off+n]...)
			if cmd == 0x19 && n >= 72 && string(bytes.TrimRight(part[8:24], "\x00")) == "__LINKEDIT" {
				clear(part[32:40]) // vmsize
				clear(part[48:56]) // filesize
			}
			parts = append(parts, part)
		}
		off += n
	}
	if off != 32+size {
		return zero, errors.New("Mach-O 加载命令长度不一致")
	}
	// Codesign consumes zero header padding for its new 16-byte command and
	// rounds signature placement to 16 bytes. Remove only these zero regions.
	for off < end && data[off] == 0 {
		off++
	}
	for end > off && data[end-1] == 0 {
		end--
	}
	parts = append(parts, data[off:end])
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
	}
	copy(zero[:], h.Sum(nil))
	return zero, nil
}

func sign(ctx context.Context, i *Inspection) (*snapshot, error) {
	before := i.snapshot
	root, err := openAbsolute(before.Path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := openRelative(root, i.executable, false)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, 512<<20))
	f.Close()
	if err != nil {
		return nil, err
	}
	digest, err := codeContent(data)
	if err != nil {
		return nil, err
	}
	if _, err = verifySnapshotAllowProvenance(ctx, before); err != nil {
		return nil, err
	}
	if _, err = systemCommand(ctx, "/usr/bin/codesign", "--sign", "-", "--timestamp=none", before.Path); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	state, reason := signature(ctx, before.Path)
	if state != "adhoc" || reason != "" {
		return nil, errors.New("补签产物未通过签名校验")
	}
	post, err := capture(ctx, before.Path)
	if err != nil {
		return nil, err
	}
	postRoot, err := openAbsolute(before.Path)
	if err != nil {
		return nil, err
	}
	defer postRoot.Close()
	postExe, err := openRelative(postRoot, i.executable, false)
	if err != nil {
		return nil, err
	}
	afterCode, err := io.ReadAll(io.LimitReader(postExe, 512<<20))
	postExe.Close()
	if err != nil {
		return nil, err
	}
	afterDigest, err := codeContent(afterCode)
	if err != nil || digest != afterDigest {
		return nil, errors.New("主程序发生了签名范围以外的变化")
	}
	if err := expectedSigningChanges(before, post, i.executable); err != nil {
		return nil, err
	}
	post, err = verifySnapshotAllowProvenance(ctx, post)
	if err != nil {
		return nil, err
	}
	return post, nil
}

func expectedSigningChanges(before, after *snapshot, executable string) error {
	for _, e := range before.Entries {
		n, ok := after.at(e.Path)
		if !ok {
			return changedError()
		}
		if e.Path == executable {
			// codesign may atomically replace the main executable. Its code
			// digest above must remain identical, and all metadata is retained.
			if e.Mode != n.Mode || e.UID != n.UID || e.Dev != n.Dev || n.Nlink != 1 || !sameAttributesAllowProvenance(e.Attrs, n.Attrs) {
				return errors.New("补签后主程序的身份或属性异常")
			}
			continue
		}
		if e.Path == "Contents" || e.Path == "Contents/MacOS" {
			// Creating _CodeSignature updates ancestor directory timestamps,
			// size and link counts. Identity, ownership, permissions and all
			// extended attributes other than system provenance remain strict.
			if e.Dev != n.Dev || e.Ino != n.Ino || e.UID != n.UID || e.Mode != n.Mode || !sameAttributesAllowProvenance(e.Attrs, n.Attrs) {
				return fmt.Errorf("补签范围外的目录发生变化：%s", e.Path)
			}
			continue
		}
		if !sameEntryAllowProvenance(e, n) {
			return fmt.Errorf("补签范围外的文件发生变化：%s", e.Path)
		}
	}
	for _, e := range after.Entries {
		if _, ok := before.at(e.Path); ok {
			continue
		}
		if e.Path != "Contents/_CodeSignature" && e.Path != "Contents/_CodeSignature/CodeResources" {
			return errors.New("补签产生了预期外的新文件")
		}
		if e.UID != uint32(os.Geteuid()) || !expectedNewAttributes(e.Attrs, before) || e.Path == "Contents/_CodeSignature/CodeResources" && e.Mode&unix.S_IFMT != unix.S_IFREG {
			return fmt.Errorf("补签新增文件的元数据异常：%s", e.Path)
		}
	}
	return nil
}

func expectedNewAttributes(attrs map[string][]byte, before *snapshot) bool {
	for name := range attrs {
		if name != provenanceAttribute && name != QuarantineAttribute {
			return false
		}
	}
	if quarantine, ok := attrs[QuarantineAttribute]; ok {
		for _, e := range before.Entries {
			value, inherited := e.Attrs[QuarantineAttribute]
			// On this macOS version mkdir inside a quarantined bundle creates
			// 0281;00000000;; rather than copying the download-origin string.
			// Accept that exact derived marker only if quarantine was in the
			// user's original plan. It is removed during the confirmed stage.
			if inherited && (bytes.Equal(quarantine, value) || string(quarantine) == "0281;00000000;;") {
				return true
			}
		}
		return false
	}
	return true
}
