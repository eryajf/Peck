package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/eryajf/Peck/internal/brand"
	"github.com/eryajf/Peck/internal/vui"
	vapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const appPanelCornerRadius = float32(14)

func vivaColor(c vui.Color) ui.Color {
	return ui.Color{Red: c.Red, Green: c.Green, Blue: c.Blue, Alpha: c.A}
}
func vivaTheme(a *application) *brandTheme {
	appearance := a.effectiveAppearance()
	dark := appearance == "midnight" || appearance == "ocean"
	if appearance == "system" {
		dark = vapp.SystemAppearance() == vapp.AppearanceDark
	}
	t := brand.Theme(appearance, dark)
	v := brand.Visuals(appearance, dark)
	return &brandTheme{theme: t, visuals: v}
}

type brandTheme struct {
	theme   *vui.Theme
	visuals brand.VisualStyle
}

func (a *application) paintMain(canvas *ui.Canvas, size ui.Size) {
	a.ensureMessages()
	bt := vivaTheme(a)
	// Paint the surface itself as one rounded shape as well as using the native
	// macOS clip. The canvas/headless path then keeps the same transparent four
	// corners even when no native window compositor is present.
	panel := ui.Rect{Width: size.Width, Height: size.Height}
	canvas.Shadow(panel, appPanelCornerRadius, ui.ShadowStyle{OffsetY: 10, Blur: 30}, vivaColor(bt.visuals.GlassGlow))
	canvas.RoundedBorder(panel, appPanelCornerRadius, 1, vivaColor(bt.visuals.GlassEdge), vivaColor(bt.visuals.BackgroundFrom.Alpha(.90)))
	vui.NewPainter(canvas).FillGradient(vui.Rect{X: 1, Y: 1, W: size.Width - 2, H: size.Height - 2}, vui.LinearGradient{
		From: bt.visuals.BackgroundFrom.Alpha(.90), To: bt.visuals.BackgroundTo.Alpha(.90), Angle: 45,
	}, appPanelCornerRadius-1)

	// The original Peck panel is frameless. Viva keeps the same appearance and
	// uses the declared header move region for dragging the borderless window.
	header := ui.Rect{X: 0, Y: 0, Width: size.Width, Height: 45}
	canvas.MoveRegion(header)
	// Keep the bird's color identity on dark surfaces while reversing only the
	// wordmark. A single all-white lockup loses the crown and face at a glance.
	headerPainter := vui.NewPainter(canvas)
	if bt.theme.Dark {
		a.paintAnimatedMark(canvas, brand.MarkSource(false), vui.Rect{X: 16, Y: 1, W: 38, H: 42})
		headerPainter.Image(brand.WordmarkSource(true), vui.Rect{X: 57, Y: 8, W: 73, H: 28}, vui.Contain)
	} else if a.prefs.Motion && !canvas.ReduceMotion() {
		a.paintAnimatedMark(canvas, brand.MarkSource(false), vui.Rect{X: 16, Y: 1, W: 38, H: 42})
		headerPainter.Image(brand.WordmarkSource(false), vui.Rect{X: 57, Y: 8, W: 73, H: 28}, vui.Contain)
	} else {
		// Use the approved horizontal lockup as one vector so the mark and
		// wordmark share the same optical baseline and proportions.
		headerPainter.Image(brand.LockupSource(false), vui.Rect{X: 16, Y: 1, W: 112, H: 42}, vui.Contain)
	}
	if a.button(canvas, idSettings, ui.Rect{X: size.Width - 126, Y: 18, Width: 54, Height: 28}, a.messages.MainSettings, false, false, bt) {
		a.openSettings()
	}
	if a.button(canvas, idHide, ui.Rect{X: size.Width - 68, Y: 18, Width: 54, Height: 28}, a.messages.MainHide, false, false, bt) {
		a.hide()
	}

	actionsY := size.Height - 76
	actionsHeight := float32(68)
	if a.state == pageConfirm {
		actionsY = size.Height - 50
		actionsHeight = 50
	}
	content := ui.Rect{X: 18, Y: 65, Width: size.Width - 36, Height: actionsY - 65}
	canvas.Clip(content)
	a.paintWorkspace(canvas, content, bt)
	canvas.Unclip()
	a.paintActions(canvas, ui.Rect{X: 18, Y: actionsY, Width: size.Width - 36, Height: actionsHeight}, bt)
}

