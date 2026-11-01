package brand

import (
	"math"

	ui "github.com/eryajf/Peck/internal/vui"
)

type sceneColors struct {
	bodyLight, bodyMid, bodyDark, bodyEdge         ui.Color
	headLight, headMid, headDark, headEdge         ui.Color
	tailLight, tailDark                            ui.Color
	wingLight, wingDark, farWingLight, farWingDark ui.Color
	feathers, legShade, legLight, legEdge          ui.Color
	bellyLight, bellyDark                          ui.Color
	beakLight, beakDark, beakBottom, beakSeam      ui.Color
	eye, eyeShine                                  ui.Color
}

func sceneColorsFor(t *ui.Theme) sceneColors {
	colors := sceneColors{
		bodyLight: ui.Hex("#66755B"), bodyMid: ui.Hex("#3A503C"), bodyDark: ui.Hex("#253A2B"), bodyEdge: ui.Hex("#1E3225").Alpha(.55),
		headLight: ui.Hex("#6B7C5D"), headMid: ui.Hex("#354C37"), headDark: ui.Hex("#1E3124"), headEdge: ui.Hex("#1C3023").Alpha(.5),
		tailLight: ui.Hex("#50614A"), tailDark: ui.Hex("#203026"),
		wingLight: ui.Hex("#7B8968"), wingDark: ui.Hex("#3A4D36"), farWingLight: ui.Hex("#4B6045"), farWingDark: ui.Hex("#24392A"),
		feathers: ui.Hex("#C3CBAA"), legShade: ui.Hex("#786A50"), legLight: ui.Hex("#A28C64"), legEdge: ui.Hex("#62563F"),
		bellyLight: ui.Hex("#FFF9E8"), bellyDark: ui.Hex("#C9CCAD"),
		beakLight: ui.Hex("#E4C087"), beakDark: ui.Hex("#B58A49"), beakBottom: ui.Hex("#AF874A"), beakSeam: ui.Hex("#725335").Alpha(.5),
		eye: ui.Hex("#18251B"), eyeShine: ui.Hex("#FFFFFF").Alpha(.95),
	}
	if t != nil && t.Dark {
		// Lift the bird's midtones against dark cards while keeping its green
		// identity. The pale belly and crown remain unchanged anchors.
		colors.bodyLight, colors.bodyMid, colors.bodyDark = ui.Hex("#819A76"), ui.Hex("#4E6D54"), ui.Hex("#263E34")
		colors.headLight, colors.headMid, colors.headDark = ui.Hex("#89A27D"), ui.Hex("#55725A"), ui.Hex("#2B463A")
		colors.bodyEdge, colors.headEdge = ui.Hex("#B6C9A9").Alpha(.45), ui.Hex("#B6C9A9").Alpha(.42)
		colors.tailLight, colors.tailDark = ui.Hex("#5D795F"), ui.Hex("#2C4639")
		colors.wingLight, colors.wingDark = ui.Hex("#6F896A"), ui.Hex("#344F40")
		colors.farWingLight, colors.farWingDark = ui.Hex("#58705C"), ui.Hex("#2A4035")
		colors.feathers = ui.Hex("#D1DDBE")
		colors.legShade, colors.legLight, colors.legEdge = ui.Hex("#635B46"), ui.Hex("#B2A66C"), ui.Hex("#514B3A")
		colors.bellyLight, colors.bellyDark = ui.Hex("#FFFFF0"), ui.Hex("#D2DDC7")
		colors.beakLight, colors.beakDark = ui.Hex("#F0CB83"), ui.Hex("#B98A47")
		colors.beakBottom, colors.beakSeam = ui.Hex("#C0914D"), ui.Hex("#765633").Alpha(.6)
		colors.eye = ui.Hex("#13251D")
	}
	return colors
}

