package core

import (
	"bytes"
	"context"
	"debug/macho"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// systemCommand never invokes a shell and never inherits executable search or
// dynamic-loader variables from a target app. All callers pass fixed tools.
func systemCommand(ctx context.Context, tool string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C"}
	var buf limitedBuffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return buf.Bytes(), err
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := (1 << 20) - b.Len(); room > 0 {
		b.Buffer.Write(p[:min(room, len(p))])
	}
	return n, nil
}

const (
	assessmentUnavailable      = "系统评估暂不可用"
	assessmentRejected         = "应用未通过安全评估"
	assessmentQuarantineOnly   = "签名校验通过；系统未说明拒绝原因，可保留签名并仅移除隔离标记后尝试打开"
	assessmentAlreadyProcessed = "应用已处理：签名校验通过，未发现隔离标记；可尝试打开"
)

var (
	assessmentUnavailableError = errors.New("系统评估服务暂时异常：macOS 未返回可确认的结果，请稍后重试。Peck 未修改应用。")
	assessmentRejectedError    = errors.New("应用未通过 macOS 安全评估：请从开发者处确认应用来源和签名。Peck 未修改应用。")
)

func Inspect(ctx context.Context, path string) (*Inspection, error) {
	// Application bundles are directories; Finder paths may end in a slash.
	path = strings.TrimRight(path, string(os.PathSeparator))
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".app") || !safeText(path) {
		return nil, errors.New("请选择一个本地 .app 应用")
	}
	// Resolve the user's initial Finder path once, then never follow links
	// during snapshot traversal or writes. /tmp on macOS resolves to /private/tmp.
	real, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("无法读取应用：%w", err)
	}
	if !strings.EqualFold(filepath.Ext(real), ".app") {
		return nil, errors.New("链接目标不是 .app 应用")
	}
	for _, forbidden := range []string{"/System/", "/Library/Apple/", "/usr/", "/bin/", "/sbin/"} {
		if strings.HasPrefix(real, forbidden) {
			return nil, errors.New("不能处理系统目录中的应用")
		}
	}
	s, err := captureStable(ctx, real)
	if err != nil {
		return nil, err
	}
	if plist, ok := s.at("Contents/Info.plist"); !ok || plist.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, errors.New("应用缺少 Contents/Info.plist，请从项目来源重新下载")
	}
	data, err := systemCommand(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", filepath.Join(real, "Contents/Info.plist"))
	if err != nil {
		return nil, errors.New("无法读取应用信息文件")
	}
	var info map[string]any
	if json.Unmarshal(data, &info) != nil {
		return nil, errors.New("应用信息文件格式无效")
	}
	str := func(key string) string { v, _ := info[key].(string); return v }
	exe := str("CFBundleExecutable")
	if exe == "" || exe == "." || filepath.Base(exe) != exe || !safeText(exe) {
		return nil, errors.New("应用主程序名称无效")
	}
	relExe := filepath.Join("Contents/MacOS", exe)
	main, ok := s.at(relExe)
	if !ok || main.Mode&unix.S_IFMT != unix.S_IFREG || main.Mode&0111 == 0 {
		return nil, errors.New("应用缺少可执行主程序")
	}
	name := str("CFBundleDisplayName")
	if name == "" {
		name = str("CFBundleName")
	}
	if name == "" || !safeText(name) {
		name = strings.TrimSuffix(filepath.Base(real), filepath.Ext(real))
	}
	if !safeText(str("CFBundleShortVersionString")) || !safeText(str("CFBundleIdentifier")) {
		return nil, errors.New("应用信息包含不支持的控制字符")
	}
	i := &Inspection{Path: real, Name: name, Version: str("CFBundleShortVersionString"), BundleID: str("CFBundleIdentifier"), snapshot: s, executable: relExe, created: time.Now()}
	for _, e := range s.Entries {
		if _, ok := e.Attrs[QuarantineAttribute]; ok {
			i.QuarantineCount++
		}
	}
	i.SignatureStatus, i.BlockedReason = signature(ctx, real)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	i.Assessment, err = assess(ctx, real, i.SignatureStatus)
	if err == nil && isAlreadyProcessed(s, i.SignatureStatus, i.Assessment) {
		i.AlreadyProcessed = true
		i.Assessment = assessmentAlreadyProcessed
	}
	if err != nil && i.BlockedReason == "" {
		i.BlockedReason = err.Error()
	}
	root, err := openAbsolute(real)
	if err != nil {
		return nil, err
	}
	var fs unix.Statfs_t
	err = unix.Fstatfs(int(root.Fd()), &fs)
	root.Close()
	if err != nil {
		return nil, err
	}
	if fs.Flags&unix.MNT_RDONLY != 0 {
		i.BlockedReason = "应用位于只读磁盘或安装镜像，请先将 .app 复制到下载文件夹或应用程序文件夹，再重新选择"
	}
	if i.BlockedReason == "" {
		i.SignReason = signingEligibility(ctx, i)
		i.CanSign = i.SignReason == ""
		// An incomplete ad-hoc bundle can still be handled by removing its
		// quarantine marker. SignReason describes the optional repair path and
		// must not block the normal, user-confirmed flow.
	} else {
		i.SignReason = i.BlockedReason
	}
	if i.snapshot, err = verifySnapshotAllowProvenance(ctx, s); err != nil {
		return nil, err
	}
	return i, nil
}

