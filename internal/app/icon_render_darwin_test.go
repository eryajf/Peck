//go:build darwin && vivahead

package app

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/core"
	"github.com/eryajf/Peck/internal/vui"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestAppIconPixelsReachMetal(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 255, B: 255, A: 255})
		}
	}
	bitmap, _ := iconBitmap(source, "pixel-fixture")
	for _, mode := range []brand.SceneMode{brand.SceneReady, brand.SceneProcessing, brand.SceneSuccess, brand.SceneLaunch} {
		t.Run(map[brand.SceneMode]string{
			brand.SceneReady: "door", brand.SceneProcessing: "processing",
			brand.SceneSuccess: "success", brand.SceneLaunch: "flight",
		}[mode], func(t *testing.T) {
			session, err := headless.Open(vapp.Config{
				Width: 360, Height: 170,
				Paint: func(canvas *ui.Canvas, _ ui.Size) {
					brand.Scene(vui.NewPainter(canvas), vui.Rect{W: 360, H: 170}, brand.Theme("rose", false), brand.SceneOptions{
						Mode: mode, Animate: true, Elapsed: 900 * time.Millisecond, AppIcon: vui.NewBitmapSource(bitmap),
					})
				},
			}, nil, headless.Options{Scale: 2})
			if err != nil {
				if strings.Contains(err.Error(), "no Metal device") {
					t.Skipf("Metal offscreen renderer unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer session.Close()
			picture, err := session.Image()
			if err != nil {
				t.Fatal(err)
			}
			visible := 0
			for y := 0; y < picture.Bounds().Dy(); y++ {
				for x := 0; x < picture.Bounds().Dx(); x++ {
					pixel := picture.RGBAAt(x, y)
					if pixel.R > 240 && pixel.G < 15 && pixel.B > 240 {
						visible++
					}
				}
			}
			if visible < 1000 {
				t.Fatalf("only %d application icon pixels visible on Metal", visible)
			}
			for _, label := range session.Snapshot().Labels {
				if label.BitmapID != bitmap.ID || len(label.Bitmap) == 0 {
					continue
				}
				r := label.Bounds
				for _, at := range [][2]float32{
					{r.X + 1, r.Y + 1}, {r.X + r.Width - 2, r.Y + 1},
					{r.X + 1, r.Y + r.Height - 2}, {r.X + r.Width - 2, r.Y + r.Height - 2},
				} {
					pixel := picture.RGBAAt(int(at[0]*2), int(at[1]*2))
					if pixel.R > 240 && pixel.G < 15 && pixel.B > 240 {
						t.Fatalf("application icon has a square corner at %v", at)
					}
				}
			}
			if mode == brand.SceneReady {
				// This region is inside the opaque magenta icon. The green
				// insect must still cover its upper edge after compositing.
				insectPixels := 0
				for y := 65 * 2; y < 84*2; y++ {
					for x := 266 * 2; x < 289*2; x++ {
						pixel := picture.RGBAAt(x, y)
						if pixel.G > pixel.R && pixel.R < 220 {
							insectPixels++
						}
					}
				}
				if insectPixels < 80 {
					t.Fatalf("bitmap covered the insect: only %d green pixels remain", insectPixels)
				}
			}
		})
	}
}

// An opt-in local check reads an installed app's artwork without processing or
// launching it, and exports the actual Metal-rendered confirmation and flight.
func TestInstalledAppIconPreview(t *testing.T) {
	path := os.Getenv("PECK_ICON_APP")
	if path == "" {
		t.Skip("set PECK_ICON_APP to inspect a local application's icon")
	}
	icon := loadAppIcon(path)
	if icon == nil {
		t.Fatal("installed application icon was not loaded")
	}
	a := &application{
		state: pageConfirm, visible: true, icon: icon, targetPath: path,
		prefs: preferences{Appearance: "rose", Motion: true},
		inspection: &core.Inspection{
			Path: path, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Version: "1.14.0",
			SignatureStatus: "adhoc", QuarantineCount: 14,
		},
	}
	session, err := headless.Open(vapp.Config{
		Width: panelWidth, Height: panelHeight, WindowMode: vapp.WindowFrameless,
		ClearColor: transparentClearColor(), Paint: a.paintMain, Invalidator: &a.mainInv,
	}, nil, headless.Options{Scale: 2})
	if err != nil {
		if strings.Contains(err.Error(), "no Metal device") {
			t.Skipf("Metal offscreen renderer unavailable: %v", err)
		}
		t.Fatal(err)
	}
	defer session.Close()
	dir := os.Getenv("PECK_ICON_PREVIEW_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.Screenshot(filepath.Join(dir, "confirmation.png")); err != nil {
		t.Fatal(err)
	}
	for _, preview := range []struct {
		name        string
		sign, trust bool
	}{
		{"confirmation-trust.png", false, true},
		{"confirmation-sign.png", true, false},
		{"confirmation-both.png", true, true},
	} {
		a.inspection.CanSign, a.prefs.RequireTrust = preview.sign, preview.trust
		a.invalidate()
		if err := session.Screenshot(filepath.Join(dir, preview.name)); err != nil {
			t.Fatal(err)
		}
	}
	a.state = pageResult
	a.result = core.Result{CanOpen: true}
	a.completed = time.Now().Add(-brand.SuccessDuration)
	a.invalidate()
	if err := session.Screenshot(filepath.Join(dir, "opened-door.png")); err != nil {
		t.Fatal(err)
	}
	a.launchStarted = time.Now().Add(-1100 * time.Millisecond)
	a.invalidate()
	if err := session.Screenshot(filepath.Join(dir, "flight.png")); err != nil {
		t.Fatal(err)
	}
	a.launchStarted = time.Now().Add(-1700 * time.Millisecond)
	a.invalidate()
	if err := session.Screenshot(filepath.Join(dir, "flight-trail.png")); err != nil {
		t.Fatal(err)
	}
	t.Logf("Metal previews written to %s", dir)
}
