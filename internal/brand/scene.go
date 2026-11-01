package brand

import (
	"math"
	"time"

	ui "github.com/eryajf/Peck/internal/vui"
)

// SceneMode describes artwork only. The caller chooses Success after the real
// operation succeeds; no scene transition can authorize or finish a task.
type SceneMode uint8

const (
	SceneIdle SceneMode = iota
	SceneInspect
	SceneReady
	SceneProcessing
	SceneSuccess
	SceneStopped
	SceneLaunch
)

const (
	InsectCount     = 4
	SuccessDuration = 2600 * time.Millisecond
	LaunchDuration  = 3250 * time.Millisecond
)

// SceneOptions keeps the randomly chosen insect stable for one application.
// Elapsed belongs to the current mode; Animate=false paints a still scene.
type SceneOptions struct {
	Mode    SceneMode
	Insect  int
	Elapsed time.Duration
	Animate bool
	AppIcon ui.ImageSource
}

var sceneInsects = [InsectCount]*ui.SVG{
	insectVector(`<path d="m6 22-1 3m6-2v3m6-3 1 3m6-6 2 2" stroke="#4e6844" stroke-width="1.5" stroke-linecap="round"/><circle cx="7" cy="19" r="4" fill="#8aa66c"/><circle cx="13" cy="18" r="4.7" fill="#a4bb7b"/><circle cx="19" cy="16" r="4.9" fill="#b4c88c"/><circle cx="24" cy="12" r="5.5" fill="#a4bb7b"/><path d="m22 7-1-3m6 4 2-3" stroke="#4e6844" stroke-width="1.5" stroke-linecap="round"/><circle cx="26" cy="11" r="1" fill="#273c2e"/><path d="m26 15 2-1" stroke="#4e6844" stroke-width="1.2" stroke-linecap="round"/>`),
	insectVector(`<path d="m10 12-5-3m5 9-6 1m8 4-5 4m15-15 5-3m-5 9 6 1m-8 4 5 4m-13-20-2-3m8 3 2-3" stroke="#5b4436" stroke-width="1.5" stroke-linecap="round"/><ellipse cx="16" cy="18" rx="8.5" ry="10" fill="#bf7657"/><path d="M16 9v18" stroke="#79523c" stroke-width="1.4"/><ellipse cx="16" cy="8" rx="5" ry="4" fill="#4e4537"/><circle cx="13" cy="7" r=".8" fill="#f4e9d4"/><circle cx="19" cy="7" r=".8" fill="#f4e9d4"/><circle cx="11.5" cy="17" r="1.7" fill="#875038"/><circle cx="20.5" cy="21" r="1.8" fill="#875038"/>`),
	insectVector(`<path d="M5 24c7 3 5-10 11-7 5 2 1-10 8-10" fill="none" stroke="#aa7d48" stroke-width="9" stroke-linecap="round"/><path d="M5 23c7 3 5-10 11-7 5 2 1-10 8-10" fill="none" stroke="#d3b477" stroke-width="6" stroke-linecap="round"/><path d="m8 22 3 2m2-7 2 4m4-7 3 1" stroke="#aa7d48" stroke-width="1.2" stroke-linecap="round"/><circle cx="25" cy="6" r="4" fill="#d3b477"/><circle cx="26" cy="5" r=".9" fill="#4b4932"/><path d="m24 2-1-2m5 3 2-2" stroke="#8b754e" stroke-width="1.2" stroke-linecap="round"/>`),
	insectVector(`<path d="M15 15C3-1-2 15 13 20 1 26 11 32 16 21c6 11 15 5 3-1C34 15 29-1 17 15" fill="#88b7a0"/><path d="M15 16C6 7 4 15 13 18m5-2c8-9 11-1 2 2" fill="none" stroke="#bfd5aa" stroke-width="2.5" stroke-linecap="round"/><ellipse cx="16" cy="19" rx="2.3" ry="8" fill="#4b6755"/><circle cx="16" cy="10" r="3" fill="#4b6755"/><path d="m15 8-4-4m6 4 4-4" stroke="#4b6755" stroke-width="1.3" stroke-linecap="round"/>`),
}

func insectVector(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">` + paths + `</svg>`))
}