// paintAnimatedMark adds the short three-tap drumming motion used by the web
// integration artwork to the native brand mark. The pivot matches the source
// SVG's tail anchor, so the wordmark remains fixed while the bird leans toward
// its beak.
func (a *application) paintAnimatedMark(canvas *ui.Canvas, source vui.ImageSource, r vui.Rect) {
	painter := vui.NewPainter(canvas)
	animate := a.prefs.Motion && !canvas.ReduceMotion()
	if !animate {
		painter.Image(source, r, vui.Contain)
		return
	}
	canvas.RequestFrameIn(time.Second / 30)
	elapsed := time.Since(a.motionStarted)
	if elapsed < 0 {
		elapsed = 0
	}
	angle := brand.MarkPeckAngle(elapsed)
	pivotX := r.X + r.W*180/512
	pivotY := r.Y + r.H*430/512
	painter.Transform(vui.Rotation(angle), pivotX, pivotY, func() {
		painter.Image(source, r, vui.Contain)
	})
	// Keep the impact strokes in the page coordinate space, just like the web
	// artwork's `.tap` path sits outside the rotating bird group. They briefly
	// appear in front of the beak at each forward tap and then fade away.
	if impact := brand.MarkPeckImpact(elapsed); impact > 0 {
		stroke := maxFloat32(.65, r.W*12/512)
		impactColor := vui.Hex("#D85D48").Alpha(impact)
		var marks vui.Path
		marks.MoveTo(r.X+r.W*465/512, r.Y+r.H*180/512).
			LineTo(r.X+r.W*483/512, r.Y+r.H*174/512).
			MoveTo(r.X+r.W*465/512, r.Y+r.H*204/512).
			LineTo(r.X+r.W*483/512, r.Y+r.H*211/512)
		painter.StrokePath(&marks, stroke, impactColor)
	}
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func (a *application) paintWorkspace(canvas *ui.Canvas, r ui.Rect, bt *brandTheme) {
	cardHeight := float32(224)
	if a.state == pageConfirm {
		// Use only the space needed by visible status, notice and consent rows.
		// The scene takes the remaining height instead of leaving empty rows.
		cardHeight = r.Height - 10 - 45 - 16
		if a.link != nil {
			cardHeight -= 20
		}
		if a.confirmationTrustOption() {
			cardHeight -= 31
		}
		if lines := a.confirmationNoticeLines(canvas, r.Width-4); len(lines) > 0 {
			cardHeight -= 8 + float32(len(lines))*18
		}
	} else if a.link != nil {
		// Reserve the project-link row without pushing the instructions into
		// the fixed action area.
		cardHeight -= 22
	}
	card := ui.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: cardHeight}
	from, to := vivaColor(bt.visuals.SurfaceFrom.Alpha(.72)), vivaColor(bt.visuals.SurfaceTo.Alpha(.60))
	canvas.FillLinearGradient(card, ui.LinearGradient{Start: ui.Point{X: card.X, Y: card.Y}, End: ui.Point{X: card.X + card.Width, Y: card.Y + card.Height}, From: from, To: to})
	canvas.RoundedBorder(card, 10, 1, vivaColor(bt.visuals.GlassEdge.Alpha(.72)), from)
	if a.dropOver {
		canvas.Rounded(card, 10, vivaColor(bt.theme.Accent.Alpha(.09)))
	}
	artHeight := card.Height - 70
	a.artwork(canvas, ui.Rect{X: card.X + 4, Y: card.Y + 6, Width: card.Width - 8, Height: artHeight}, bt.theme)
	name, sub := a.messages.IdleName, a.messages.IdleSubtitle
	if a.targetPath != "" {
		file := filepath.Base(a.targetPath)
		name = strings.TrimSuffix(file, filepath.Ext(file))
		sub = a.messages.Inspecting
		if a.state == pageIdle {
			sub = a.messages.InspectFailedSubtitle
		}
	}
	if a.inspection != nil {
		name, sub = a.inspection.Name, a.inspection.Version
		if sub == "" {
			sub = a.messages.VersionMissing
		}
	}
	if a.notice != "" && a.state == pageConfirm {
		sub = compactNotice(a.notice)
	}
	canvas.LabelCenter(name, ui.Rect{X: card.X + 10, Y: card.Y + card.Height - 60, Width: card.Width - 20, Height: 24}, ui.TextStyle{Size: 17, Weight: ui.WeightSemibold, Color: vivaColor(bt.theme.Text)})
	canvas.LabelCenter(sub, ui.Rect{X: card.X + 10, Y: card.Y + card.Height - 34, Width: card.Width - 20, Height: 20}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.TextMuted)})

	y := card.Y + card.Height + 10
	if a.link != nil && a.state == pageConfirm && a.inspection != nil {
		canvas.Label(fmt.Sprintf(a.messages.LocalApp, a.inspection.Path), ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 18}, ui.TextStyle{Size: 10, Color: vivaColor(bt.theme.TextMuted)})
		y += 20
	}
	if a.link != nil && a.state != pageConfirm {
		canvas.Label(fmt.Sprintf(a.messages.LinkProject, a.link.Project), ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 20}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.TextMuted)})
		y += 22
	}
	switch a.state {
	case pageIdle:
		title, message := a.messages.PrepareTitle, a.messages.PrepareMessage
		if a.linkResolving {
			title, message = a.messages.ResolveTitle, a.messages.ResolveMessage
		}
		canvas.Label(title, ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 22}, ui.TextStyle{Size: 11, Weight: ui.WeightBold, Color: vivaColor(bt.theme.Accent)})
		canvas.Label(message, ui.Rect{X: r.X + 2, Y: y + 23, Width: r.Width - 4, Height: 30}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.TextMuted)})
	case pageInspect:
		canvas.Label(a.messages.ProfileTitle, ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 22}, ui.TextStyle{Size: 16, Weight: ui.WeightBold, Color: vivaColor(bt.theme.Text)})
		canvas.Label(a.messages.ProfileMessage, ui.Rect{X: r.X + 2, Y: y + 25, Width: r.Width - 4, Height: 30}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.TextMuted)})
	case pageConfirm:
		y = a.paintConfirm(canvas, r, y, bt)
	case pageProcessing:
		canvas.Label(a.messages.ProcessingTitle, ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 22}, ui.TextStyle{Size: 16, Weight: ui.WeightBold, Color: vivaColor(bt.theme.Text)})
		msg := a.progress.Message
		switch a.progress.Stage {
		case "sign":
			msg = a.messages.ProgressSign
		case "quarantine":
			msg = a.messages.ProgressQuarantine
		case "finish":
			msg = a.messages.ProgressFinish
		case "verify":
			msg = a.messages.ProgressValidate
		}
		if a.stopping {
			msg = a.messages.Stopping
		}
		canvas.Label(msg, ui.Rect{X: r.X + 2, Y: y + 25, Width: r.Width - 4, Height: 30}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.TextMuted)})
	case pageResult:
		y = a.paintResult(canvas, r, y, bt)
	}
}

