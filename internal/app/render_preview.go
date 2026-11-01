//go:build vivahead

package app

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/core"
	"github.com/eryajf/Peck/internal/i18n"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// renderPreview paints the real Viva canvas through the framework's headless
// driver. Pixel export requires Viva's optional offscreen renderer, so callers
// should use `go run -tags vivahead . --render-preview <dir>` on macOS.
func renderPreview(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	a := &application{prefs: preferences{Motion: false, Appearance: "paper"}, language: i18n.Chinese, messages: i18n.For(i18n.Chinese), visible: true, motionStarted: time.Now()}
	write := func(name string, width, height int, paint func(*ui.Canvas, ui.Size)) error {
		config := vapp.Config{Title: "Peck", Width: width, Height: height, WindowMode: vapp.WindowFrameless, ClearColor: transparentClearColor(), Paint: paint}
		canvas := &ui.Canvas{}
		session, err := headless.Open(config, canvas, headless.Options{Scale: 2})
		if err != nil {
			return err
		}
		defer session.Close()
		path := filepath.Join(dir, name)
		if err := session.Screenshot(path); err == nil {
			return nil
		} else if !errors.Is(err, headless.ErrNoRenderer) {
			// Keep the useful diagnostic while retaining a portable fallback.
			_ = err
		}
		picture := rasterizePreview(canvas, width, height, 2)
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		encodeErr := png.Encode(file, picture)
		if closeErr := file.Close(); encodeErr == nil {
			encodeErr = closeErr
		}
		return encodeErr
	}
	mainPaint := func(canvas *ui.Canvas, size ui.Size) { a.paintMain(canvas, size) }
	if err := write("01-idle.png", panelWidth, panelHeight, mainPaint); err != nil {
		return err
	}
	a.inspection = &core.Inspection{Path: "/Users/you/Applications/Example.app", Name: "Example App", Version: "1.2.3", BundleID: "org.example.app", SignatureStatus: "unsigned", QuarantineCount: 3, CanSign: true}
	a.targetPath = a.inspection.Path
	a.state = pageConfirm
	a.link = &core.LinkRequest{Project: "Example App", BundleID: "org.example.app"}
	if err := write("02-confirm.png", panelWidth, panelHeight, mainPaint); err != nil {
		return err
	}
	a.state = pageProcessing
	a.started = time.Now().Add(-260 * time.Millisecond)
	a.progress = core.Progress{Stage: "quarantine", Message: a.messages.ProgressQuarantine}
	if err := write("03-processing.png", panelWidth, panelHeight, mainPaint); err != nil {
		return err
	}
	a.state = pageResult
	a.completed = time.Now().Add(-brand.SuccessDuration)
	a.result = core.Result{Name: "Example App", Signature: "未选择补签", Quarantine: "已移除 3 处隔离标记", Verification: "已通过", Launch: "尚未请求打开", Changed: 3, CanOpen: true}
	if err := write("04-result.png", panelWidth, panelHeight, mainPaint); err != nil {
		return err
	}
	a.state, a.targetPath, a.inspection, a.link = pageIdle, "", nil, nil
	a.prefs.Appearance = "midnight"
	if err := write("05-night-forest.png", panelWidth, panelHeight, mainPaint); err != nil {
		return err
	}
	a.prefs.Appearance = "ocean"
	if err := write("06-deep-sea.png", panelWidth, panelHeight, mainPaint); err != nil {
		return err
	}
	a.prefs.Appearance = "midnight"
	return write("07-settings.png", settingsWidth, settingsHeight, func(canvas *ui.Canvas, size ui.Size) {
		a.paintSettings(canvas, size)
	})
}