func quarantineCount(s *snapshot) int {
	if s == nil {
		return 0
	}
	count := 0
	for _, e := range s.Entries {
		if _, ok := e.Attrs[QuarantineAttribute]; ok {
			count++
		}
	}
	return count
}

// The quarantine-only assessment also permits a no-op verification when the
// attributes were already removed. It never overrides a failed assessment.
func isAlreadyProcessed(s *snapshot, signatureStatus, assessment string) bool {
	return s != nil && signatureStatus == "valid" && quarantineCount(s) == 0 && assessment == assessmentQuarantineOnly
}

func signature(ctx context.Context, path string) (string, string) {
	out, err := systemCommand(ctx, "/usr/bin/codesign", "--display", "--verbose=4", path)
	msg := diagnosticText(string(out), path)
	if err != nil {
		if strings.Contains(msg, "code object is not signed at all") {
			return "unsigned", ""
		}
		return "unknown", "无法确认应用签名状态，请从开发者处取得完整应用"
	}
	verifyOut, verifyErr := systemCommand(ctx, "/usr/bin/codesign", "--verify", "--strict", "--verbose=2", path)
	if verifyErr != nil {
		// Some toolchains put an ad-hoc linker signature on the executable
		// before the rest of the .app bundle is assembled. macOS then reports
		// the bundle as invalid although there is no developer signature to preserve.
		if repairableLinkerSignature(msg, diagnosticText(string(verifyOut), path)) {
			// Check executable pages independently before completing the missing
			// bundle seal. A modified executable must not enter this repair path.
			if _, err := systemCommand(ctx, "/usr/bin/codesign", "--verify", "--strict", "--ignore-resources", path); err == nil {
				return "adhoc-invalid", ""
			}
		}
		return "invalid", "签名校验未通过，停止处理；请从项目来源重新下载"
	}
	for _, line := range strings.Split(msg, "\n") {
		if line == "Signature=adhoc" {
			return "adhoc", ""
		}
	}
	return "valid", ""
}

func repairableLinkerSignature(display, diagnostic string) bool {
	lines := make(map[string]bool)
	linker := false
	for _, line := range strings.Split(display, "\n") {
		lines[line] = true
		if strings.HasPrefix(line, "CodeDirectory ") && strings.Contains(line, "flags=0x20002(adhoc,linker-signed)") {
			linker = true
		}
	}
	return linker && lines["Signature=adhoc"] && lines["Info.plist=not bound"] && lines["Sealed Resources=none"] && lines["Internal requirements=none"] &&
		strings.TrimSpace(diagnostic) == "code has no resources but signature indicates they must be present"
}