// The bird turns by moving its features around a rounded body. Its silhouette
// keeps its width at every angle; no negative/near-zero scale flips the artwork.
func paintSceneBird(p *ui.Painter, t *ui.Theme, pose scenePose) {
	colors := sceneColorsFor(t)
	p.Transform(ui.Scaling(.8, .8).Then(ui.Translation(pose.x, pose.y)), 0, 0, func() {
		p.Transform(ui.Rotation(pose.bodyAngle), 76, 145, func() {
			yaw := max(-1, min(pose.facing, 1))
			front := 1 - float32(math.Abs(float64(yaw)))
			cx := 76 + 2*yaw
			leftNear, rightNear := (1+yaw)/2, (1-yaw)/2
			paintSteppingLeg(p, cx-10, pose.step, pose, colors, colors.legShade.Mix(colors.legLight, leftNear))
			paintSteppingLeg(p, cx+10, pose.step+.5, pose, colors, colors.legShade.Mix(colors.legLight, rightNear))

			p.Transform(ui.Rotation(pose.tailSwing), cx-15*yaw, 136+pose.bob, func() {
				var tail ui.Path
				tail.MoveTo(cx-12*yaw, 113+pose.bob).
					QuadTo(cx-30*yaw, 140+pose.bob, cx-43*yaw, 166+pose.bob).
					QuadTo(cx-9*yaw, 156+pose.bob, cx+7*yaw, 136+pose.bob).Close()
				p.FillPathGradient(&tail, ui.LinearGradient{From: colors.tailLight, To: colors.tailDark, Angle: 150})
			})

			p.Transform(ui.Rotation(-pose.wingSwing*.28), cx-(24+front*3), 103+pose.bob, func() {
				paintWing(p, cx-(24+front*3), 103+pose.bob, 11+3*front, 31, true, 1-pose.wingLift, colors)
			})
			p.Transform(ui.Rotation(pose.wingSwing*.28), cx+(24+front*3), 103+pose.bob, func() {
				paintWing(p, cx+(24+front*3), 103+pose.bob, 11+3*front, 31, true, 1, colors)
			})
			body := sceneEllipse(cx, 104+pose.bob, 30+4*front, 43)
			p.FillPathGradient(&body, ui.LinearGradient{Angle: 125, Stops: []ui.GradientStop{
				{Color: colors.bodyLight, At: 0},
				{Color: colors.bodyMid, At: .44},
				{Color: colors.bodyDark, At: 1},
			}})
			p.StrokePath(&body, 1, colors.bodyEdge)
			belly := sceneEllipse(cx+11*yaw, 109+pose.bob, 17+6*front, 33)
			p.FillPathGradient(&belly, ui.LinearGradient{From: colors.bellyLight, To: colors.bellyDark, Angle: 135})
			var chest ui.Path
			chest.MoveTo(cx-7+8*yaw, 79+pose.bob).QuadTo(cx-16+8*yaw, 89+pose.bob, cx-13+8*yaw, 103+pose.bob)
			p.StrokePath(&chest, 3, ui.Hex("#FFFFFF").Alpha(.18))
			// Near wings cross-fade over the body while the rear wings stay behind
			// it, so passing through the front view never swaps whole layers.
			nearX, nearY := cx-(19+front*9), 105+pose.bob
			nearWidth := 12 + 8*max(0, yaw)
			// Raise the wing from its attached shoulder, keeping the root in
			// place. The tip passes above the shoulder before two farewell
			// beats; translating/rotating its center only shakes a folded wing.
			shoulderX, shoulderY := nearX-nearWidth*.45, nearY-31*.75
			waveAngle := 125*pose.wingLift + 22*pose.wingWave
			p.Transform(ui.Rotation(waveAngle), shoulderX, shoulderY, func() {
				p.Transform(ui.Rotation(pose.wingSwing), nearX, nearY, func() {
					paintWing(p, nearX, nearY, nearWidth, 31, false, leftNear, colors)
				})
			})
			p.Transform(ui.Rotation(-pose.wingSwing), cx+(19+front*9), 105+pose.bob, func() {
				paintWing(p, cx+(19+front*9), 105+pose.bob, 12+8*max(0, -yaw), 31, false, rightNear, colors)
			})

			p.Transform(ui.Rotation(pose.angle), 70, 75+pose.bob, func() {
				// The head, face, crown and bill share one neck transform.
				// During observation, a small clockwise tilt lowers the right
				// profile toward the insect while keeping the neck connected.
				p.Transform(ui.Translation(pose.headOffsetX, pose.headOffsetY), 0, 0, func() {
					p.Transform(ui.Rotation(pose.headRoll), 70, 75+pose.bob, func() {
						paintRoundHead(p, colors, pose.headFacing, pose.bob, pose.happy, pose.blink, pose.crownColor)
					})
				})
			})
		})
	})
}

