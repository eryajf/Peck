package app

import (
	"context"
	"testing"
	"time"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/core"
	"github.com/eryajf/Peck/internal/vui"
	"github.com/vivaui/viva/ui"
)

func TestNativeMotionHonorsVisibilityAndReducedMotion(t *testing.T) {
	a := &application{state: pageProcessing, visible: true, prefs: preferences{Motion: true, Appearance: "paper"}, started: time.Now()}
	c := &ui.Canvas{}
	a.artwork(c, ui.Rect{Width: 400, Height: 180}, brand.Theme("paper", false))
	if len(c.Triangles()) == 0 && len(c.Rectangles()) == 0 {
		t.Fatal("scene did not paint")
	}
	a.hide()
	if a.visible {
		t.Fatal("hide did not clear visibility")
	}
	a.prefs.Motion = false
	a.artwork(c, ui.Rect{Width: 400, Height: 180}, brand.Theme("paper", false))
}

func TestHideKeepsProcessingAndResultState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	i := &core.Inspection{Path: "/fixture.app"}
	a := &application{state: pageProcessing, visible: true, inspection: i, cancel: cancel, trusted: true}
	a.hide()
	if a.visible || a.state != pageProcessing || a.inspection != i || !a.trusted || ctx.Err() != nil {
		t.Fatal("hiding changed processing state")
	}
	a.state = pageResult
	a.result = core.Result{Name: "fixture", Signature: "签名已验证", Changed: 3}
	a.hide()
	if a.state != pageResult || a.result.Changed != 3 {
		t.Fatal("hiding discarded result")
	}
}

func TestSceneStillPaintsWithReducedMotion(t *testing.T) {
	c := &ui.Canvas{}
	p := vui.NewPainter(c)
	brand.Scene(p, vui.Rect{W: 360, H: 170}, brand.Theme("paper", false), brand.SceneOptions{Mode: brand.SceneSuccess, Elapsed: brand.SuccessDuration, Animate: false})
	if len(c.Triangles()) == 0 {
		t.Fatal("static scene did not paint")
	}
}

func TestLateIconWakeIsKeptForLaunch(t *testing.T) {
	a := &application{
		state:            pageResult,
		targetPath:       "/Applications/Example.app",
		targetGeneration: 4,
		generation:       5,
	}
	a.wakes = []wakeEvent{{kind: wakeIcon, generation: 4, icon: brand.AppIcon}}
	a.applyWakes()
	if a.icon != brand.AppIcon {
		t.Fatal("icon arriving after processing was discarded")
	}
}

func TestIconWakeBeforeInspectionIsNotOverwritten(t *testing.T) {
	icon := vui.NewBitmapSource(ui.Bitmap{Pix: []byte{0, 255, 0, 255}, Width: 1, Height: 1, Stride: 4, ID: 1})
	a := &application{
		state:            pageInspect,
		targetPath:       "/Applications/Example.app",
		targetGeneration: 4,
		generation:       4,
	}
	a.wakes = []wakeEvent{
		{kind: wakeIcon, generation: 4, icon: icon},
		{kind: wakeInspect, generation: 4, inspection: &core.Inspection{Path: a.targetPath}},
	}
	a.applyWakes()
	if a.icon != icon {
		t.Fatal("icon arriving before inspection was overwritten by fallback")
	}
}

func TestStaleIconWakeCannotReplaceCurrentTarget(t *testing.T) {
	a := &application{
		state:            pageResult,
		targetPath:       "/Applications/Current.app",
		targetGeneration: 4,
		generation:       5,
	}
	a.wakes = []wakeEvent{{kind: wakeIcon, generation: 3, icon: brand.AppIcon}}
	a.applyWakes()
	if a.icon != nil {
		t.Fatal("stale icon wake replaced the current target")
	}
}

func TestFastProcessingFinishesPeckBurstWithoutDelayingResult(t *testing.T) {
	start := time.Unix(100, 0)
	for _, tt := range []struct {
		name      string
		now       time.Duration
		completed time.Duration
		animate   bool
		launch    bool
		cancelled bool
		err       error
		wantMode  brand.SceneMode
		wantStart time.Duration
	}{
		{name: "fast result", now: 200 * time.Millisecond, completed: 100 * time.Millisecond, animate: true, wantMode: brand.SceneProcessing},
		{name: "burst finished", now: brand.PeckBurstDuration, completed: 100 * time.Millisecond, animate: true, wantMode: brand.SceneSuccess, wantStart: brand.PeckBurstDuration},
		{name: "long operation", now: 2100 * time.Millisecond, completed: 2 * time.Second, animate: true, wantMode: brand.SceneSuccess, wantStart: 2 * time.Second},
		{name: "reduced motion", now: 200 * time.Millisecond, completed: 100 * time.Millisecond, wantMode: brand.SceneSuccess, wantStart: 100 * time.Millisecond},
		{name: "open immediately", now: 200 * time.Millisecond, completed: 100 * time.Millisecond, animate: true, launch: true, wantMode: brand.SceneLaunch, wantStart: 150 * time.Millisecond},
		{name: "cancelled", now: 200 * time.Millisecond, completed: 100 * time.Millisecond, animate: true, cancelled: true, wantMode: brand.SceneStopped},
		{name: "failed", now: 200 * time.Millisecond, completed: 100 * time.Millisecond, animate: true, err: context.DeadlineExceeded, wantMode: brand.SceneStopped},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := core.Result{CanOpen: tt.err == nil && !tt.cancelled, Error: tt.err, Cancelled: tt.cancelled}
			a := &application{
				state: pageResult, started: start, completed: start.Add(tt.completed),
				motionStarted: start, result: result,
			}
			if tt.launch {
				a.launchStarted = start.Add(150 * time.Millisecond)
			}
			mode, began := a.sceneTiming(start.Add(tt.now), tt.animate)
			if mode != tt.wantMode || !began.Equal(start.Add(tt.wantStart)) {
				t.Fatalf("scene = (%v, %s), want (%v, %s)", mode, began.Sub(start), tt.wantMode, tt.wantStart)
			}
			if a.state != pageResult || a.result != result {
				t.Fatal("animation delayed or changed the actual result")
			}
		})
	}
}

func BenchmarkPaintMain(b *testing.B) {
	a := &application{prefs: preferences{Motion: false, Appearance: "paper"}, visible: true, state: pageIdle}
	canvas := &ui.Canvas{}
	for b.Loop() {
		canvas.Reset()
		a.paintMain(canvas, ui.Size{Width: panelWidth, Height: panelHeight})
	}
}

func BenchmarkPaintSettings(b *testing.B) {
	for _, open := range []bool{false, true} {
		name := "closed"
		if open {
			name = "open"
		}
		b.Run(name, func(b *testing.B) {
			a := &application{prefs: preferences{Appearance: "rose"}}
			if open {
				a.openThemeSelect()
			}
			canvas := &ui.Canvas{}
			canvas.SetReduceMotion(true)
			size := ui.Size{Width: settingsWidth, Height: settingsHeight}
			a.paintSettings(canvas, size)
			b.ReportAllocs()
			for b.Loop() {
				canvas.Reset()
				a.paintSettings(canvas, size)
			}
		})
	}
}