func assess(ctx context.Context, path, signatureStatus string) (string, error) {
	out, err := systemCommand(ctx, "/usr/sbin/spctl", "--assess", "--type", "execute", "--verbose=4", path)
	if err == nil {
		return "系统评估已接受", nil
	}
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
		return assessmentUnavailable, assessmentUnavailableError
	}
	msg := strings.ToLower(diagnosticText(string(out), path))
	exitCode := -1
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		exitCode = exit.ExitCode()
	}
	assessment, needsDistribution, err := classifyAssessment(msg, signatureStatus, exitCode)
	if needsDistribution {
		return assessDistribution(ctx, path)
	}
	return assessment, err
}

// classifyAssessment separates a distribution warning, a system-service
// failure, and an application rejection. A bare rejection does not identify a
// security finding: a verified signature can be preserved for quarantine-only
// processing, while ad-hoc signatures still need a structured diagnostic.
func classifyAssessment(msg, signatureStatus string, exitCode int) (string, bool, error) {
	for _, prohibited := range []string{"revoked", "malware", "malicious", "xprotect", "resource envelope is obsolete", "invalid signature", "a sealed resource is missing", "resource is missing", "resource missing"} {
		if strings.Contains(msg, prohibited) {
			return assessmentRejected, false, assessmentRejectedError
		}
	}
	// Signature inspection has verified the executable independently. These
	// exact diagnostics describe the incomplete linker seal; unrelated errors
	// and explicit security findings still block processing.
	if signatureStatus == "adhoc-invalid" && (strings.TrimSpace(msg) == "code has no resources but signature indicates they must be present" || (exitCode == 1 && strings.TrimSpace(msg) == "internal error in code signing subsystem") || (exitCode == 3 && strings.TrimSpace(msg) == "rejected")) {
		return "本地签名未封装完整，可移除隔离标记后尝试打开", false, nil
	}
	if signatureStatus != "unsigned" && signatureStatus != "adhoc" && signatureStatus != "adhoc-invalid" && signatureStatus != "valid" {
		return "系统评估结果无法确认", false, errors.New("无法解释系统评估结果，请从开发者处确认应用状态")
	}
	if strings.Contains(msg, "rejected") {
		if strings.TrimSpace(msg) == "rejected" && exitCode == 3 {
			switch signatureStatus {
			case "valid":
				return assessmentQuarantineOnly, false, nil
			case "adhoc":
				return "", true, nil
			}
		}
		if strings.Contains(msg, "no usable signature") || strings.Contains(msg, "unnotarized developer id") {
			return "未通过系统分发评估；处理不会提供 Apple 公证或安全认证", false, nil
		}
		if exitCode == 3 {
			return assessmentRejected, false, assessmentRejectedError
		}
		return assessmentUnavailable, false, assessmentUnavailableError
	}
	return assessmentUnavailable, false, assessmentUnavailableError
}

// Tool diagnostics prefix target paths on lines. Remove that exact prefix
// before recognizing status words so a crafted app name cannot look like a
// security verdict. Paths with embedded control characters are rejected.
func diagnosticText(output, path string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, path+": ")
	}
	return strings.Join(lines, "\n")
}

