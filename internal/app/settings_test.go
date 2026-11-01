package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eryajf/Peck/internal/brand"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestThemePreviewRepaintsBothWindowsAndRollsBack(t *testing.T) {
	a, main, settings := themePreviewDrivers(t)
	original, err := os.ReadFile(a.prefsPath)
	mustThemeEvent(t, err)
	for _, exit := range []string{"leave option", "leave window", "escape", "tab", "dismiss", "close settings", "minimize"} {
		t.Run(exit, func(t *testing.T) {
			if !a.themeOpen {
				clickThemeHit(t, settings, idTheme)
			}
			mustThemeEvent(t, settings.Advance(250*time.Millisecond))
			point := themeHitCenter(t, settings, themeID(t, "sunset"))
			mustThemeEvent(t, settings.Move(point.X, point.Y))
			assertWindowThemes(t, main, settings, "sunset")
			if a.prefs.Appearance != "rose" {
				t.Fatal("hover changed the saved theme")
			}
			for _, hit := range settings.Snapshot().Hits {
				if hit.ID == themeID(t, "sunset") && hit.Selected {
					t.Fatal("hover moved the saved selection indicator")
				}
			}
			switch exit {
			case "leave option":
				mustThemeEvent(t, settings.Move(30, 130))
			case "leave window":
				mustThemeEvent(t, settings.Leave())
			case "escape":
				mustThemeEvent(t, settings.Key(ui.KeyEscape, 0))
			case "tab":
				mustThemeEvent(t, settings.Key(ui.KeyTab, 0))
			case "dismiss":
				mustThemeEvent(t, settings.Click(30, 130))
			case "close settings":
				mustThemeEvent(t, settings.Chord(ui.ModifierMeta, "w"))
			case "minimize":
				mustThemeEvent(t, settings.SetMinimized(true))
				mustThemeEvent(t, settings.SetMinimized(false))
			}
			assertWindowThemes(t, main, settings, "rose")
			data, err := os.ReadFile(a.prefsPath)
			mustThemeEvent(t, err)
			if !bytes.Equal(data, original) {
				t.Fatal("preview or cancellation wrote preferences")
			}
		})
	}
}

func TestThemeClickCommitsPreviewAndLaterHoverRestoresCommittedTheme(t *testing.T) {
	a, main, settings := themePreviewDrivers(t)
	clickThemeHit(t, settings, idTheme)
	mustThemeEvent(t, settings.Advance(250*time.Millisecond))
	clickThemeHit(t, settings, themeID(t, "sunset"))
	assertWindowThemes(t, main, settings, "sunset")
	data, err := os.ReadFile(a.prefsPath)
	mustThemeEvent(t, err)
	var saved preferences
	mustThemeEvent(t, json.Unmarshal(data, &saved))
	if saved.Appearance != "sunset" || a.prefs.Appearance != "sunset" || a.themeOpen || a.themePreview != "" {
		t.Fatalf("click did not commit and end preview: saved=%q, current=%q, preview=%q, open=%t", saved.Appearance, a.prefs.Appearance, a.themePreview, a.themeOpen)
	}
	clickThemeHit(t, settings, idTheme)
	mustThemeEvent(t, settings.Advance(250*time.Millisecond))
	point := themeHitCenter(t, settings, themeID(t, "midnight"))
	mustThemeEvent(t, settings.Move(point.X, point.Y))
	assertWindowThemes(t, main, settings, "midnight")
	mustThemeEvent(t, settings.Leave())
	assertWindowThemes(t, main, settings, "sunset")
	after, err := os.ReadFile(a.prefsPath)
	mustThemeEvent(t, err)
	if !bytes.Equal(data, after) {
		t.Fatal("later preview overwrote the committed theme")
	}
}

func themePreviewDrivers(t *testing.T) (*application, *vapp.Driver, *vapp.Driver) {
	t.Helper()
	// Disable artwork motion: a hover must wake the main window itself, not
	// merely wait for an animation frame. Writes stay in a temporary directory.
	a := &application{prefs: preferences{Appearance: "rose", Motion: false}, visible: true, prefsPath: filepath.Join(t.TempDir(), "preferences.json")}
	a.savePreferences()
	main, err := vapp.NewDriver(vapp.Config{
		Width: panelWidth, Height: panelHeight, Tree: &tree,
		Paint: a.paintMain, Invalidator: &a.mainInv,
	}, nil)
	mustThemeEvent(t, err)
	t.Cleanup(main.Close)
	settings, err := vapp.NewDriver(a.settingsConfig(), nil)
	mustThemeEvent(t, err)
	t.Cleanup(settings.Close)
	return a, main, settings
}

func assertWindowThemes(t *testing.T, main, settings *vapp.Driver, appearance string) {
	t.Helper()
	// Pump only applies pending invalidations; it cannot force a missing main
	// repaint, so these assertions cover the cross-window notification.
	mustThemeEvent(t, main.Pump())
	mustThemeEvent(t, settings.Pump())
	dark := appearance == "midnight" || appearance == "ocean"
	theme := brand.Theme(appearance, dark)
	for _, check := range []struct {
		driver *vapp.Driver
		label  string
		color  ui.Color
	}{
		{main, "准备工作区", vivaColor(theme.Accent)},
		{settings, "让 Peck 按你的习惯工作", vivaColor(theme.Text)},
	} {
		found := false
		for _, label := range check.driver.Snapshot().Labels {
			if label.Value == check.label {
				found = true
				if label.Color != check.color {
					t.Fatalf("%q did not repaint in %s: got %+v, want %+v", check.label, appearance, label.Color, check.color)
				}
				break
			}
		}
		if !found {
			t.Fatalf("missing label %q", check.label)
		}
	}
}

func themeID(t *testing.T, value string) ui.ID {
	t.Helper()
	for index, option := range settingsThemes {
		if option.Value == value {
			return themeOptionScope.At(index)
		}
	}
	t.Fatalf("missing theme %q", value)
	return ui.ID{}
}

func themeHitCenter(t *testing.T, driver *vapp.Driver, id ui.ID) ui.Point {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			return ui.Point{X: hit.Bounds.X + hit.Bounds.Width/2, Y: hit.Bounds.Y + hit.Bounds.Height/2}
		}
	}
	t.Fatalf("missing hit %v", id)
	return ui.Point{}
}

func clickThemeHit(t *testing.T, driver *vapp.Driver, id ui.ID) {
	t.Helper()
	point := themeHitCenter(t, driver, id)
	mustThemeEvent(t, driver.Click(point.X, point.Y))
}

func mustThemeEvent(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
