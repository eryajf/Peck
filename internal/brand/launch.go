package brand

import (
	"math"

	ui "github.com/eryajf/Peck/internal/vui"
)

const (
	launchWingsEnd = .28
	launchRiseEnd  = .72
	launchExit     = 2.18
	farewellRaised = launchExit + .24
	farewellWaved  = farewellRaised + .72
)

// One sampled flight drives both the icon and the bird's response. The bird
// follows the icon's actual location instead of anticipating it on a timer.
type launchMotion struct {
	x, y, size, angle, spread, flap, lift float32
	seconds                               float64
	animate, visible                      bool
}

func launchMotionAt(seconds float64, animate bool) launchMotion {
	if !animate {
		return launchMotion{x: 241, y: 25, size: 38, angle: -6, spread: 1, flap: 25, lift: 1, visible: true}
	}
	seconds = max(0, min(seconds, LaunchDuration.Seconds()))
	lift := smoothScene((seconds - launchWingsEnd) / (launchRiseEnd - launchWingsEnd))
	u := float32(max(0, min((seconds-launchRiseEnd)/(launchExit-launchRiseEnd), 1)))
	travel := u * u
	// Rise beside the door before accelerating left, leaving clearance
	// above the bird's crown throughout the flight.
	motion := launchMotion{
		x:       289 - 13*lift - 330*travel,
		y:       89 - 47*lift - 55*travel - 6*float32(math.Sin(float64(travel)*math.Pi)),
		size:    50 - 3*lift - 17*travel,
		angle:   -8*lift - 8*travel,
		spread:  smoothScene(seconds / launchWingsEnd),
		flap:    -18,
		lift:    lift,
		seconds: seconds,
		animate: true,
		visible: seconds < launchExit,
	}
	if seconds > launchWingsEnd {
		// A quicker downstroke and slower return give the feathers a less
		// metronomic beat than rotating both wings with a plain sine wave.
		phase := math.Mod((seconds-launchWingsEnd)*5.6, 1)
		beat := smoothScene(phase / .4)
		if phase >= .4 {
			beat = 1 - smoothScene((phase-.4)/.6)
		}
		motion.flap += 76 * beat
	}
	return motion
}

func launchBirdPose(motion launchMotion) scenePose {
	hop := float32(math.Sin(float64(motion.lift) * math.Pi))
	pose := scenePose{
		x: 133 - 32*motion.lift, y: 31 - 5*hop,
		facing: 1, headFacing: 1, wingSwing: -8 * hop,
		happy: true, crownColor: 1,
	}
	if motion.animate {
		// Once the icon exits, raise the near wing, wave twice while it
		// stays above the shoulder, then fold it back. Separate lift and
		// wave phases keep the gesture readable at the scene's small size.
		pose.wingLift = smoothScene((motion.seconds - launchExit) / (farewellRaised - launchExit))
		pose.wingLift *= 1 - smoothScene((motion.seconds-farewellWaved)/(LaunchDuration.Seconds()-farewellWaved))
		waveProgress := max(0, min((motion.seconds-farewellRaised)/(farewellWaved-farewellRaised), 1))
		wave := float32(math.Sin(waveProgress * math.Pi * 2))
		pose.wingWave = wave * wave * pose.wingLift
		pose.bob -= 1.2*pose.wingLift + .9*pose.wingWave
	}
	// Watch the wings grow while facing the door. Retreat and look upward
	// on the same beat as liftoff; turn left only as the icon passes overhead.
	headX := pose.x + .8*80
	watch := smoothScene(float64((headX + 48 - motion.x) / 82))
	pose.headFacing = 1 - 1.85*watch
	pose.angle = -5 - 7*motion.lift + 27*watch
	return pose
}

// paintLaunchingIcon gives the one doorway icon a pair of wings and lets it
// fly out of this local scene. It does not animate the target app's window or
// represent a successful launch. Reduced motion uses a separate static pose.
func paintLaunchingIcon(p *ui.Painter, t *ui.Theme, icon ui.ImageSource, motion launchMotion) {
	if icon == nil {
		icon = AppIcon
	}
	if motion.animate {
		paintFlightTrail(p, t, motion.seconds)
	}
	if motion.visible {
		paintWingedIcon(p, t, icon, motion.x, motion.y, motion.size, motion.spread, motion.flap, motion.angle)
	}
}