// rasterizePreview is a portable emergency renderer for development exports.
// Viva's Metal offscreen path remains the authoritative screenshot renderer;
// this path keeps previews useful on CI and machines without a Metal device.
func rasterizePreview(canvas *ui.Canvas, width, height, scale int) *image.RGBA {
	picture := image.NewRGBA(image.Rect(0, 0, width*scale, height*scale))
	// Canvas keeps rectangles, triangles and shadows in separate slices, but
	// records the cross-slice positions on triangles and shadows. Replay that
	// ordering here so a background gradient cannot overwrite a later button
	// or the scene artwork (the portable fallback is used when Metal is absent).
	// This mirrors Viva's painter ordering without introducing a second renderer.
	rectangles, triangles, shadows := canvas.Rectangles(), canvas.Triangles(), canvas.Shadows()
	nextTriangle, nextShadow := 0, 0
	drawBefore := func(rectangleCount int) {
		for {
			triangleDue := nextTriangle < len(triangles) && int(triangles[nextTriangle].RectsBefore) <= rectangleCount
			shadowDue := nextShadow < len(shadows) && int(shadows[nextShadow].RectsBefore) <= rectangleCount
			if shadowDue && (!triangleDue || int(shadows[nextShadow].TrianglesBefore) <= nextTriangle) {
				shadow := shadows[nextShadow]
				nextShadow++
				r := shadow.Bounds
				r.X += shadow.Style.OffsetX - shadow.Style.Blur
				r.Y += shadow.Style.OffsetY - shadow.Style.Blur
				r.Width += 2 * shadow.Style.Blur
				r.Height += 2 * shadow.Style.Blur
				fillPreviewRect(picture, scaleRect(r, scale), shadow.Color, shadow.Radius*float32(scale))
				continue
			}
			if !triangleDue {
				return
			}
			drawPreviewTriangle(picture, triangles[nextTriangle], scale)
			nextTriangle++
		}
	}
	for index, rectangle := range rectangles {
		drawBefore(index)
		fillPreviewRect(picture, scaleRect(rectangle.Bounds, scale), rectangle.Color, rectangle.Radius*float32(scale))
	}
	drawBefore(len(rectangles))
	for _, text := range canvas.Texts() {
		drawPreviewText(picture, text, scale)
	}
	return picture
}

func scaleRect(rect ui.Rect, scale int) ui.Rect {
	f := float32(scale)
	return ui.Rect{X: rect.X * f, Y: rect.Y * f, Width: rect.Width * f, Height: rect.Height * f}
}

func previewColor(c ui.Color, alpha float32) color.NRGBA {
	alpha *= c.Alpha
	alpha = maxPreview(0, minPreview(1, alpha))
	return color.NRGBA{R: uint8(maxPreview(0, minPreview(1, c.Red))*255 + .5), G: uint8(maxPreview(0, minPreview(1, c.Green))*255 + .5), B: uint8(maxPreview(0, minPreview(1, c.Blue))*255 + .5), A: uint8(alpha*255 + .5)}
}

func fillPreviewRect(dst *image.RGBA, rect ui.Rect, c ui.Color, radius float32) {
	if rect.Width <= 0 || rect.Height <= 0 || c.Alpha <= 0 {
		return
	}
	minX, minY := maxIntPreview(0, int(rect.X)), maxIntPreview(0, int(rect.Y))
	maxX, maxY := minIntPreview(dst.Bounds().Max.X, int(math.Ceil(float64(rect.X+rect.Width)))), minIntPreview(dst.Bounds().Max.Y, int(math.Ceil(float64(rect.Y+rect.Height))))
	radius = minPreview(radius, minPreview(rect.Width, rect.Height)/2)
	ink := previewColor(c, 1)
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			if radius > 0 && !insideRoundedPreview(float32(x)+.5, float32(y)+.5, rect, radius) {
				continue
			}
			blendPreview(dst, x, y, ink)
		}
	}
}

