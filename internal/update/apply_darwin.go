//go:build darwin

package update

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	applyReady           = "ready\n"
	applyReadyTimeout    = 5 * time.Second
	applyExitTimeout     = 60 * time.Second
	applyErrorSuffix     = ".error"
	applyStartupSuffix   = ".started"
	applyReadyDescriptor = "--ready-fd"
)

var (
	applyWaitForExit  = waitForExit
	applyCopyBundle   = copyBundle
	applyRestart      = restartApplication
	applyPersistError = persistApplyError
	applyWaitStartup  = waitForStartup
)

func StartApply(packagePath string) error {
	if filepath.Ext(strings.ToLower(packagePath)) != ".zip" {
		return errors.New("update package must be a ZIP archive")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	bundle := appBundlePath(exe)
	if bundle == "" {
		return errors.New("current Peck is not running from an application bundle")
	}
	parent := filepath.Dir(bundle)
	if !writable(parent) {
		return errors.New("the current application folder is not writable")
	}
	staging := filepath.Join(filepath.Dir(packagePath), "Peck-update-staged")
	_ = os.RemoveAll(staging)
	if err := extractBundle(packagePath, staging); err != nil {
		return err
	}
	candidate := filepath.Join(staging, filepath.Base(bundle))
	if _, err := os.Stat(filepath.Join(candidate, "Contents", "Info.plist")); err != nil {
		return errors.New("update ZIP does not contain Peck.app")
	}
	if err := verifyBundle(candidate); err != nil {
		return err
	}
	if err := verifySameTeam(bundle, candidate); err != nil {
		return err
	}
	helper := filepath.Join(filepath.Dir(packagePath), "Peck-update-helper.app")
	if err := copyBundle(bundle, helper); err != nil {
		return err
	}
	helperExe := filepath.Join(helper, "Contents", "MacOS", filepath.Base(exe))
	readyReader, readyWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer readyReader.Close()
	cmd := exec.Command(helperExe, "--apply-update", "--pid", fmt.Sprint(os.Getpid()), "--staged", candidate, "--target", bundle, "--package", packagePath, applyReadyDescriptor, "3")
	cmd.Stdout, cmd.Stderr = nil, nil
	cmd.ExtraFiles = []*os.File{readyWriter}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		readyWriter.Close()
		return err
	}
	readyWriter.Close()
	if err := waitForReady(readyReader, applyReadyTimeout); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Process.Release()
		return fmt.Errorf("update helper did not become ready: %w", err)
	}
	return cmd.Process.Release()
}

func RunApply(args []string) error { return runApply(args) }