func paintSteppingLeg(p *ui.Painter, hip float32, phase float64, pose scenePose, colors sceneColors, color ui.Color) {
	phase = math.Mod(phase, 1)
	shift, lift := float32(0), float32(0)
	if phase < .64 {
		// During stance the foot travels backwards relative to the body,
		// keeping its position close to the ground while weight moves over it.
		shift = 9.2 - 18.4*float32(phase/.64)
	} else {
		u := (phase - .64) / .36
		shift = -9.2 + 18.4*smoothScene(u)
		lift = 12 * float32(math.Sin(u*math.Pi))
	}
	shift *= pose.walk * pose.facing
	lift *= pose.walk
	footX, footY := hip+shift, 160-lift
	var leg ui.Path
	leg.MoveTo(hip, 142+pose.bob).LineTo(hip+shift*.42, 151-lift*.3+pose.bob*.4).LineTo(footX, footY)
	p.StrokePath(&leg, 3.8, colors.legEdge)
	p.StrokePath(&leg, 2.3, color)
	leg.Reset().MoveTo(footX-5, footY+1).QuadTo(footX, footY-1, footX+7, footY+1)
	leg.MoveTo(footX+1, footY).LineTo(footX+3, footY+3)
	p.StrokePath(&leg, 2.3, color)
}

func paintWing(p *ui.Painter, x, y, width, height float32, far bool, opacity float32, colors sceneColors) {
	if opacity <= .01 {
		return
	}
	var wing ui.Path
	wing.MoveTo(x-width*.45, y-height*.75).
		CubeTo(x-width*1.05, y-height*.15, x-width*.85, y+height*.7, x+width*.05, y+height).
		CubeTo(x+width*.85, y+height*.25, x+width*.8, y-height*.45, x-width*.45, y-height*.75).Close()
	light, dark := colors.wingLight, colors.wingDark
	if far {
		light, dark = colors.farWingLight, colors.farWingDark
	}
	p.FillPathGradient(&wing, ui.LinearGradient{From: light.Alpha(opacity), To: dark.Alpha(opacity), Angle: 140})
	if !far {
		var feathers ui.Path
		feathers.MoveTo(x-width*.35, y+2).QuadTo(x, y+7, x+width*.3, y+5)
		feathers.MoveTo(x-width*.3, y+12).QuadTo(x, y+17, x+width*.18, y+15)
		p.StrokePath(&feathers, 2.1, colors.feathers.Alpha(.7*opacity))
	}
}

func paintRoundHead(p *ui.Painter, colors sceneColors, facing, bob float32, happy bool, blink, crownColor float32) {
	yaw := max(-1, min(facing, 1))
	front := 1 - float32(math.Abs(float64(yaw)))
	cx, cy := 76+4*yaw, 48+bob
	head := sceneEllipse(cx, cy, 29+front*2, 30)
	p.FillPathGradient(&head, ui.LinearGradient{Angle: 125, Stops: []ui.GradientStop{
		{Color: colors.headLight, At: 0},
		{Color: colors.headMid, At: .45},
		{Color: colors.headDark, At: 1},
	}})
	p.StrokePath(&head, 1, colors.headEdge)
	cheek := sceneEllipse(cx+10*yaw, 56+bob, 19+front*6, 18)
	p.FillPathGradient(&cheek, ui.LinearGradient{From: colors.bellyLight, To: colors.bellyDark, Angle: 130})
	paintBirdCrown(p, cx, yaw, bob, crownColor)

	paintBirdEye(p, colors, cx-11+3*yaw, 43+bob, min(1, max(0, 1-yaw)), happy, blink)
	paintBirdEye(p, colors, cx+11+3*yaw, 43+bob, min(1, max(0, 1+yaw)), happy, blink)
	// Foreshortening comes from the head's yaw. The bill stays part of the
	// same face instead of counter-rotating separately from the eyes/crown.
	paintBirdBeak(p, colors, cx, yaw, front, bob)
}