func insideRoundedPreview(x, y float32, rect ui.Rect, radius float32) bool {
	if radius <= 0 || (x >= rect.X+radius && x <= rect.X+rect.Width-radius) || (y >= rect.Y+radius && y <= rect.Y+rect.Height-radius) {
		return true
	}
	cx, cy := rect.X+radius, rect.Y+radius
	if x > rect.X+rect.Width-radius {
		cx = rect.X + rect.Width - radius
	}
	if y > rect.Y+rect.Height-radius {
		cy = rect.Y + rect.Height - radius
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

func drawPreviewTriangle(dst *image.RGBA, triangle ui.Triangle, scale int) {
	points := [3]ui.Point{}
	for i, point := range triangle.Points {
		points[i] = ui.Point{X: point.X * float32(scale), Y: point.Y * float32(scale)}
	}
	minX := maxIntPreview(0, int(math.Floor(float64(minPointPreview(points[0].X, points[1].X, points[2].X)))))
	minY := maxIntPreview(0, int(math.Floor(float64(minPointPreview(points[0].Y, points[1].Y, points[2].Y)))))
	maxX := minIntPreview(dst.Bounds().Max.X, int(math.Ceil(float64(maxPointPreview(points[0].X, points[1].X, points[2].X)))))
	maxY := minIntPreview(dst.Bounds().Max.Y, int(math.Ceil(float64(maxPointPreview(points[0].Y, points[1].Y, points[2].Y)))))
	denom := (points[1].Y-points[2].Y)*(points[0].X-points[2].X) + (points[2].X-points[1].X)*(points[0].Y-points[2].Y)
	if math.Abs(float64(denom)) < 1e-6 {
		return
	}
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			px, py := float32(x)+.5, float32(y)+.5
			u := ((points[1].Y-points[2].Y)*(px-points[2].X) + (points[2].X-points[1].X)*(py-points[2].Y)) / denom
			v := ((points[2].Y-points[0].Y)*(px-points[2].X) + (points[0].X-points[2].X)*(py-points[2].Y)) / denom
			w := 1 - u - v
			if u < 0 || v < 0 || w < 0 {
				continue
			}
			c := triangle.Color
			if triangle.Shaded {
				c = ui.Color{Red: c.Red*w + triangle.Shade[0].Red*u + triangle.Shade[1].Red*v, Green: c.Green*w + triangle.Shade[0].Green*u + triangle.Shade[1].Green*v, Blue: c.Blue*w + triangle.Shade[0].Blue*u + triangle.Shade[1].Blue*v, Alpha: c.Alpha*w + triangle.Shade[0].Alpha*u + triangle.Shade[1].Alpha*v}
			}
			blendPreview(dst, x, y, previewColor(c, 1))
		}
	}
}

var previewFont struct {
	sync.Mutex
	loaded bool
	font   *opentype.Font
}

func drawPreviewText(dst *image.RGBA, text ui.Text, scale int) {
	face := previewFontFace(float64(maxPreview(9, text.Style.Size)) * float64(scale))
	if closer, ok := face.(io.Closer); ok {
		defer closer.Close()
	}
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(previewColor(text.Style.Color, 1)), Face: face, Dot: fixedPointPreview(text.Bounds.X*float32(scale), (text.Bounds.Y+text.Style.Size)*float32(scale))}
	d.DrawString(text.Value)
}

func previewFontFace(size float64) font.Face {
	previewFont.Lock()
	defer previewFont.Unlock()
	if !previewFont.loaded {
		previewFont.loaded = true
		for _, path := range []string{
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/SFNS.ttf",
			"/System/Library/Fonts/CJKSymbolsFallback.ttc",
		} {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			collection, err := opentype.ParseCollection(data)
			if err != nil || collection.NumFonts() == 0 {
				continue
			}
			previewFont.font, _ = collection.Font(0)
			if previewFont.font != nil {
				break
			}
		}
	}
	if previewFont.font != nil {
		if face, err := opentype.NewFace(previewFont.font, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull}); err == nil {
			return face
		}
	}
	return basicfont.Face7x13
}

func fixedPointPreview(x, y float32) fixed.Point26_6 {
	return fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
}
func blendPreview(dst *image.RGBA, x, y int, src color.NRGBA) {
	draw.Draw(dst, image.Rect(x, y, x+1, y+1), image.NewUniform(src), image.Point{}, draw.Over)
}
func minPreview(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
func maxPreview(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
func minPointPreview(a, b, c float32) float32 { return minPreview(a, minPreview(b, c)) }
func maxPointPreview(a, b, c float32) float32 { return maxPreview(a, maxPreview(b, c)) }
func minIntPreview(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxIntPreview(a, b int) int {
	if a > b {
		return a
	}
	return b
}
