package brand

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/eryajf/Peck/internal/vui"
	ui "github.com/vivaui/viva/ui"
)

// The PNGs are the approved exported masters, kept outside the Go source so
// the same files can be audited and replaced independently of the painter.
//
//go:embed assets/runtime/app-icon.png
var appIconPNG []byte

//go:embed assets/runtime/menubar.png
var menuBarPNG []byte

var (
	markColorSource       = vui.NewIconSource(PeckMarkColor, vui.Hex("#FFFFFF"))
	markReverseSource     = vui.NewIconSource(PeckMarkReverse, vui.Hex("#FFFFFF"))
	lockupColorSource     = vui.NewIconSource(PeckLockupHorizontalColor, vui.Hex("#FFFFFF"))
	lockupReverseSource   = vui.NewIconSource(PeckLockupHorizontalReverse, vui.Hex("#FFFFFF"))
	wordmarkColorSource   = vui.NewIconSource(PeckWordmark, vui.Hex("#FFFFFF"))
	wordmarkReverseSource = vui.NewIconSource(PeckWordmarkReverse, vui.Hex("#FFFFFF"))
	identityOnce          sync.Once
	identityBitmap        *vui.BitmapSource
	appPixels             []byte
	appSize               int
	menuPixels            []byte
	menuSize              int
)

// MarkSource keeps the full-color bird mark visible on both light and dark
// surfaces. The wordmark has its own reverse source for dark headers.
func MarkSource(dark bool) vui.ImageSource {
	_ = dark
	return markColorSource
}

// LockupSource is the approved horizontal bird plus path wordmark used where
// the compact header has enough room for the complete identity.
func LockupSource(dark bool) vui.ImageSource {
	if dark {
		return lockupReverseSource
	}
	return lockupColorSource
}

// WordmarkSource returns the path wordmark with a light fill in dark themes.
func WordmarkSource(dark bool) vui.ImageSource {
	if dark {
		return wordmarkReverseSource
	}
	return wordmarkColorSource
}

func embeddedAppIcon() vui.ImageSource {
	identityOnce.Do(loadIdentityAssets)
	return identityBitmap
}

// AppIconPixels returns the straight RGBA application icon used by Viva's
// WindowIcon and by the macOS Dock fallback. The returned slice is immutable.
func AppIconPixels() (size int, rgba []byte) {
	identityOnce.Do(loadIdentityAssets)
	return appSize, appPixels
}

// MenuBarPixels returns the alpha template at its native 18pt@2x resolution.
func MenuBarPixels() (size int, rgba []byte) {
	identityOnce.Do(loadIdentityAssets)
	return menuSize, menuPixels
}

func loadIdentityAssets() {
	appImage, err := png.Decode(bytes.NewReader(appIconPNG))
	if err == nil {
		appSize, appPixels = straightRGBA(appImage)
		identityBitmap = vui.NewBitmapSource(bitmapFromImage(appImage, 0x5045434b))
	}
	menuImage, err := png.Decode(bytes.NewReader(menuBarPNG))
	if err == nil {
		menuSize, menuPixels = straightRGBA(menuImage)
	}
	if identityBitmap == nil {
		// The exported asset is checked during the build. This fallback only keeps
		// tests and a partially copied checkout drawable.
		identityBitmap = vui.NewBitmapSource(ui.Bitmap{Pix: []byte{0, 0, 0, 0}, Width: 1, Height: 1, Stride: 4, ID: 0x5045434b, Version: 1})
	}
}

func straightRGBA(source image.Image) (int, []byte) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := color.NRGBAModel.Convert(source.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			r, g, b, a := pixel.R, pixel.G, pixel.B, pixel.A
			i := (y*width + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = r, g, b, a
		}
	}
	return width, pixels
}

func bitmapFromImage(source image.Image, id uint64) ui.Bitmap {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := color.NRGBAModel.Convert(source.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			r, g, b, a := pixel.R, pixel.G, pixel.B, pixel.A
			alpha := uint16(a)
			i := (y*width + x) * 4
			// Viva bitmaps are premultiplied BGRA.
			pixels[i] = uint8(uint16(b) * alpha / 255)
			pixels[i+1] = uint8(uint16(g) * alpha / 255)
			pixels[i+2] = uint8(uint16(r) * alpha / 255)
			pixels[i+3] = a
		}
	}
	return ui.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4, ID: id, Version: 1}
}