func runApply(args []string) error {
	values := map[string]string{}
	for index := 0; index+1 < len(args); index += 2 {
		values[args[index]] = args[index+1]
	}
	pid := 0
	if _, err := fmt.Sscanf(values["--pid"], "%d", &pid); err != nil || pid <= 0 {
		return errors.New("invalid update process ID")
	}
	staged, target, packagePath := values["--staged"], values["--target"], values["--package"]
	if staged == "" || target == "" {
		return errors.New("update job is incomplete")
	}
	if fdText := values[applyReadyDescriptor]; fdText != "" {
		var fd int
		if _, err := fmt.Sscanf(fdText, "%d", &fd); err != nil || fd < 0 {
			return errors.New("invalid update ready descriptor")
		}
		if err := signalReady(fd); err != nil {
			return err
		}
	}
	if err := applyWaitForExit(context.Background(), pid); err != nil {
		// The parent may still be alive; never start another copy in this case.
		if packagePath != "" {
			_ = applyPersistError(packagePath+applyErrorSuffix, err)
		}
		return err
	}
	parent := filepath.Dir(target)
	next, old := filepath.Join(parent, ".Peck.app.new"), filepath.Join(parent, ".Peck.app.old")
	if err := recoverPreviousBackup(target, old); err != nil {
		// A backup is never removed just because a retry is attempted. It is
		// safe to discard it only after this installation has confirmed that
		// the new process reached its ready state.
		return applyFailure(packagePath, target, err)
	}
	_ = os.RemoveAll(next)
	if err := applyCopyBundle(staged, next); err != nil {
		return applyFailure(packagePath, target, err)
	}
	if err := os.Rename(target, old); err != nil {
		_ = os.RemoveAll(next)
		return applyFailure(packagePath, target, err)
	}
	if err := os.Rename(next, target); err != nil {
		rollbackErr := os.Rename(old, target)
		if rollbackErr != nil {
			err = fmt.Errorf("%w (rollback failed: %v)", err, rollbackErr)
		}
		return applyFailure(packagePath, target, err)
	}
	// Keep the previous bundle until Launch Services accepts the new one.
	if packagePath != "" {
		_ = os.Remove(packagePath + applyStartupSuffix)
		// The record ties a retained backup to the exact prior transaction,
		// even if a later retry uses a different release package path.
		if err := os.WriteFile(old+".transaction", []byte(packagePath+applyStartupSuffix), 0600); err != nil {
			if removeErr := os.RemoveAll(target); removeErr != nil {
				return applyFailure(packagePath, target, errors.Join(err, removeErr))
			}
			if rollbackErr := os.Rename(old, target); rollbackErr != nil {
				return applyFailure(packagePath, target, errors.Join(err, rollbackErr))
			}
			return applyFailure(packagePath, target, err)
		}
	}
	launchErr := applyRestart(target)
	if launchErr == nil && packagePath != "" {
		if err := applyWaitStartup(packagePath+applyStartupSuffix, 30*time.Second); err != nil {
			// Launch Services accepted the request: an unconfirmed process may
			// still be alive and own the single-instance lock. Never replace its
			// bundle underneath it; preserve both copies for manual recovery.
			_ = applyPersistError(packagePath+applyErrorSuffix, err)
			return err
		}
	}
	if err := launchErr; err != nil {
		failed := filepath.Join(parent, ".Peck.app.failed")
		_ = os.RemoveAll(failed)
		if moveErr := os.Rename(target, failed); moveErr != nil {
			return applyFailure(packagePath, target, errors.Join(err, moveErr))
		}
		if rollbackErr := os.Rename(old, target); rollbackErr != nil {
			// Keep a valid app at the original path if restoring the old copy
			// failed; retain the backup for manual recovery.
			if restoreNewErr := os.Rename(failed, target); restoreNewErr != nil {
				err = errors.Join(err, fmt.Errorf("restore new bundle: %w", restoreNewErr))
			}
			return applyFailure(packagePath, target, errors.Join(err, rollbackErr))
		}
		_ = os.RemoveAll(failed)
		_ = os.Remove(old + ".transaction")
		return applyFailure(packagePath, target, err)
	}
	// The new process explicitly confirmed that its UI event loop is ready.
	_ = os.RemoveAll(old)
	_ = os.Remove(old + ".transaction")
	_ = os.RemoveAll(filepath.Dir(staged))
	if packagePath != "" {
		_ = os.Remove(packagePath)
		_ = os.Remove(packagePath + applyErrorSuffix)
		_ = os.Remove(packagePath + applyStartupSuffix)
	}
	return nil
}

func recoverPreviousBackup(target, old string) error {
	if _, err := os.Lstat(old); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	markerData, err := os.ReadFile(old + ".transaction")
	if err != nil {
		return fmt.Errorf("previous update backup exists and its transaction is unknown: %w", err)
	}
	marker := string(markerData)
	if !filepath.IsAbs(marker) || !strings.HasSuffix(marker, ".zip"+applyStartupSuffix) {
		return errors.New("previous update backup has invalid transaction marker")
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		return fmt.Errorf("previous update backup exists and startup was not confirmed: %w", err)
	}
	if string(data) != applyReady {
		return fmt.Errorf("previous update backup exists and startup confirmation is invalid")
	}
	if _, err := os.Lstat(target); err != nil {
		return fmt.Errorf("cannot recover previous update without current bundle: %w", err)
	}
	if err := os.RemoveAll(old); err != nil {
		return fmt.Errorf("remove confirmed update backup: %w", err)
	}
	_ = os.Remove(marker)
	_ = os.Remove(old + ".transaction")
	return nil
}

func applyFailure(packagePath, target string, cause error) error {
	if packagePath != "" {
		_ = applyPersistError(packagePath+applyErrorSuffix, cause)
	}
	_ = applyRestart(target)
	return cause
}

func waitForStartup(path string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if data, err := os.ReadFile(path); err == nil && string(data) == applyReady {
			return nil
		}
		select {
		case <-timer.C:
			return errors.New("updated application did not confirm startup")
		case <-ticker.C:
		}
	}
}

