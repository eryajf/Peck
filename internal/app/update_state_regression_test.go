package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eryajf/Peck/internal/update"
	vapp "github.com/vivaui/viva/app"
)

func TestRestoreUpdateStateRejectsOldVersions(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "old.zip")
	if err := os.WriteFile(pkg, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	store := update.NewStateStore(filepath.Join(dir, "state.json"))
	old := availableUpdate()
	if err := store.SaveLastCheck(old); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReady(update.ReadyState{Version: "0.1.0", Path: pkg}); err != nil {
		t.Fatal(err)
	}
	a := &application{currentVersion: "0.2.0", updateStore: store}
	a.restoreUpdateState()
	if a.updateReadyOK || a.updateResult.Comparison == update.ComparisonUpdateAvailable {
		t.Fatal("restored an older update")
	}
	if _, ok := store.Ready(); ok {
		t.Fatal("stale ready state persisted")
	}
	if _, err := os.Stat(pkg); err != nil {
		t.Fatal("must not delete the cached file during state cleanup", err)
	}
}

func TestRestoreUpdateStateRewritesCachedSource(t *testing.T) {
	store := update.NewStateStore(filepath.Join(t.TempDir(), "state.json"))
	r := availableUpdate()
	r.Release.Asset = &update.Asset{DownloadURL: projectURL + "/releases/download/v0.1.0/a.zip", ChecksumURL: projectURL + "/releases/download/v0.1.0/SHA256SUMS"}
	if err := store.SaveLastCheck(r); err != nil {
		t.Fatal(err)
	}
	a := &application{currentVersion: "0.0.9", prefs: preferences{UpdateSource: update.SourceCNB}, updateStore: store}
	a.restoreUpdateState()
	if got := a.updateResult.Release.Asset.DownloadURL; got != "https://cnb.cool/znb/Peck/-/releases/download/v0.1.0/a.zip" {
		t.Fatalf("source = %s", got)
	}
}

func TestStaleCheckCannotOverrideNewChannel(t *testing.T) {
	a := &application{currentVersion: "0.0.9", updateCheckGeneration: 2}
	a.publish(wakeEvent{kind: wakeUpdateCheck, generation: 1, update: availableUpdate()})
	a.applyWakes()
	if a.updateResult.Comparison != "" {
		t.Fatal("stale result accepted")
	}
}

func TestApplyUpdateFailureKeepsApplicationRunning(t *testing.T) {
	a, host := reopenTestApplication(t)
	a.currentVersion = "0.0.9"
	a.updateReadyOK = true
	a.updateReady = update.ReadyState{Version: "0.1.0", Path: filepath.Join(t.TempDir(), "update.zip")}
	if err := os.WriteFile(a.updateReady.Path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	old := startUpdateApply
	called := false
	startUpdateApply = func(path string) error { called = path == a.updateReady.Path; return errors.New("fixture failure") }
	defer func() { startUpdateApply = old }()
	a.applyUpdate()
	if !called || a.quitting || a.updateError != "fixture failure" || len(host.windows) == 0 {
		t.Fatal("apply failure not surfaced safely")
	}
}

func TestCanceledDownloadDoesNotBecomeReady(t *testing.T) {
	a := &application{updateDownloading: true, updateDownloadCancelRequested: true, updateResult: availableUpdate()}
	a.ensureMessages()
	a.publish(wakeEvent{kind: wakeUpdateDownload, update: availableUpdate(), updatePath: filepath.Join(t.TempDir(), "done.zip")})
	a.applyWakes()
	if a.updateReadyOK || a.updateDownloading || a.updateError != a.messages.UpdateDownloadCanceled {
		t.Fatalf("canceled state: %+v", a.updateReady)
	}
}

func TestBusySettingsDisablesApply(t *testing.T) {
	a := &application{state: pageProcessing, currentVersion: "0.0.9", updateReadyOK: true, updateReady: update.ReadyState{Version: "0.1.0", Path: "/never/open"}}
	a.ensureMessages()
	d, err := vapp.NewDriver(a.settingsConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, hit := range d.Snapshot().Hits {
		if hit.ID == idCheckUpdate {
			if !hit.Disabled {
				t.Fatal("apply enabled while busy")
			}
			return
		}
	}
	t.Fatal("missing action")
}
