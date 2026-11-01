package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// Process uses the inspection's private snapshot rather than public path or
// status fields. Calling it is the user's explicit confirmation boundary.
func Process(ctx context.Context, i *Inspection, opts Options, progress func(Progress)) (result Result) {
	result = Result{Signature: "未请求", Quarantine: "未处理", Launch: "未请求", Verification: "未开始"}
	fail := func(err error) Result {
		result.Error = err
		result.Cancelled = errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
		result.CanOpen = false
		if result.Verification == "核验中" {
			result.Verification = "未通过"
			if result.Cancelled {
				result.Verification = "已停止"
			}
		} else if result.Quarantine != "已移除" && result.Quarantine != "无需处理" {
			if result.Cancelled {
				result.Quarantine = "已停止，未移除属性"
				if result.Changed > 0 {
					result.Quarantine = fmt.Sprintf("已停止；已执行 %d 项移除", result.Changed)
				}
			} else if result.Changed > 0 {
				result.Quarantine = fmt.Sprintf("已执行 %d 项移除；后续操作未完成", result.Changed)
			}
		}
		return result
	}
	if i == nil || i.snapshot == nil {
		return fail(errors.New("请先选择并检查应用"))
	}
	result.Path, result.Name = i.snapshot.Path, i.Name
	if time.Since(i.created) > 10*time.Minute {
		return fail(errors.New("检查结果已过期，请重新选择应用"))
	}
	report(progress, "verify", "正在重新核对应用", 0, 0)
	current, err := verifySnapshotAllowProvenance(ctx, i.snapshot)
	if err != nil {
		return fail(err)
	}
	// Repeat trust checks; display fields are not authority and cached trust
	// decisions can change even when application bytes have not.
	state, blocked := signature(ctx, current.Path)
	if blocked != "" {
		return fail(errors.New(blocked))
	}
	// An incomplete ad-hoc bundle does not require re-signing to remove its
	// quarantine marker. The user may attempt opening after marker removal;
	// signing remains an explicit internal option for callers that
	// need it, but is never a prerequisite for the normal flow.
	if _, assessmentErr := assess(ctx, current.Path, state); assessmentErr != nil {
		return fail(assessmentErr)
	}
	if opts.Sign {
		clone := *i
		clone.Path, clone.snapshot, clone.SignatureStatus = current.Path, current, state
		if reason := signingEligibility(ctx, &clone); reason != "" {
			return fail(errors.New(reason))
		}
		report(progress, "sign", "正在创建本地签名", 0, 0)
		result.Signature = "已开始，结果尚未确认"
		current, err = sign(ctx, &clone)
		if err != nil {
			result.Signature = "未完成，可能留下签名修改"
			return fail(fmt.Errorf("补签未完成：%w；请从项目来源获取干净副本", err))
		}
		result.Signature = "本地 ad-hoc 签名已验证"
		if state == "adhoc-invalid" {
			if _, err := assess(ctx, current.Path, "adhoc"); err != nil {
				return fail(err)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if current, err = verifySnapshotAllowProvenance(ctx, current); err != nil {
		return fail(err)
	}
	root, err := openAbsolute(current.Path)
	if err != nil {
		return fail(err)
	}
	defer root.Close()
	var rootStat unix.Stat_t
	if err = unix.Fstat(int(root.Fd()), &rootStat); err != nil || rootStat.Ino != current.Entries[0].Ino || rootStat.Dev != current.Entries[0].Dev {
		return fail(changedError())
	}
	total := 0
	for _, e := range current.Entries {
		if _, ok := e.Attrs[QuarantineAttribute]; ok {
			total++
		}
	}
	expected := &snapshot{Path: current.Path, Entries: append([]entry(nil), current.Entries...)}
	for idx, e := range current.Entries {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if _, ok := e.Attrs[QuarantineAttribute]; !ok {
			result.Skipped++
			continue
		}
		report(progress, "quarantine", "正在移除已确认的隔离标记", result.Changed, total)
		removed, err := removeOne(ctx, root, e)
		if err != nil {
			result.Failed++
			if removed {
				result.Changed++
			}
			return fail(fmt.Errorf("%s：%w", e.Path, err))
		}
		result.Changed++
		attrs := make(map[string][]byte, len(e.Attrs))
		for k, v := range e.Attrs {
			if k != QuarantineAttribute {
				attrs[k] = v
			}
		}
		expected.Entries[idx].Attrs = attrs
	}
	// Every planned removal succeeded and its attribute was read back absent.
	// Keep that completed operation separate from the whole-app verification;
	// a later mismatch must not misreport the removal loop as incomplete.
	result.Verification = "核验中"
	if total == 0 {
		result.Quarantine = "未发现隔离标记"
	} else {
		result.Quarantine = fmt.Sprintf("已执行 %d 项移除", result.Changed)
	}
	report(progress, "finish", "正在核验处理结果", result.Changed, total)
	post, err := verifySnapshotAllowProvenance(ctx, expected)
	if err != nil {
		return fail(fmt.Errorf("应用最终核验未通过：%w", err))
	}
	result.post, result.CanOpen, result.Verification = post, true, "已通过"
	if total == 0 {
		result.Quarantine = "无需处理"
	} else {
		result.Quarantine = "已移除"
	}
	return result
}

func removeOne(ctx context.Context, root *os.File, e entry) (bool, error) {
	var f *os.File
	var err error
	if e.Mode&unix.S_IFMT == unix.S_IFLNK {
		f, err = openRelativeLink(root, e.Path)
	} else {
		f, err = openRelative(root, e.Path, false)
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	var now entry
	for attempt := 0; attempt < 2; attempt++ {
		if e.Mode&unix.S_IFMT == unix.S_IFLNK {
			// The same descriptor is checked and modified; links are never
			// followed to their targets, which have their own entries.
			now, err = readLinkEntry(ctx, root, f, e.Path)
		} else {
			now, err = readEntry(ctx, f, e.Path)
		}
		if !errors.Is(err, errUnstableEntry) || ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		return false, err
	}
	if !sameEntryAllowProvenance(now, e) {
		return false, changedError()
	}
	// The descriptor pins the checked inode. No shell, recursive xattr command,
	// arbitrary attribute name, or path lookup is involved in the write.
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err = unix.Fremovexattr(int(f.Fd()), QuarantineAttribute); err != nil {
		return false, fmt.Errorf("无法移除隔离标记（请确认当前用户有写入权限）：%w", err)
	}
	_, err = unix.Fgetxattr(int(f.Fd()), QuarantineAttribute, nil)
	if !errors.Is(err, unix.ENOATTR) {
		return true, errors.New("隔离标记移除后的验证未通过")
	}
	return true, nil
}

// Open requests Launch Services to open the verified result. A successful
// request does not imply that the target application finished launching.
func Open(ctx context.Context, r *Result) error {
	if r == nil || !r.CanOpen || r.post == nil || r.Error != nil {
		return errors.New("仅可打开已通过本次处理核验的应用")
	}
	post, err := verifySnapshotAllowProvenance(ctx, r.post)
	if err != nil {
		r.Launch = "未打开，应用已变化"
		r.CanOpen = false
		return err
	}
	r.post = post
	if _, err := systemCommand(ctx, "/usr/bin/open", "-a", filepath.Clean(r.post.Path)); err != nil {
		r.Launch = "打开请求未被接受"
		return fmt.Errorf("系统未接受打开请求：%w", err)
	}
	r.Launch = "打开请求已提交"
	return nil
}
