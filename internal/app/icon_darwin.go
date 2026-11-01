//go:build darwin

package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/vui"
	"github.com/vivaui/viva/ui"
)

// loadAppIcon reads the bundle's declared icon and converts macOS icon masters
// into a bitmap that Viva can retain. It runs off the window thread; callers
// keep a built-in mark visible until this best-effort lookup completes.
func loadAppIcon(path string) vui.ImageSource {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "peck-icon-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)
	for index, candidate := range appIconCandidates(ctx, path) {
		decoded, decodeErr := decodeIconCandidate(ctx, candidate, dir, index)
		if decodeErr != nil {
			continue
		}
		if bitmap, ok := iconBitmap(decoded, path); ok {
			return vui.NewBitmapSource(bitmap)
		}
	}
	return nil
}

type iconCandidate struct {
	path  string
	rank  int
	order int
}

// appIconCandidates prefers a resource named after the bundle, then explicit
// icon-like resources. Some Electron bundles declare a generic electron.icns
// while shipping a product-specific icon alongside it, so directory ranking
// is intentionally allowed to beat that generic declaration.
func appIconCandidates(ctx context.Context, path string) []string {
	resources := filepath.Join(path, "Contents", "Resources")
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ToLower(base)
	candidates := make([]iconCandidate, 0, 12)
	seen := make(map[string]bool)
	order := 0
	add := func(candidate string, rank int) {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			return
		}
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			return
		}
		seen[candidate] = true
		candidates = append(candidates, iconCandidate{path: candidate, rank: rank, order: order})
		order++
	}
	addName := func(name string, rank int) {
		name = strings.TrimSpace(name)
		if name == "" || name == "." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
			return
		}
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".icns", ".png":
			add(filepath.Join(resources, name), rank)
		default:
			add(filepath.Join(resources, name+".icns"), rank)
			add(filepath.Join(resources, name+".png"), rank)
		}
	}
	for _, name := range plistIconNames(ctx, filepath.Join(path, "Contents", "Info.plist")) {
		// A declared AppIcon.icns is the bundle's source of truth. Keep
		// product-specific files named after the bundle ahead of a generic
		// declaration such as Electron.icns, while still preferring the
		// declared resource over auxiliary files such as AppIcon-source.png.
		addName(name, 5)
	}
	entries, err := os.ReadDir(resources)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".icns" && ext != ".png" {
				continue
			}
			lower := strings.ToLower(entry.Name())
			rank := 100
			if base != "" && strings.Contains(lower, base) {
				rank = 0
			} else if strings.Contains(lower, "icon") {
				rank = 10
			} else if strings.Contains(lower, "app") {
				rank = 20
			}
			add(filepath.Join(resources, entry.Name()), rank)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].rank != candidates[j].rank {
			return candidates[i].rank < candidates[j].rank
		}
		return candidates[i].order < candidates[j].order
	})
	paths := make([]string, len(candidates))
	for i, candidate := range candidates {
		paths[i] = candidate.path
	}
	return paths
}

func plistIconNames(ctx context.Context, path string) []string {
	cmd := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", path)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C"}
	cmd.Stderr = io.Discard
	var output bytes.Buffer
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return nil
	}
	var info map[string]any
	if err := json.Unmarshal(output.Bytes(), &info); err != nil {
		return nil
	}
	var names []string
	seen := make(map[string]bool)
	add := func(value any) {}
	add = func(value any) {
		switch value := value.(type) {
		case string:
			value = strings.TrimSpace(value)
			if value != "" && !seen[value] {
				seen[value] = true
				names = append(names, value)
			}
		case []any:
			for _, item := range value {
				add(item)
			}
		}
	}
	// Product-specific icon base names should win over generic Electron names.
	for key, value := range info {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "icon") && strings.Contains(lower, "base") {
			add(value)
		}
	}
	for _, key := range []string{"CFBundleIconName", "CFBundleIconFile", "CFBundleIconFiles"} {
		add(info[key])
	}
	return names
}

func decodeIconCandidate(ctx context.Context, path, dir string, index int) (image.Image, error) {
	input := path
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".icns" || ext == ".png" {
		output := filepath.Join(dir, fmt.Sprintf("icon-%d.png", index))
		cmd := exec.CommandContext(ctx, "/usr/bin/sips", "-s", "format", "png", "-z", "128", "128", path, "--out", output)
		cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C"}
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return nil, err
		}
		input = output
	}
	file, err := os.Open(input)
	if err != nil {
		return nil, err
	}
	decoded, err := png.Decode(file)
	_ = file.Close()
	return decoded, err
}

func iconBitmap(source image.Image, path string) (ui.Bitmap, bool) {
	bounds := source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return ui.Bitmap{}, false
	}
	width, height := bounds.Dx(), bounds.Dy()
	radius := float64(min(width, height)) * brand.AppIconCornerRatio
	pixels := make([]byte, width*height*4)
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(path))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := source.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			index := (y*width + x) * 4
			// color.Color.RGBA returns premultiplied channels. Viva expects
			// those channels in BGRA byte order.
			pixels[index] = uint8(b >> 8)
			pixels[index+1] = uint8(g >> 8)
			pixels[index+2] = uint8(r >> 8)
			pixels[index+3] = uint8(a >> 8)
			// Mask once at load time. Premultiplied color and alpha must fade
			// together, and the same retained pixels follow the icon in flight.
			dx := math.Max(radius-float64(x)-.5, float64(x)+.5-float64(width)+radius)
			dy := math.Max(radius-float64(y)-.5, float64(y)+.5-float64(height)+radius)
			if dx > 0 && dy > 0 {
				coverage := min(1.0, max(0.0, radius+.5-math.Hypot(dx, dy)))
				for channel := 0; channel < 4; channel++ {
					pixels[index+channel] = uint8(float64(pixels[index+channel])*coverage + .5)
				}
			}
		}
	}
	var idBytes [8]byte
	binary.LittleEndian.PutUint64(idBytes[:], hash.Sum64())
	_, _ = hash.Write(pixels)
	return ui.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4, ID: binary.LittleEndian.Uint64(idBytes[:]) ^ hash.Sum64(), Version: 1}, true
}