type scenePose struct {
	x, y, facing, headFacing, bob, angle float32
	bodyAngle, tailSwing, wingSwing      float32
	wingLift, wingWave                   float32
	headRoll, crownColor                 float32
	headOffsetX, headOffsetY             float32
	blink                                float32
	step                                 float64
	walk                                 float32
	happy                                bool
}

// Scene paints a 360×170 native vector composition, fitted into r. It never
// schedules a frame; the app owns visibility, reduced motion and its 30 fps cap.
func Scene(p *ui.Painter, r ui.Rect, t *ui.Theme, options SceneOptions) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if t == nil {
		t = ui.LightTheme()
	}
	scale := min(r.W/360, r.H/170)
	x, y := r.X+(r.W-360*scale)/2, r.Y+(r.H-170*scale)/2
	p.Clip(r, 0, func() {
		p.Transform(ui.Scaling(scale, scale).Then(ui.Translation(x, y)), 0, 0, func() {
			paintScene(p, t, options)
		})
	})
}

func paintScene(p *ui.Painter, t *ui.Theme, options SceneOptions) {
	seconds := max(0, options.Elapsed.Seconds())
	if !options.Animate {
		seconds = 0
		if options.Mode == SceneSuccess {
			seconds = SuccessDuration.Seconds()
		} else if options.Mode == SceneLaunch {
			seconds = LaunchDuration.Seconds()
		}
	}
	pose := scenePose{x: 145, y: 31, facing: 1, headFacing: 1}
	var flight launchMotion
	open, insectSize := float32(0), float32(27)
	bugX, bugY := float32(268), float32(88)
	if options.AppIcon != nil {
		// Keep the selected app readable while the bug peeks from its upper
		// edge, close enough for the beak to reach.
		bugX, bugY, insectSize = 277, 73, 23
	}
	bugAngle, impact := float32(0), float32(0)
	switch options.Mode {
	case SceneIdle:
		pose.x = 79
		if options.Animate {
			pose = walkingPose(seconds)
		}
	case SceneInspect:
		pose.x, pose.angle = 133, -5
		if options.Animate {
			pose.angle += 3 * float32(math.Sin(seconds*2.4))
		}
	case SceneReady:
		pose.angle = 6
	case SceneProcessing:
		pose.x = 151
		if options.Animate {
			peck := PeckAngle(options.Elapsed) / 19
			pose.x += 3 * peck
			pose.angle = 17 * peck
			impact = max(0, (peck-.72)/.28)
		} else {
			pose.angle = 8
		}
	case SceneSuccess:
		seconds = min(seconds, SuccessDuration.Seconds())
		open = smoothScene(seconds / .52)
		grab := smoothScene((seconds - .52) / .53)
		eat := smoothScene((seconds - 1.05) / .65)
		pose.x, pose.angle = 151-18*grab, 14-12*grab
		if seconds >= .52 {
			beakX, beakY := sceneBeak(pose, 140-34*eat)
			catch := smoothScene((seconds - .52) / .18)
			bugX += (beakX - bugX) * catch
			bugY += (beakY - bugY) * catch
			insectSize *= 1 - eat
			bugAngle = float32(math.Sin(seconds*18)) * 12 * (1 - eat)
		}
		if seconds > 1.05 && seconds < 1.7 {
			pose.angle -= 7*eat + float32(math.Sin(seconds*28))*3*float32(math.Sin(float64(eat)*math.Pi))
			bugX, bugY = sceneBeak(pose, 140-34*eat)
		}
		if seconds >= 1.7 {
			celebrate := float32((seconds - 1.7) / .9)
			hop := float32(math.Abs(math.Sin(float64(celebrate)*math.Pi*2))) * 8 * (1 - celebrate)
			pose.y -= max(0, hop)
			pose.angle = -5
			pose.happy = true
			pose.crownColor = smoothScene((seconds - 1.7) / .7)
			insectSize = 0
		}
	case SceneStopped:
		pose.x, pose.angle = 127, -5
	case SceneLaunch:
		flight = launchMotionAt(seconds, options.Animate)
		pose = launchBirdPose(flight)
		open, insectSize = 1, 0
	}
	if options.Animate && options.Mode != SceneSuccess && options.Mode != SceneStopped && options.Mode != SceneLaunch {
		bugAngle = float32(math.Sin(seconds*2.6)) * 5
	}

	p.Fill(ui.Rect{X: 48, Y: 151, W: 282, H: 11}, t.Accent.Alpha(.055), 6)
	doorIcon := options.AppIcon
	if options.Mode == SceneLaunch {
		// Transfer the one icon to the foreground instead of leaving a copy
		// behind in the door while another copy moves out.
		doorIcon = nil
	}
	paintDoor(p, t, open, doorIcon)
	// Viva composites bitmaps after vectors within one layer. Keep the bird
	// and insect above the door icon, and the flying icon above both.
	p.Layer(1, func() {
		p.Fill(ui.Rect{X: pose.x + 34, Y: 155, W: 60, H: 8}, t.Text.Alpha(.065), 4)
		p.Fill(ui.Rect{X: pose.x + 46, Y: 157, W: 35, H: 4}, t.Text.Alpha(.065), 2)
		paintSceneBird(p, t, pose)
		if insectSize > .4 {
			insect := options.Insect % InsectCount
			if insect < 0 {
				insect += InsectCount
			}
			if t.Dark {
				// Give the small insect a quiet edge light so green and
				// mint variants stay legible against dark door interiors.
				halo := sceneEllipse(bugX, bugY, insectSize*.62, insectSize*.52)
				p.FillPath(&halo, t.Text.Alpha(.035))
				p.StrokePath(&halo, .7, t.Accent.Alpha(.11))
			}
			p.Transform(ui.Rotation(bugAngle), bugX, bugY, func() {
				p.Image(sceneInsects[insect], ui.Rect{X: bugX - insectSize/2, Y: bugY - insectSize/2, W: insectSize, H: insectSize}, ui.Contain)
			})
		}
		if impact > 0 {
			color := ui.Hex("#B18C56").Alpha(.7 * impact)
			var marks ui.Path
			marks.MoveTo(271, 68).LineTo(275, 63).MoveTo(280, 75).LineTo(286, 73).MoveTo(282, 84).LineTo(288, 85)
			p.StrokePath(&marks, 1.8, color)
		}
		if pose.happy {
			paintHappyMarks(p, t, pose.x+73, pose.y-1)
		}
	})
	if options.Mode == SceneLaunch {
		p.Layer(2, func() {
			paintLaunchingIcon(p, t, options.AppIcon, flight)
		})
	}
}