func paintBirdBeak(p *ui.Painter, colors sceneColors, cx, yaw, front, bob float32) {
	rootX, tipX := cx+21*yaw, cx+60*yaw
	tipY := 53 + 13*front + bob
	var beak ui.Path
	beak.MoveTo(rootX-7*front, 47+2*front+bob).LineTo(tipX, tipY).
		LineTo(rootX, 54-3*front+bob).Close()
	p.FillPathGradient(&beak, ui.LinearGradient{From: colors.beakLight, To: colors.beakDark, Angle: 135})
	beak.Reset().MoveTo(rootX, 54-3*front+bob).LineTo(tipX, tipY).
		LineTo(cx+20*yaw+7*front, 62-13*front+bob).Close()
	p.FillPathGradient(&beak, ui.LinearGradient{From: colors.beakBottom, To: ui.Hex("#826139"), Angle: 180})
	var seam ui.Path
	seam.MoveTo(rootX, 54-3*front+bob).LineTo(tipX, tipY)
	p.StrokePath(&seam, .8, colors.beakSeam)
}

func paintBirdCrown(p *ui.Painter, cx, yaw, bob, colorProgress float32) {
	var crown ui.Path
	crown.MoveTo(cx-23, 29+bob).LineTo(cx-27-3*yaw, 10+bob).
		LineTo(cx-11-3*yaw, 17+bob).LineTo(cx-4, 2+bob).
		LineTo(cx+7, 17+bob).LineTo(cx+23+3*yaw, 10+bob).
		LineTo(cx+17, 29+bob).QuadTo(cx-1, 22+bob, cx-23, 29+bob).Close()
	p.FillPathGradient(&crown, ui.LinearGradient{From: ui.Hex("#E07655"), To: ui.Hex("#A6392E"), Angle: 150})
	colorProgress = max(0, min(colorProgress, 1))
	if colorProgress > 0 {
		// Bloom once after successful processing, then keep the colors still
		// through the result and launch scenes. Other states retain red.
		p.FillPathGradient(&crown, ui.LinearGradient{Angle: 100, Stops: []ui.GradientStop{
			{Color: ui.Hex("#E9846C").Alpha(colorProgress), At: 0},
			{Color: ui.Hex("#E6B75F").Alpha(colorProgress), At: .25},
			{Color: ui.Hex("#88B989").Alpha(colorProgress), At: .5},
			{Color: ui.Hex("#69ADC4").Alpha(colorProgress), At: .75},
			{Color: ui.Hex("#A18DC7").Alpha(colorProgress), At: 1},
		}})
	}
	var crestShine ui.Path
	crestShine.MoveTo(cx-17, 23+bob).LineTo(cx-19, 17+bob)
	crestShine.MoveTo(cx-4, 19+bob).LineTo(cx-4, 11+bob)
	p.StrokePath(&crestShine, 2, ui.Hex("#F3A275").Mix(ui.Hex("#FFF3D9"), colorProgress).Alpha(.6))
}

func paintBirdEye(p *ui.Painter, colors sceneColors, x, y, visibility float32, happy bool, blink float32) {
	if visibility <= .01 {
		return
	}
	if happy {
		var smile ui.Path
		smile.MoveTo(x-3.8, y+1).QuadTo(x, y-3.5, x+3.8, y+1)
		p.StrokePath(&smile, 2.1, colors.eye.Alpha(visibility))
		return
	}
	blink = max(0, min(blink, 1))
	if blink > .01 {
		var lid ui.Path
		lid.MoveTo(x-3.6, y).QuadTo(x, y+2.2*blink, x+3.6, y)
		p.StrokePath(&lid, 1.8, colors.eye.Alpha(visibility*blink))
	}
	if blink > .95 {
		return
	}
	eye := sceneEllipse(x, y, 3.25, 3.7*(1-blink))
	p.FillPath(&eye, colors.eye.Alpha(visibility))
	eye = sceneEllipse(x+.7, y-1.1*(1-blink), .9, .9*(1-blink))
	p.FillPath(&eye, colors.eyeShine.Alpha(visibility*(1-blink)))
}

func sceneEllipse(cx, cy, rx, ry float32) ui.Path {
	const k = .55228475
	var path ui.Path
	path.MoveTo(cx+rx, cy).
		CubeTo(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry).
		CubeTo(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy).
		CubeTo(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry).
		CubeTo(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy).Close()
	return path
}