// compactNotice keeps worker and system errors readable inside the reserved
// one-line notice row of the fixed-size panel. The beginning retains the
// actionable category while the tail is shortened when a path or tool error
// is unusually long.
func compactNotice(notice string) string {
	const maxWidth = 338.0
	width := 0.0
	runes := []rune(notice)
	end := len(runes)
	for index, r := range runes {
		advance := 6.5
		if r > 0x7f {
			advance = 12
		}
		if width+advance > maxWidth-12 {
			end = index
			break
		}
		width += advance
	}
	if end == len(runes) {
		return notice
	}
	return string(runes[:end]) + "…"
}

func (a *application) paintConfirm(canvas *ui.Canvas, r ui.Rect, y float32, bt *brandTheme) float32 {
	i := a.inspection
	if i == nil {
		return y
	}
	q := a.messages.QuarantineNone
	if i.QuarantineCount > 0 {
		q = fmt.Sprintf(a.messages.QuarantineRemoved, i.QuarantineCount)
	} else if i.AlreadyProcessed {
		q = a.messages.QuarantineAlready
	}
	a.statusRow(canvas, r, y, a.messages.Signature, a.compactSignatureLabel(i.SignatureStatus), bt)
	a.statusRow(canvas, r, y+25, a.messages.Quarantine, q, bt)
	y += 45
	if lines := a.confirmationNoticeLines(canvas, r.Width-4); len(lines) > 0 {
		y += 8
		color := bt.theme.Accent
		if i.BlockedReason != "" {
			color = bt.theme.Warning
		}
		for _, line := range lines {
			canvas.Label(line, ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 18}, ui.TextStyle{Size: 12, Color: vivaColor(color)})
			y += 18
		}
	}
	return y
}