func walkingPose(seconds float64) scenePose {
	const outward, homeward, doorLook, awayLook = 2.8, 3.05, 2.25, .65
	const turn, settle = .72, .18
	const outwardCycle = outward + doorLook + turn + settle
	const fullCycle = outwardCycle + homeward + awayLook + turn + settle
	phase := math.Mod(seconds, fullCycle)
	direction := float32(1)
	// Keep the standing position clear of the door. Stride follows the
	// actual distance to this endpoint.
	start, end := float32(53), float32(134)
	travel, observe := outward, doorLook
	if phase >= outwardCycle {
		phase -= outwardCycle
		travel, observe = homeward, awayLook
		direction, start, end = -1, end, start
	}
	pose := scenePose{x: start, y: 31, facing: direction, headFacing: direction}
	if phase < travel {
		u := phase / travel
		distance := walkingProgress(u) * math.Abs(float64(end-start))
		pose.x += direction * float32(distance)
		pose.walk = min(smoothScene(u/.14), smoothScene((1-u)/.14))
		pose.step = distance / 22.5
		stride := pose.step * math.Pi * 2
		weight := float32(math.Sin(stride))
		pose.bob = -1.6 * pose.walk * float32(1-math.Cos(stride*2))
		pose.bodyAngle = direction * pose.walk * (1.4 + 1.1*weight)
		pose.tailSwing = 4.5 * direction * pose.walk * float32(math.Sin(stride-.8))
		pose.wingSwing = 4 * direction * pose.walk * float32(math.Sin(stride+.4))
		// A small counter-motion keeps the gaze steady while the body walks;
		// the tail and folded wings follow at different phases.
		pose.angle = -direction * pose.walk * (2 + 1.7*float32(math.Sin(stride-.55)))
		pose.blink = blinkPulse(phase, travel*.43)
		return pose
	}
	pose.x = end
	if phase < travel+observe {
		pause := phase - travel
		attend := smoothScene(pause/.3) * (1 - smoothScene((pause-observe+.3)/.3))
		if direction > 0 {
			// Keep the right-side profile aimed at the door. Looking at the
			// bug is a small downward cock of that profile, not a turn toward
			// the viewer and not a large neck bend.
			secondLook := smoothScene((pause - .48) / .24)
			sideTilt := smoothScene((pause-.15)/.28) * (1 - smoothScene((pause-observe+.42)/.38))
			pose.angle = 0
			pose.headFacing = 1
			pose.headRoll = (10 + 2*secondLook) * sideTilt
			// The tiny offset lets the beak meet the insect without making
			// the whole head slide toward the doorway.
			pose.headOffsetX, pose.headOffsetY = 1.5*sideTilt, 2.5*sideTilt
			pose.bodyAngle = 0
			pose.tailSwing = -3 * attend
			pose.wingSwing = -2 * attend
			pose.blink = blinkPulse(pause, .92)
		} else {
			pose.angle = 6 * attend
			pose.headFacing = -1 + .12*attend
			pose.headOffsetX = 3.5 * attend * pose.headFacing
			pose.headOffsetY = 2 * attend
		}
		return pose
	}
	if phase < travel+observe+turn {
		u := (phase - travel - observe) / turn
		pose.facing = direction * float32(math.Cos(float64(smoothScene(u))*math.Pi))
		pose.headFacing = direction * float32(math.Cos(float64(smoothScene(u/.82))*math.Pi))
		pivot := float32(math.Sin(u * math.Pi))
		pose.bob = -1.2 * pivot
		pose.angle = -3 * direction * pivot
		pose.tailSwing = 5 * direction * pivot
		return pose
	}
	pose.facing, pose.headFacing = -direction, -direction
	return pose
}