func paintWingedIcon(p *ui.Painter, t *ui.Theme, icon ui.ImageSource, x, y, size, spread, flap, angle float32) {
	p.Transform(ui.Rotation(angle), x, y, func() {
		paintIconWing(p, t, x, y, size, -1, spread, flap)
		paintIconWing(p, t, x, y, size, 1, spread, flap)
		frame := ui.Rect{X: x - size/2, Y: y - size/2, W: size, H: size}
		badge := ui.Rect{X: frame.X - 2, Y: frame.Y - 2, W: frame.W + 4, H: frame.H + 4}
		p.Shadow(badge, badge.W*AppIconCornerRatio, 0, 2, 4, 0, t.Text.Alpha(.12))
		p.Fill(badge, ui.Hex("#FFF9E8").Alpha(.9), badge.W*AppIconCornerRatio)
		// Wings sit behind the icon, leaving the actual application artwork
		// intact and unique throughout growth, takeoff and departure.
		p.Image(icon, frame, ui.Contain)
	})
}

func paintIconWing(p *ui.Painter, t *ui.Theme, x, y, size, side, spread, flap float32) {
	if spread <= .001 {
		return
	}
	scale := size / 48
	rootX, rootY := x+side*size*.4, y+scale*2
	length := 32 * scale * spread
	depth := 28 * scale * spread
	pointX := func(v float32) float32 { return rootX + side*length*v }
	pointY := func(v float32) float32 { return rootY + depth*v }
	p.Transform(ui.Rotation(side*flap), rootX, rootY, func() {
		var wing ui.Path
		wing.MoveTo(rootX, pointY(.2)).
			CubeTo(pointX(.06), pointY(-.3), pointX(.6), pointY(-.88), pointX(1), pointY(-1)).
			QuadTo(pointX(1.08), pointY(-.58), pointX(.73), pointY(-.25)).
			QuadTo(pointX(.96), pointY(-.08), pointX(.56), pointY(.03)).
			QuadTo(pointX(.69), pointY(.2), pointX(.36), pointY(.2)).
			QuadTo(pointX(.35), pointY(.41), pointX(.13), pointY(.32)).
			QuadTo(pointX(.04), pointY(.29), rootX, pointY(.2)).Close()
		p.FillPathGradient(&wing, ui.LinearGradient{From: ui.Hex("#FFFBEF"), To: ui.Hex("#CBD8BC"), Angle: 140})
		p.StrokePath(&wing, max(.5, .8*scale), t.Accent.Alpha(.24*spread))
		var feathers ui.Path
		feathers.MoveTo(pointX(.1), pointY(.13)).QuadTo(pointX(.42), pointY(-.12), pointX(.84), pointY(-.72))
		feathers.MoveTo(pointX(.12), pointY(.18)).QuadTo(pointX(.39), pointY(.08), pointX(.63), pointY(-.16))
		feathers.MoveTo(pointX(.14), pointY(.23)).QuadTo(pointX(.28), pointY(.24), pointX(.39), pointY(.13))
		p.StrokePath(&feathers, max(.5, .85*scale), ui.Hex("#91AA80").Alpha(.48*spread))
	})
}

func paintFlightTrail(p *ui.Painter, t *ui.Theme, seconds float64) {
	// Sample the same path as the icon so the streak follows its actual
	// takeoff instead of floating at fixed coordinates.
	fadeIn := smoothScene((seconds - .25) / .28)
	fadeOut := 1 - smoothScene((seconds-(launchExit-.1))/.7)
	if fadeIn <= .001 || fadeOut <= .001 {
		return
	}
	current := launchMotionAt(seconds, true)
	for index, lag := range []float64{.12, .26, .44} {
		past := launchMotionAt(max(0, seconds-lag), true)
		opacity := []float32{.3, .18, .1}[index] * fadeIn * fadeOut
		if opacity <= .001 || past.x == current.x && past.y == current.y {
			continue
		}
		var line ui.Path
		midX := past.x + (current.x-past.x)*.5
		midY := past.y + (current.y-past.y)*.5 - float32(index+1)*3
		line.MoveTo(past.x, past.y).QuadTo(midX, midY, current.x, current.y)
		p.StrokePath(&line, 2.2-float32(index)*.45, t.Accent.Alpha(opacity))
	}
}