// ConfirmStartup is called by the new application after its primary window is
// ready. The helper accepts the marker only for the version it just installed.
func ConfirmStartup(packagePath, expected, current string) error {
	if packagePath == "" || expected == "" || compare(current, expected) != ComparisonUpToDate {
		return nil
	}
	path := packagePath + applyStartupSuffix
	file, err := os.CreateTemp(filepath.Dir(path), ".peck-started-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.WriteString(applyReady); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func waitForReady(reader *os.File, timeout time.Duration) error {
	result := make(chan error, 1)
	go func() {
		b := make([]byte, len(applyReady))
		n, err := io.ReadFull(reader, b)
		if err != nil {
			result <- err
			return
		}
		if string(b[:n]) != applyReady {
			result <- errors.New("invalid helper ready message")
			return
		}
		result <- nil
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-result:
		return err
	case <-timer.C:
		return errors.New("timed out waiting for helper readiness")
	}
}

func signalReady(fd int) error {
	file := os.NewFile(uintptr(fd), "update-ready")
	if file == nil {
		return errors.New("invalid update ready descriptor")
	}
	defer file.Close()
	_, err := io.WriteString(file, applyReady)
	return err
}

func persistApplyError(path string, err error) error {
	if err == nil {
		return nil
	}
	temporary := path + ".tmp"
	if writeErr := os.WriteFile(temporary, []byte(err.Error()+"\n"), 0o600); writeErr != nil {
		return writeErr
	}
	if renameErr := os.Rename(temporary, path); renameErr != nil {
		_ = os.Remove(temporary)
		return renameErr
	}
	return nil
}

func restartApplication(path string) error {
	cmd := exec.Command("/usr/bin/open", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launch application: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func waitForExit(ctx context.Context, pid int) error {
	deadline := time.NewTimer(applyExitTimeout)
	defer deadline.Stop()
	for {
		if err := exec.Command("/bin/kill", "-0", fmt.Sprint(pid)).Run(); err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("timed out waiting for Peck to exit")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func appBundlePath(exe string) string {
	path := exe
	for path != "/" && path != "." {
		if strings.HasSuffix(path, ".app") {
			return path
		}
		path = filepath.Dir(path)
	}
	return ""
}

func writable(dir string) bool {
	file, err := os.CreateTemp(dir, ".peck-update-")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	return os.Remove(name) == nil
}

func verifyBundle(bundle string) error {
	if output, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", bundle).CombinedOutput(); err != nil {
		return fmt.Errorf("updated application signature is invalid: %s", strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("/usr/sbin/spctl", "--assess", "--type", "execute", bundle).CombinedOutput(); err != nil {
		return fmt.Errorf("macOS rejected the updated application: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func verifySameTeam(current, candidate string) error {
	team := func(bundle string) (string, error) {
		output, err := exec.Command("/usr/bin/codesign", "-dvv", bundle).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("could not inspect application signature: %s", strings.TrimSpace(string(output)))
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "TeamIdentifier=") {
				value := strings.TrimSpace(strings.TrimPrefix(line, "TeamIdentifier="))
				if value != "" && value != "not set" {
					return value, nil
				}
			}
		}
		return "", errors.New("application has no Developer ID team identifier")
	}
	currentTeam, err := team(current)
	if err != nil {
		return err
	}
	candidateTeam, err := team(candidate)
	if err != nil {
		return err
	}
	if currentTeam != candidateTeam {
		return fmt.Errorf("updated application is signed by team %s, want %s", candidateTeam, currentTeam)
	}
	return nil
}

func extractBundle(archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	root := filepath.Clean(destination) + string(os.PathSeparator)
	if len(reader.File) > 10000 {
		return errors.New("update ZIP contains too many entries")
	}
	var expanded uint64
	for _, file := range reader.File {
		if file.UncompressedSize64 > 2<<30 || expanded > (2<<30)-file.UncompressedSize64 {
			return errors.New("update ZIP expands beyond the size limit")
		}
		expanded += file.UncompressedSize64
		if file.Mode()&os.ModeSymlink != 0 || (!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
			return fmt.Errorf("update ZIP contains symlink %q", file.Name)
		}
		name := filepath.Clean(filepath.FromSlash(file.Name))
		if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || strings.Contains(file.Name, `\`) || !strings.HasPrefix(filepath.Join(destination, name)+string(os.PathSeparator), root) {
			return fmt.Errorf("update ZIP contains unsafe path %q", file.Name)
		}
	}
	return exec.Command("/usr/bin/ditto", "-x", "-k", archive, destination).Run()
}

func copyBundle(source, destination string) error {
	_ = os.RemoveAll(destination)
	return exec.Command("/usr/bin/ditto", source, destination).Run()
}
