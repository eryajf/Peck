package brand

import (
	"testing"
	"time"

	"github.com/eryajf/Peck/internal/vui"
	"github.com/vivaui/viva/ui"
)

func TestNormalizeAppearanceKeepsThemeCompatibility(t *testing.T) {
	tests := map[string]string{
		"":         "system",
		"unknown":  "system",
		"system":   "system",
		"light":    "paper",
		"dark":     "midnight",
		"paper":    "paper",
		"midnight": "midnight",
		"mist":     "mist",
		"clay":     "clay",
		"ink":      "ink",
		"aurora":   "aurora",
		"rose":     "rose",
		"sunset":   "sunset",
		"ocean":    "ocean",
		"lavender": "lavender",
		"citrus":   "citrus",
	}
	for input, want := range tests {
		if got := NormalizeAppearance(input); got != want {
			t.Errorf("NormalizeAppearance(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAppearanceOptionsExposeDistinctThemes(t *testing.T) {
	options := AppearanceOptions()
	if len(options) != 12 {
		t.Fatalf("AppearanceOptions() returned %d options, want 12", len(options))
	}
	seen := make(map[string]bool, len(options))
	for _, option := range options {
		if option.Value == "" || option.Label == "" || option.Description == "" {
			t.Fatalf("theme option is missing copy: %+v", option)
		}
		if seen[option.Value] {
			t.Fatalf("theme option %q is duplicated", option.Value)
		}
		seen[option.Value] = true
	}
}

func TestPeckAngleEmitsThreeQuickImpacts(t *testing.T) {
	peaks := 0
	wasHigh := false
	for millis := int64(0); millis <= PeckBurstDuration.Milliseconds(); millis += 5 {
		angle := PeckAngle(time.Duration(millis) * time.Millisecond)
		high := angle > 17
		if high && !wasHigh {
			peaks++
		}
		wasHigh = high
	}
	if peaks != 3 {
		t.Fatalf("PeckAngle produced %d impacts, want 3", peaks)
	}
	if got := PeckAngle(1200 * time.Millisecond); got != 0 {
		t.Fatalf("PeckAngle did not rest after the three impacts: %v", got)
	}
}

func TestMarkPeckAngleUsesThreeSmallNodsAndRest(t *testing.T) {
	peaks := 0
	wasHigh := false
	for millis := int64(0); millis <= 1800; millis += 5 {
		angle := MarkPeckAngle(time.Duration(millis) * time.Millisecond)
		high := angle > 8
		if high && !wasHigh {
			peaks++
		}
		wasHigh = high
	}
	if peaks != 3 {
		t.Fatalf("MarkPeckAngle produced %d forward nods, want 3", peaks)
	}
	if got := MarkPeckAngle(1200 * time.Millisecond); got != 0 {
		t.Fatalf("MarkPeckAngle did not rest after the nods: %v", got)
	}
	if got := MarkPeckAngle(150 * time.Millisecond); got <= 0 {
		t.Fatalf("MarkPeckAngle did not lean forward during the first nod: %v", got)
	}
}

func TestMarkPeckImpactHasThreeShortWindows(t *testing.T) {
	peaks := 0
	wasVisible := false
	for millis := int64(0); millis <= 1800; millis += 5 {
		alpha := MarkPeckImpact(time.Duration(millis) * time.Millisecond)
		visible := alpha > .5
		if visible && !wasVisible {
			peaks++
		}
		wasVisible = visible
	}
	if peaks != 3 {
		t.Fatalf("MarkPeckImpact produced %d impact windows, want 3", peaks)
	}
	if got := MarkPeckImpact(330 * time.Millisecond); got != 0 {
		t.Fatalf("MarkPeckImpact remained visible in the gap after the first tap: %v", got)
	}
	if got := MarkPeckImpact(590 * time.Millisecond); got <= .5 {
		t.Fatalf("MarkPeckImpact did not show the third tap: %v", got)
	}
}

func TestSceneKeepsApplicationArtworkOnLaunchPath(t *testing.T) {
	canvas := &ui.Canvas{}
	painter := vui.NewPainter(canvas)
	Scene(painter, vui.Rect{W: 360, H: 170}, Theme("paper", false), SceneOptions{
		Mode: SceneLaunch, Insect: 0, Elapsed: 900 * time.Millisecond, Animate: true, AppIcon: AppIcon,
	})
	if len(canvas.Images()) != 1 {
		t.Fatalf("launch scene painted %d bitmaps, want the one flying app icon", len(canvas.Images()))
	}
	image := canvas.Images()[0]
	if image.Bounds.Width <= 0 || image.Bounds.Height <= 0 {
		t.Fatalf("launch icon has empty bounds: %+v", image.Bounds)
	}

	canvas.Reset()
	Scene(painter, vui.Rect{W: 360, H: 170}, Theme("paper", false), SceneOptions{
		Mode: SceneLaunch, Insect: 0, Elapsed: 2500 * time.Millisecond, Animate: true, AppIcon: AppIcon,
	})
	if len(canvas.Images()) != 0 {
		t.Fatal("launch scene left a duplicate app icon after departure")
	}
}
