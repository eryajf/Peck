package app

import (
	"time"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/vui"
	"github.com/vivaui/viva/ui"
)

// artwork keeps the same native scene and timing as the original version. Viva's
// frame scheduler is used only while the scene is visible and moving.
func (a *application) artwork(canvas *ui.Canvas, r ui.Rect, theme *vui.Theme) {
	now := time.Now()
	animate := a.visible && a.prefs.Motion && !canvas.ReduceMotion() && !a.stopping
	mode, started := a.sceneTiming(now, animate)
	icon := a.icon
	if icon == nil && (a.targetPath != "" || a.inspection != nil) {
		icon = brand.AppIcon
	}
	elapsed := time.Duration(0)
	if !started.IsZero() {
		elapsed = now.Sub(started)
	}
	moving := mode == brand.SceneIdle || mode == brand.SceneInspect || mode == brand.SceneProcessing
	if mode == brand.SceneSuccess && elapsed < brand.SuccessDuration {
		moving = true
	}
	if mode == brand.SceneLaunch && elapsed < brand.LaunchDuration {
		moving = true
	}
	if animate && moving {
		canvas.RequestFrameIn(time.Second / 30)
	}
	p := vui.NewPainter(canvas)
	brand.Scene(p, vui.Rect{X: r.X, Y: r.Y, W: r.Width, H: r.Height}, theme, brand.SceneOptions{Mode: mode, Insect: a.insectKind, Elapsed: elapsed, Animate: animate, AppIcon: icon})
}

func (a *application) sceneTiming(now time.Time, animate bool) (brand.SceneMode, time.Time) {
	mode, started := brand.SceneIdle, a.motionStarted
	if a.state == pageInspect {
		mode, started = brand.SceneInspect, a.started
	}
	if a.state == pageConfirm {
		mode, started = brand.SceneReady, time.Time{}
	}
	if a.state == pageProcessing {
		mode, started = brand.SceneProcessing, a.started
	}
	if a.state == pageResult {
		mode = brand.SceneStopped
		if a.result.Error == nil && !a.result.Cancelled {
			mode, started = brand.SceneSuccess, a.completed
			// A fast operation may finish between two animation frames. Finish
			// the first burst visually without delaying the result or its actions.
			if animate && !a.started.IsZero() && !a.completed.IsZero() {
				burstEnd := a.started.Add(brand.PeckBurstDuration)
				if burstEnd.After(a.completed) {
					started = burstEnd
					if now.Before(burstEnd) {
						mode, started = brand.SceneProcessing, a.started
					}
				}
			}
			if !a.launchStarted.IsZero() {
				mode, started = brand.SceneLaunch, a.launchStarted
			}
		}
	}
	return mode, started
}
