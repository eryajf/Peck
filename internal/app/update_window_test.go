package app

import (
	"strings"
	"testing"

	"github.com/eryajf/Peck/internal/update"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func availableUpdate() update.Result {
	return update.Result{CurrentVersion: "0.0.9", LatestVersion: "0.1.0", Comparison: update.ComparisonUpdateAvailable,
		Release: update.Release{Version: "0.1.0", URL: projectURL + "/releases/tag/v0.1.0", Notes: "## 更新说明\n\n- 修复问题\n- 优化体验"}}
}

func TestUpdateCheckOpensIndependentWindowWithoutSettings(t *testing.T) {
	a, host := reopenTestApplication(t)
	a.ensureMessages()
	a.settings.Close()
	a.settings = nil
	a.hide()
	a.currentVersion = "0.0.9"
	before := len(host.windows)
	a.publish(wakeEvent{kind: wakeUpdateCheck, update: availableUpdate()})
	a.applyWakes()
	window := a.updateWindow
	if window == nil || window.Hidden() || len(host.windows) != before+1 {
		t.Fatal("background check did not present an independent update window")
	}
	config := host.configs[len(host.configs)-1]
	if config.WindowMode != vapp.WindowCustomTitlebar || config.Paint == nil || config.OnWake == nil {
		t.Fatal("update prompt is not a retained, non-transient native window")
	}
	if a.settings != nil || a.visible {
		t.Fatal("update prompt opened settings or changed the hidden workspace")
	}
	if host.front != "Peck · 更新" {
		t.Fatalf("front window = %q", host.front)
	}
	a.openUpdateWindow()
	if len(host.windows) != before+1 || a.updateWindow != window {
		t.Fatal("repeated check created a duplicate update window")
	}
	a.dismissUpdateWindow()
	if !window.Closed() || a.updateWindow != nil || a.updatePromptDismissedVersion != "0.1.0" {
		t.Fatal("Later did not close and release the update window")
	}
	a.publish(wakeEvent{kind: wakeUpdateCheck, update: availableUpdate()})
	a.applyWakes()
	if a.updateWindow != nil {
		t.Fatal("same version prompted again after Later")
	}
}

func TestUpdateWindowRendersMarkdownScrollsAndLinks(t *testing.T) {
	a := &application{currentVersion: "0.0.9", prefs: preferences{Appearance: "paper"}, updateResult: availableUpdate()}
	a.ensureMessages()
	a.updateResult.Release.Notes += strings.Repeat("\n\n- 更多更新内容", 35)
	driver, err := vapp.NewDriver(a.updateWindowConfig(), nil)
	mustThemeEvent(t, err)
	defer driver.Close()
	for _, label := range driver.Snapshot().Labels {
		if strings.HasPrefix(label.Value, "## ") {
			t.Fatal("release notes were rendered as raw Markdown")
		}
	}
	if a.updateNotes.maximum <= 0 {
		t.Fatal("long release notes did not have a scrollable viewport")
	}
	mustThemeEvent(t, driver.Key(ui.KeyPageDown, 0))
	if a.updateNotes.offset <= 0 {
		t.Fatal("PageDown did not scroll update notes")
	}
	clickThemeHit(t, driver, idUpdatePromptRelease)
	url, ok := driver.LastOpenedURL()
	if !ok || url != a.updateResult.Release.URL {
		t.Fatalf("release link = %q, ok=%t", url, ok)
	}
	// The ordinary settings painter never declares update-dialog actions.
	settings, err := vapp.NewDriver(a.settingsConfig(), nil)
	mustThemeEvent(t, err)
	defer settings.Close()
	for _, hit := range settings.Snapshot().Hits {
		if hit.ID == idUpdatePromptLater || hit.ID == idUpdatePromptDownload {
			t.Fatal("independent window actions leaked into settings")
		}
	}
}

func TestUpdateWindowShowsReadyAction(t *testing.T) {
	a := &application{currentVersion: "0.0.9", updateResult: availableUpdate(), updateReadyOK: true,
		updateReady: update.ReadyState{Version: "0.1.0"}}
	a.ensureMessages()
	driver, err := vapp.NewDriver(a.updateWindowConfig(), nil)
	mustThemeEvent(t, err)
	defer driver.Close()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == idUpdatePromptDownload {
			if hit.Label != a.messages.ApplyUpdate {
				t.Fatalf("ready action = %q", hit.Label)
			}
			return
		}
	}
	t.Fatal("missing primary update action")
}

func BenchmarkPaintUpdateWindow(b *testing.B) {
	a := &application{currentVersion: "0.0.9", prefs: preferences{Appearance: "paper"}, updateResult: availableUpdate()}
	canvas := &ui.Canvas{}
	a.paintUpdateWindow(canvas, ui.Size{Width: updateWindowWidth, Height: updateWindowHeight})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		canvas.Reset()
		a.paintUpdateWindow(canvas, ui.Size{Width: updateWindowWidth, Height: updateWindowHeight})
	}
}