func signingEligibility(ctx context.Context, i *Inspection) string {
	if i.SignatureStatus != "unsigned" && i.SignatureStatus != "adhoc-invalid" {
		return "仅可为未签名或可修复本地签名的简单应用补签，现有开发者签名不会覆盖"
	}
	if err := trustedParentDirectories(i.Path); err != nil {
		return err.Error()
	}
	for _, e := range i.snapshot.Entries {
		if e.Mode&unix.S_IFMT == unix.S_IFLNK {
			return "此应用包含符号链接，当前版本仅支持移除隔离标记，暂不补签"
		}
		if e.UID != uint32(os.Geteuid()) || e.Mode&0200 == 0 || e.Mode&0022 != 0 {
			return "补签需要当前用户拥有并可写的完整应用"
		}
		p := strings.ToLower(e.Path)
		if strings.Contains(p, "_codesignature") || strings.Contains(p, ".entitlements") || strings.HasPrefix(p, "contents/frameworks") || strings.HasPrefix(p, "contents/plugins") || strings.HasPrefix(p, "contents/plug-ins") || strings.HasPrefix(p, "contents/xpcservices") || strings.HasPrefix(p, "contents/library") || strings.HasSuffix(p, ".app") {
			return "此应用包含嵌套代码或额外权限配置，当前版本暂不补签"
		}
		if e.Mode&unix.S_IFMT == unix.S_IFREG && e.Path != i.executable {
			if isMachO(filepath.Join(i.Path, e.Path)) || strings.HasPrefix(e.Path, "Contents/MacOS/") {
				return "当前版本仅支持单一 Mach-O 主程序"
			}
		}
	}
	if !isThin64MachO(filepath.Join(i.Path, i.executable)) {
		return "当前版本仅支持单一 Mach-O 主程序"
	}
	if err := notRunning(ctx, filepath.Join(i.Path, i.executable)); err != nil {
		return err.Error()
	}
	return ""
}

// codesign accepts a pathname, not a directory descriptor. Reject parents that
// untrusted users can rename through. The standard root:admin /Applications
// directory trusts administrators; root-owned sticky directories are safe
// because the bundle itself is checked as belonging to this user.
func trustedParentDirectories(path string) error {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		f, err := openAbsolute(parent)
		if err != nil {
			return err
		}
		var st unix.Stat_t
		err = unix.Fstat(int(f.Fd()), &st)
		f.Close()
		if err != nil {
			return err
		}
		if !trustedSigningParent(parent, st, adminGroupID) {
			return errors.New("补签需要位于当前用户或系统管理的目录，请先移动到个人应用目录")
		}
		if parent == "/" {
			return nil
		}
	}
}

func adminGroupID() (uint32, bool) {
	group, err := user.LookupGroup("admin")
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseUint(group.Gid, 10, 32)
	return uint32(id), err == nil
}

func trustedSigningParent(path string, st unix.Stat_t, lookupAdmin func() (uint32, bool)) bool {
	if st.Uid != 0 && st.Uid != uint32(os.Geteuid()) {
		return false
	}
	if st.Mode&0022 == 0 || st.Uid == 0 && st.Mode&unix.S_ISVTX != 0 {
		return true
	}
	if path == "/Applications" && st.Uid == 0 && st.Mode&0002 == 0 {
		gid, ok := lookupAdmin()
		return ok && st.Gid == gid
	}
	return false
}

func isMachO(path string) bool {
	f, err := macho.Open(path)
	if err == nil {
		f.Close()
		return true
	}
	fat, err := macho.OpenFat(path)
	if err == nil {
		fat.Close()
		return true
	}
	return false
}

func isThin64MachO(path string) bool {
	f, err := macho.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	return f.FileHeader.Magic == macho.Magic64 && f.FileHeader.Cpu == macho.CpuArm64 || f.FileHeader.Magic == macho.Magic64 && f.FileHeader.Cpu == macho.CpuAmd64
}

func notRunning(ctx context.Context, executable string) error {
	out, err := systemCommand(ctx, "/usr/sbin/lsof", "-t", "--", executable)
	if len(bytes.TrimSpace(out)) == 0 {
		var ex *exec.ExitError
		if errors.As(err, &ex) && ex.ExitCode() == 1 {
			return nil // lsof's documented "no files found" result.
		}
	}
	if err == nil {
		return errors.New("应用正在使用中，请退出应用后重新选择")
	}
	return errors.New("无法确认应用是否正在使用，暂不提供补签")
}