func (a *application) confirmationTrustOption() bool {
	i := a.inspection
	if a.state != pageConfirm || i == nil || i.BlockedReason != "" {
		return false
	}
	return a.prefs.RequireTrust
}

func (a *application) confirmationNoticeLines(canvas *ui.Canvas, width float32) []string {
	if a.inspection == nil {
		return nil
	}
	message := a.messages.CoreText(a.inspection.BlockedReason)
	if message == "" && a.inspection.AlreadyProcessed {
		message = a.messages.QuarantineAlready
	}
	return canvas.WrapLines(nil, message, 12, ui.WeightRegular, width, 2)
}

func (a *application) paintResult(canvas *ui.Canvas, r ui.Rect, y float32, bt *brandTheme) float32 {
	title := a.messages.Completed
	if a.result.Cancelled {
		title = a.messages.Canceled
	} else if a.result.Error != nil {
		title = a.messages.Failed
	} else if !a.result.CanOpen {
		title = a.messages.Recheck
	} else if a.opening {
		title = a.messages.Opening
	} else if !a.launchStarted.IsZero() {
		title = a.messages.OpenSubmitted
	}
	canvas.Label(title, ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 24}, ui.TextStyle{Size: 16, Weight: ui.WeightBold, Color: vivaColor(bt.theme.Text)})
	y += 30
	if a.result.Error != nil {
		canvas.Label(compactNotice(a.messages.CoreText(a.result.Error.Error())), ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 38}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.Warning)})
		y += 44
	}
	msg := a.messages.ResultDoneMessage
	if a.result.Changed > 0 {
		msg = fmt.Sprintf(a.messages.ResultChangedFmt, a.result.Changed)
	}
	if a.result.Cancelled {
		msg = a.messages.ResultCanceledMessage
	}
	canvas.Label(msg, ui.Rect{X: r.X + 2, Y: y, Width: r.Width - 4, Height: 34}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.TextMuted)})
	return y + 42
}
func (a *application) statusRow(canvas *ui.Canvas, r ui.Rect, y float32, label, value string, bt *brandTheme) {
	value = compactNotice(value)
	canvas.Label(label, ui.Rect{X: r.X + 2, Y: y, Width: 65, Height: 20}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.TextMuted)})
	canvas.Label(value, ui.Rect{X: r.X + 79, Y: y, Width: r.Width - 81, Height: 20}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.Text)})
}
func (a *application) paintActions(canvas *ui.Canvas, r ui.Rect, bt *brandTheme) {
	a.ensureMessages()
	y := r.Y
	trust := a.confirmationTrustOption()
	optionY := y
	if trust {
		optionY -= 31
		a.checkbox(canvas, idTrust, ui.Rect{X: r.X + 2, Y: optionY, Width: r.Width - 4, Height: 27}, a.trusted, a.messages.TrustCheckbox, false, bt)
	}
	switch a.state {
	case pageIdle:
		if a.button(canvas, idChoose, ui.Rect{X: r.X, Y: y, Width: r.Width, Height: 42}, a.messages.Choose, false, true, bt) {
			a.choose()
		}
	case pageInspect, pageProcessing:
		label, status := a.messages.StopChecking, a.messages.Inspecting
		if a.state == pageProcessing {
			label, status = a.messages.Stop, a.messages.Processing
		}
		if a.stopping {
			label = a.messages.StopChecking
		}
		if a.button(canvas, idStop, ui.Rect{X: r.X, Y: y, Width: 98, Height: 42}, label, a.stopping, false, bt) {
			a.stop()
		}
		a.button(canvas, idBusy, ui.Rect{X: r.X + 106, Y: y, Width: r.Width - 106, Height: 42}, status, true, true, bt)
	case pageConfirm:
		if a.button(canvas, idReplace, ui.Rect{X: r.X, Y: y, Width: 108, Height: 42}, a.messages.Replace, a.dialog, false, bt) {
			a.choose()
		}
		label := a.messages.Confirm
		if a.inspection != nil && a.inspection.AlreadyProcessed {
			label = a.messages.ConfirmOpen
		}
		disabled := a.dialog || a.inspection == nil || a.inspection.BlockedReason != "" || (a.prefs.RequireTrust && !a.trusted)
		if a.button(canvas, idConfirm, ui.Rect{X: r.X + 116, Y: y, Width: r.Width - 116, Height: 42}, label, disabled, true, bt) {
			a.process()
		}
	case pageResult:
		if a.button(canvas, idDone, ui.Rect{X: r.X, Y: y, Width: 92, Height: 42}, a.messages.Finish, a.opening, false, bt) {
			a.next()
		}
		if !a.result.CanOpen {
			if a.button(canvas, idRecheck, ui.Rect{X: r.X + 100, Y: y, Width: r.Width - 100, Height: 42}, a.messages.Recheck, !a.canRecheck(), true, bt) {
				a.recheck()
			}
		} else {
			label := a.messages.Open
			if a.opening {
				label = a.messages.OpeningShort
			} else if !a.launchStarted.IsZero() {
				label = a.messages.Again
			}
			if a.button(canvas, idOpen, ui.Rect{X: r.X + 100, Y: y, Width: r.Width - 100, Height: 42}, label, a.opening, true, bt) {
				a.openResult()
			}
		}
	}
	if a.state == pageConfirm {
		return
	}
	if a.notice != "" {
		canvas.Label(compactNotice(a.notice), ui.Rect{X: r.X, Y: r.Y + r.Height - 24, Width: 300, Height: 18}, ui.TextStyle{Size: 11, Color: vivaColor(bt.theme.TextMuted)})
	}
	shortcutLabel := "⇧⌘P"
	if a.shortcutError != "" {
		shortcutLabel = "菜单栏"
	}
	canvas.Label(shortcutLabel, ui.Rect{X: r.X + r.Width - 55, Y: r.Y + r.Height - 24, Width: 55, Height: 18}, ui.TextStyle{Size: 11, Color: vivaColor(bt.theme.TextMuted)})
}

