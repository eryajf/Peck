package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/eryajf/Peck/internal/i18n"
	"github.com/eryajf/Peck/internal/update"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestSettingsShowsAboutAndUpdateControls(t *testing.T) {
	a := &application{
		prefs:          preferences{Motion: false, Appearance: "rose", AutoCheckUpdate: true, UpdateSource: update.SourceGitHub},
		language:       i18n.Chinese,
		messages:       i18n.For(i18n.Chinese),
		currentVersion: "0.1.0",
		prefsPath:      filepath.Join(t.TempDir(), "preferences.json"),
	}
	driver, err := vapp.NewDriver(a.settingsConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	labels := driver.Snapshot().Labels
	for _, want := range []string{"版本  Peck 0.1.0", "项目地址", "捐赠支持", "自动检查新版本", "更新渠道", "检查更新"} {
		found := false
		for _, label := range labels {
			if label.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("settings did not show %q; labels=%v", want, labels)
		}
	}
}

func TestSettingsUpdateSourceClickPersists(t *testing.T) {
	a := &application{
		prefs:          preferences{Motion: false, Appearance: "rose", AutoCheckUpdate: true, UpdateSource: update.SourceGitHub},
		language:       i18n.Chinese,
		messages:       i18n.For(i18n.Chinese),
		currentVersion: "0.1.0",
		prefsPath:      filepath.Join(t.TempDir(), "preferences.json"),
	}
	driver, err := vapp.NewDriver(a.settingsConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	id := tree.Sub(settingsFeature, tree.Name("update-source-option")).At(1)
	var bounds ui.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			bounds = hit.Bounds
			break
		}
	}
	if bounds.Width == 0 {
		t.Fatalf("CNB option hit was not registered")
	}
	if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if a.prefs.UpdateSource != update.SourceCNB {
		t.Fatalf("UpdateSource = %q, want %q", a.prefs.UpdateSource, update.SourceCNB)
	}
	data, err := os.ReadFile(a.prefsPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved preferences
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.UpdateSource != update.SourceCNB {
		t.Fatalf("persisted source = %q", saved.UpdateSource)
	}
	if a.updateCancel != nil {
		a.updateCancel()
	}
}