// Short acceleration and deceleration leave an even walking pace between
// endpoints. Unlike easing the entire journey, this does not look like a bird
// sliding slowly out of and back into each turn.
func walkingProgress(u float64) float64 {
	const ramp = .16
	if u < ramp {
		return u * u / (2 * ramp * (1 - ramp))
	}
	if u > 1-ramp {
		return 1 - (1-u)*(1-u)/(2*ramp*(1-ramp))
	}
	return (u - ramp/2) / (1 - ramp)
}

func blinkPulse(seconds, center float64) float32 {
	return smoothScene(1 - math.Abs(seconds-center)/.11)
}

func sceneBeak(pose scenePose, mouthX float32) (float32, float32) {
	angle := float64(pose.angle) * math.Pi / 180
	dx, dy := mouthX-70, float32(-22)
	x := 70 + dx*float32(math.Cos(angle)) - dy*float32(math.Sin(angle))
	y := 75 + dx*float32(math.Sin(angle)) + dy*float32(math.Cos(angle)) + pose.bob
	return pose.x + .8*x, pose.y + .8*y
}

func paintDoor(p *ui.Painter, t *ui.Theme, open float32, icon ui.ImageSource) {
	frame, wood, grain := ui.Hex("#B69A73"), ui.Hex("#D2B78B"), ui.Hex("#A18760")
	inside := ui.Hex("#415744")
	if t.Dark {
		frame, wood, grain = ui.Hex("#79664B"), ui.Hex("#A78C61"), ui.Hex("#725B3F")
		inside = ui.Hex("#1C3329")
	}
	p.Shadow(ui.Rect{X: 241, Y: 22, W: 96, H: 134}, 16, 0, 3, 7, 0, t.Text.Alpha(.07))
	p.FillGradient(ui.Rect{X: 241, Y: 22, W: 96, H: 134}, ui.LinearGradient{From: frame.Mix(ui.Hex("#FFF5DE"), .18), To: frame, Angle: 135}, 16)
	p.FillGradient(ui.Rect{X: 248, Y: 28, W: 82, H: 124}, ui.LinearGradient{From: inside, To: inside.Mix(ui.Hex("#16261C"), .35), Angle: 180}, 14)
	// The application stays inside the doorway. The leaf has a real aperture,
	// so the selected icon is readable before processing and revealed in full
	// when the door opens. The bug overlaps its upper edge to connect the
	// obstruction with this app, then leaves the intact icon when caught.
	if icon != nil {
		p.Fill(ui.Rect{X: 261, Y: 61, W: 56, H: 56}, ui.Hex("#FFF9E8").Alpha(.1), 13)
	}
	width := 80 * (1 - .84*open)
	left, right, top, bottom := 329-width, float32(329), float32(29), float32(152)
	lift := open * 9
	windowLeft, windowRight := left+width*.12, left+width*.88
	windowTop, windowBottom := float32(57)+lift*.6, float32(121)-lift*.2
	radius := min(float32(13), (windowRight-windowLeft)*AppIconCornerRatio)
	var door ui.Path
	door.MoveTo(left, top+lift).QuadTo(left+width*.35, top+lift-3, right, top).LineTo(right, bottom).LineTo(left, bottom-lift).Close()
	// Reverse the inner contour so non-zero winding leaves the window open.
	appendDoorWindow(&door, windowLeft, windowTop, windowRight, windowBottom, radius)
	p.FillPathGradient(&door, ui.LinearGradient{From: wood.Mix(ui.Hex("#FFF6DF"), .12), To: wood, Angle: 125})
	p.StrokePath(&door, 1.2, grain)
	for _, fraction := range []float32{.34, .68} {
		x := left + width*fraction
		var plank ui.Path
		plank.MoveTo(x, top+lift*(1-fraction)+3).LineTo(x, windowTop-3)
		plank.MoveTo(x, windowBottom+3).LineTo(x, bottom-lift*(1-fraction)-2)
		p.StrokePath(&plank, 1, grain.Alpha(.7))
	}
	var lines ui.Path
	lines.MoveTo(left+width*.76, 135).QuadTo(left+width*.85, 131, left+width*.8, 127)
	lines.MoveTo(left+width*.18, 45).QuadTo(left+width*.26, 41, left+width*.21, 36)
	p.StrokePath(&lines, 1, grain.Alpha(.5))
	var knob ui.Path
	knob.Circle(left+width*.19, 135-lift*.45, 3)
	p.FillPath(&knob, ui.Hex("#7B6545"))
	knob.Reset().Circle(left+width*.19-.7, 134-lift*.45, 1)
	p.FillPath(&knob, ui.Hex("#E8CEA1").Alpha(.65))
	p.Fill(ui.Rect{X: 326, Y: 47, W: 7, H: 7}, grain, 2)
	p.Fill(ui.Rect{X: 326, Y: 129, W: 7, H: 7}, grain, 2)
	p.Fill(ui.Rect{X: 237, Y: 151, W: 104, H: 6}, frame, 3)
	// Keep the application artwork centered in the door's aperture.
	if icon != nil {
		p.Image(icon, ui.Rect{X: 265, Y: 65, W: 48, H: 48}, ui.Contain)
	}
	var sprout ui.Path
	sprout.MoveTo(345, 153).QuadTo(339, 143, 344, 138).QuadTo(350, 141, 345, 153)
	sprout.MoveTo(345, 150).QuadTo(346, 138, 354, 137).QuadTo(354, 146, 345, 150)
	p.FillPath(&sprout, t.Accent.Alpha(.45))
}

func appendDoorWindow(path *ui.Path, left, top, right, bottom, radius float32) {
	path.MoveTo(left+radius, top).QuadTo(left, top, left, top+radius).
		LineTo(left, bottom-radius).QuadTo(left, bottom, left+radius, bottom).
		LineTo(right-radius, bottom).QuadTo(right, bottom, right, bottom-radius).
		LineTo(right, top+radius).QuadTo(right, top, right-radius, top).Close()
}

func paintHappyMarks(p *ui.Painter, t *ui.Theme, x, y float32) {
	var marks ui.Path
	marks.MoveTo(x-7, y+1).LineTo(x-10, y-6)
	marks.MoveTo(x+2, y-1).LineTo(x+3, y-9)
	marks.MoveTo(x+11, y+2).LineTo(x+16, y-4)
	p.StrokePath(&marks, 1.8, t.Accent.Alpha(.65))
}

func smoothScene(value float64) float32 {
	u := float32(max(0, min(value, 1)))
	return u * u * (3 - 2*u)
}
