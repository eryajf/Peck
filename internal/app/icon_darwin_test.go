//go:build darwin

package app

import (
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eryajf/Peck/internal/core"
	vapp "github.com/vivaui/viva/app"
)

func TestLoadedIconReachesConfirmationCanvas(t *testing.T) {
	appPath := os.Getenv("PECK_ICON_APP")
	if appPath == "" {
		t.Skip("set PECK_ICON_APP to inspect a local application bundle")
	}
	icon := loadAppIcon(appPath)
	if icon == nil {
		t.Fatal("loadAppIcon returned nil")
	}
	iconValue := reflect.ValueOf(icon)
	if iconValue.Kind() == reflect.Ptr && !iconValue.IsNil() {
		bitmap := iconValue.Elem().FieldByName("bitmap")
		if bitmap.IsValid() {
			t.Logf("loaded icon bitmap=%dx%d", bitmap.FieldByName("Width").Int(), bitmap.FieldByName("Height").Int())
		}
	}
	a := &application{
		state: pageConfirm, visible: true, icon: icon, targetPath: appPath,
		prefs:      preferences{Appearance: "rose", Motion: false},
		inspection: &core.Inspection{Path: appPath, Name: "fixture", Version: "1.0"},
	}
	driver, err := vapp.NewDriver(vapp.Config{
		Width: panelWidth, Height: panelHeight, Tree: &tree, Paint: a.paintMain,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var bitmaps int
	for _, label := range driver.Snapshot().Labels {
		if len(label.Bitmap) > 0 {
			bitmaps++
			t.Logf("bitmap bounds=%+v source=%dx%d stride=%d bytes=%d id=%d upload=%t", label.Bounds, label.BitmapWidth, label.BitmapHeight, label.BitmapStride, len(label.Bitmap), label.BitmapID, label.BitmapUpload)
		}
	}
	if bitmaps == 0 {
		t.Fatal("confirmation canvas contains no application bitmap")
	}
}

func TestIconBitmapUsesPremultipliedBGRA(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	source.SetNRGBA(1, 0, color.NRGBA{R: 3, G: 2, B: 1, A: 255})

	bitmap, ok := iconBitmap(source, "/Applications/Example.app")
	if !ok {
		t.Fatal("iconBitmap rejected a valid image")
	}
	want := []byte{25, 50, 100, 128, 1, 2, 3, 255}
	for index, value := range want {
		if bitmap.Pix[index] != value {
			t.Fatalf("pixel byte %d = %d, want %d", index, bitmap.Pix[index], value)
		}
	}
}

func TestIconBitmapRoundsCornersAndPreservesPremultipliedTransparency(t *testing.T) {
	source := image.NewNRGBA(image.Rect(7, 11, 71, 75))
	ink := color.NRGBA{R: 200, G: 100, B: 50, A: 128}
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			source.SetNRGBA(x, y, ink)
		}
	}
	bitmap, ok := iconBitmap(source, "/fixture.app")
	if !ok {
		t.Fatal("iconBitmap rejected the fixture")
	}
	pixel := func(x, y int) []byte {
		offset := y*bitmap.Stride + x*4
		return bitmap.Pix[offset : offset+4]
	}
	for _, corner := range [][2]int{{0, 0}, {63, 0}, {0, 63}, {63, 63}} {
		if got := pixel(corner[0], corner[1]); !reflect.DeepEqual(got, []byte{0, 0, 0, 0}) {
			t.Errorf("corner %v = %v, want transparent", corner, got)
		}
	}
	for _, at := range [][2]int{{32, 32}, {32, 0}, {0, 32}} {
		if got := pixel(at[0], at[1]); !reflect.DeepEqual(got, []byte{25, 50, 100, 128}) {
			t.Errorf("interior/straight edge %v changed: %v", at, got)
		}
	}
	antialiased := 0
	for offset := 0; offset < len(bitmap.Pix); offset += 4 {
		channels := bitmap.Pix[offset : offset+4]
		if channels[3] > 0 && channels[3] < 128 {
			antialiased++
		}
		for _, channel := range channels[:3] {
			if channel > channels[3] {
				t.Fatal("corner mask left an unpremultiplied color fringe")
			}
		}
	}
	if antialiased == 0 {
		t.Fatal("rounded edges have no antialiasing")
	}
	if source.NRGBAAt(7, 11) != ink {
		t.Fatal("masking changed the source artwork")
	}
	again, _ := iconBitmap(source, "/fixture.app")
	if bitmap.ID != again.ID {
		t.Fatal("identical rounded icons have different cache identities")
	}
}

func TestLoadAppIconReadsBundlePNG(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "Example.app")
	resources := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleIconFile</key><string>Peck.png</string></dict></plist>`)
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), plist, 0o644); err != nil {
		t.Fatal(err)
	}
	icon, err := os.ReadFile(filepath.Join("..", "brand", "assets", "runtime", "app-icon.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "Peck.png"), icon, 0o644); err != nil {
		t.Fatal(err)
	}
	if loadAppIcon(app) == nil {
		t.Fatal("loadAppIcon did not decode a declared bundle PNG")
	}
}

func TestAppIconCandidatesPreferDeclaredResource(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "Example.app")
	resources := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleIconFile</key><string>AppIcon.icns</string></dict></plist>`)
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), plist, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AppIcon-source.png", "AppIcon.icns"} {
		if err := os.WriteFile(filepath.Join(resources, name), []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	candidates := appIconCandidates(t.Context(), app)
	if len(candidates) < 2 || filepath.Base(candidates[0]) != "AppIcon.icns" {
		t.Fatalf("declared icon was not preferred: %v", candidates)
	}
}

func TestLoadAppIconReadsBundleICNS(t *testing.T) {
	fixture := "/Applications/Motty.app/Contents/Resources/AppIcon.icns"
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("no installed ICNS fixture")
	}
	root := t.TempDir()
	app := filepath.Join(root, "Example.app")
	resources := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(fixture, filepath.Join(resources, "AppIcon.icns")); err != nil {
		t.Fatal(err)
	}
	plist := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleIconFile</key><string>AppIcon.icns</string></dict></plist>`)
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), plist, 0o644); err != nil {
		t.Fatal(err)
	}
	if loadAppIcon(app) == nil {
		t.Fatal("loadAppIcon did not convert a declared bundle ICNS")
	}
}

func TestLoadAppIconReadsConfiguredBundle(t *testing.T) {
	app := os.Getenv("PECK_ICON_APP")
	if app == "" {
		t.Skip("set PECK_ICON_APP to inspect a local application bundle")
	}
	t.Logf("icon candidates: %v", appIconCandidates(t.Context(), app))
	if loadAppIcon(app) == nil {
		t.Fatalf("loadAppIcon did not decode %s", app)
	}
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