func (a *application) button(canvas *ui.Canvas, id ui.ID, r ui.Rect, label string, disabled, primary bool, bt *brandTheme) bool {
	fill := vivaColor(bt.theme.Surface)
	if primary {
		fill = vivaColor(bt.visuals.AccentFrom)
	}
	if disabled {
		fill.Alpha = .45
	}
	canvas.RoundedBorder(r, 9, 1, vivaColor(bt.theme.Border), fill)
	labelColor := bt.theme.Text
	if primary {
		labelColor = bt.theme.AccentText
	}
	canvas.LabelCenter(label, r, ui.TextStyle{Size: 12, Weight: ui.WeightSemibold, Color: vivaColor(labelColor)})
	options := ui.HitOptions{Role: ui.RoleButton, Disabled: disabled}
	canvas.HitWithOptions(id, r, label, options)
	return canvas.Clicked(id) && !disabled
}
func (a *application) checkbox(canvas *ui.Canvas, id ui.ID, r ui.Rect, checked bool, label string, disabled bool, bt *brandTheme) {
	box := ui.Rect{X: r.X, Y: r.Y + 4, Width: 18, Height: 18}
	canvas.RoundedBorder(box, 4, 1, vivaColor(bt.theme.Border), vivaColor(bt.theme.Surface))
	if checked {
		canvas.Rounded(box, 4, vivaColor(bt.theme.Accent))
	}
	canvas.Label(label, ui.Rect{X: r.X + 27, Y: r.Y, Width: r.Width - 27, Height: 26}, ui.TextStyle{Size: 12, Color: vivaColor(bt.theme.Text)})
	canvas.HitWithOptions(id, ui.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}, label, ui.HitOptions{Role: ui.RoleCheckbox, Checked: checked, Disabled: disabled})
	if canvas.Clicked(id) && !disabled {
		if id == idTrust {
			a.trusted = !a.trusted
		} else if id == idMotion {
			a.prefs.Motion = !a.prefs.Motion
			a.savePreferences()
		} else if id == idRequire {
			a.prefs.RequireTrust = !a.prefs.RequireTrust
			a.trusted = false
			a.savePreferences()
		}
	}
}
