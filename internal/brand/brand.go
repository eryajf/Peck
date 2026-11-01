// Package brand contains Peck's native vector artwork and motion timing.
package brand

import (
	"math"
	"time"

	ui "github.com/eryajf/Peck/internal/vui"
)

var (
	Paper  = ui.Hex("#F5F3EE")
	Ink    = ui.Hex("#1E211F")
	Forest = ui.Hex("#245C46")
	Muted  = ui.Hex("#626860")
	Line   = ui.Hex("#D8D5CC")
	Body   = vector(`<path d="M48 115 29 166 67 146" fill="#252d28"/><path d="M59 64c-21 14-26 51-14 70 13 22 39 22 52 4 12-18 10-47-5-64Z" fill="#29332c"/><path d="M72 79c-14 4-22 26-16 43 4 13 14 17 27 17 13-18 14-39-11-60Z" fill="#f7f4e8"/><path d="M47 90c-10 19-6 42 13 48l11-46Z" fill="#424f41"/><path d="m47 97 15 6m-16 7 13 6" stroke="#b6c2a8" stroke-width="3" stroke-linecap="round"/><path d="m68 147-3 13m18-14 2 14m-26 0h14m5 0h13" stroke="#8c795c" stroke-width="3" stroke-linecap="round"/>`)
	Head   = vector(`<path d="M48 65C30 46 43 18 66 19c26 0 42 21 34 42l-9 23-29-5Z" fill="#29332c"/><path d="M49 24 43 8l16 7L64 1l9 15 20-5-7 18Z" fill="#c84b3a"/><path d="M69 43c9-8 25-7 31 3l-9 27-20-5-18-12Z" fill="#f5f2e6"/><path d="m98 43 42 10-43 9Z" fill="#b78c4d"/><path d="m99 53 41 0-43 9Z" fill="#927044"/><circle cx="88" cy="43" r="3.3" fill="#151c17"/><circle cx="89" cy="42" r=".8" fill="#fff"/>`)
	// Tray and AppIcon are assigned below from the approved brand masters. The
	// former temporary bird and letter-A artwork is intentionally gone: the same
	// mark now appears in the window, settings page, door scene and menu bar.
	Tray    ui.ImageSource = ui.NewIconSource(PeckMarkColor, ui.Hex("#FFFFFF"))
	AppIcon ui.ImageSource = embeddedAppIcon()
	Gear                   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><circle cx="12" cy="12" r="7"/><circle cx="12" cy="12" r="2.6"/><path d="M12 2v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2"/></svg>`))
)

func vector(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 180">` + paths + `</svg>`))
}

const (
	PeckBurstDuration  = 840 * time.Millisecond
	AppIconCornerRatio = .22
	markPeckCycleMS    = 1800.0
)

// PeckAngle emits three quick pecks followed by a brief rest.
func PeckAngle(elapsed time.Duration) float32 {
	const (
		cycle = 280.0
		rest  = 1600.0
	)
	t := math.Mod(float64(elapsed.Milliseconds()), rest)
	if t >= float64(PeckBurstDuration.Milliseconds()) {
		return 0
	}
	t = math.Mod(t, cycle)
	switch {
	case t < 45:
		return 0
	case t < 125:
		return float32(19 * math.Sin((t-45)/80*math.Pi/2))
	default:
		return float32(19 * (1 - (t-125)/155))
	}
}

// MarkPeckAngle emits three quick forward taps, each with a small rebound,
// followed by a clear rest. It mirrors the web button artwork while making
// the burst read like a woodpecker's short drumming sequence.
func MarkPeckAngle(elapsed time.Duration) float32 {
	const cycle = markPeckCycleMS
	t := math.Mod(float64(elapsed.Milliseconds()), cycle)
	key := func(from, to, start, end float64) float32 {
		if t <= start {
			return float32(from)
		}
		if t >= end {
			return float32(to)
		}
		progress := (t - start) / (end - start)
		// CSS's ease-in-out is close to this smoothstep curve while keeping the
		// native implementation deterministic at the 30 fps paint cadence.
		progress = progress * progress * (3 - 2*progress)
		return float32(from + (to-from)*progress)
	}
	switch {
	case t < 70:
		return 0
	case t < 170:
		return key(0, 10, 70, 170)
	case t < 210:
		return key(10, -3, 170, 210)
	case t < 260:
		return key(-3, 0, 210, 260)
	case t < 280:
		return 0
	case t < 380:
		return key(0, 10, 280, 380)
	case t < 420:
		return key(10, -3, 380, 420)
	case t < 470:
		return key(-3, 0, 420, 470)
	case t < 490:
		return 0
	case t < 590:
		return key(0, 10, 490, 590)
	case t < 630:
		return key(10, -3, 590, 630)
	case t < 680:
		return key(-3, 0, 630, 680)
	default:
		return 0
	}
}

// MarkPeckImpact returns the opacity of the tiny impact strokes in front of
// the beak. The three short windows line up with the three forward taps.
func MarkPeckImpact(elapsed time.Duration) float32 {
	const cycle = markPeckCycleMS
	t := math.Mod(float64(elapsed.Milliseconds()), cycle)
	alpha := func(start, peak, end float64) float32 {
		switch {
		case t <= start || t >= end:
			return 0
		case t < peak:
			return float32((t - start) / (peak - start))
		default:
			return float32((end - t) / (end - peak))
		}
	}
	return maxFloat32(alpha(145, 170, 210), maxFloat32(alpha(355, 380, 420), alpha(565, 590, 630)))
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// Bird paints the approved vector with an independently moving head.
func Bird(p *ui.Painter, r ui.Rect, angle float32) {
	// The detailed scene uses paintSceneBird. Keep this helper for callers that
	// need a compact brand mark in a status-area preview.
	p.Image(Body, r, ui.Contain)
	p.Transform(ui.Rotation(angle), r.X+r.W*.44, r.Y+r.H*.42, func() { p.Image(Head, r, ui.Contain) })
}

type AppearanceOption struct {
	Value       string
	Label       string
	Description string
	PreviewFrom ui.Color
	PreviewTo   ui.Color
	AccentFrom  ui.Color
	AccentTo    ui.Color
	Dark        bool
	System      bool
}

var appearanceOptions = []AppearanceOption{
	{
		Value: "system", Label: "跟随系统", Description: "自动匹配 macOS 的浅色或深色",
		PreviewFrom: Paper, PreviewTo: ui.Hex("#E8F0F1"), AccentFrom: Forest, AccentTo: ui.Hex("#4A8B78"), System: true,
	},
	{
		Value: "rose", Label: "玫瑰云", Description: "玫瑰、薰衣草与柔光",
		PreviewFrom: ui.Hex("#FFF5FB"), PreviewTo: ui.Hex("#F0E1FF"), AccentFrom: ui.Hex("#D9578B"), AccentTo: ui.Hex("#805BD2"),
	},
	{
		Value: "sunset", Label: "晚霞", Description: "蜜桃、珊瑚与莓果余晖",
		PreviewFrom: ui.Hex("#FFF7E9"), PreviewTo: ui.Hex("#FFD6C0"), AccentFrom: ui.Hex("#E96B45"), AccentTo: ui.Hex("#B34183"),
	},
	{
		Value: "paper", Label: "暖纸", Description: "奶油纸张与青绿光泽",
		PreviewFrom: ui.Hex("#FFF9F1"), PreviewTo: ui.Hex("#E4F0EC"), AccentFrom: ui.Hex("#2F806B"), AccentTo: ui.Hex("#174B4B"),
	},
	{
		Value: "midnight", Label: "夜林", Description: "夜幕、薄荷与深林蓝绿",
		PreviewFrom: ui.Hex("#151D26"), PreviewTo: ui.Hex("#29443E"), AccentFrom: ui.Hex("#9AE6C2"), AccentTo: ui.Hex("#5BAAA1"), Dark: true,
	},
	{
		Value: "mist", Label: "雾蓝", Description: "晨雾蓝与柔和钴色",
		PreviewFrom: ui.Hex("#F4F8FF"), PreviewTo: ui.Hex("#DCE9FA"), AccentFrom: ui.Hex("#3F81D9"), AccentTo: ui.Hex("#5E66C9"),
	},
	{
		Value: "clay", Label: "陶土", Description: "日落陶土与莓果光晕",
		PreviewFrom: ui.Hex("#FFF8F2"), PreviewTo: ui.Hex("#F0D5D8"), AccentFrom: ui.Hex("#D66A5F"), AccentTo: ui.Hex("#A84E76"),
	},
	{
		Value: "ink", Label: "墨金", Description: "象牙纸、墨色与琥珀金",
		PreviewFrom: ui.Hex("#FFF9E9"), PreviewTo: ui.Hex("#EAD8B3"), AccentFrom: ui.Hex("#B47D35"), AccentTo: ui.Hex("#76502D"),
	},
	{
		Value: "aurora", Label: "极光", Description: "冰川青、薄荷与暮蓝",
		PreviewFrom: ui.Hex("#EAFBFF"), PreviewTo: ui.Hex("#D8F2E7"), AccentFrom: ui.Hex("#247FBA"), AccentTo: ui.Hex("#5864D0"),
	},
	{
		Value: "ocean", Label: "深海", Description: "深海蓝与清透蓝光",
		PreviewFrom: ui.Hex("#101B35"), PreviewTo: ui.Hex("#164C68"), AccentFrom: ui.Hex("#61D8E9"), AccentTo: ui.Hex("#6A88FF"), Dark: true,
	},
	{
		Value: "lavender", Label: "薰衣草", Description: "暮紫、兰花与莓紫光",
		PreviewFrom: ui.Hex("#F8F5FF"), PreviewTo: ui.Hex("#EADDF9"), AccentFrom: ui.Hex("#7657D8"), AccentTo: ui.Hex("#C151B8"),
	},
	{
		Value: "citrus", Label: "柑橘", Description: "柠檬、蜜糖与橘红",
		PreviewFrom: ui.Hex("#FFFBEA"), PreviewTo: ui.Hex("#FFE4B2"), AccentFrom: ui.Hex("#E78C26"), AccentTo: ui.Hex("#C94C46"),
	},
}

type themePalette struct {
	dark                                   bool
	background, backgroundEnd              ui.Color
	surface, surfaceEnd, surfaceHover      ui.Color
	surfacePressed, border                 ui.Color
	text, textMuted, accent                ui.Color
	accentEnd, accentHover, accentPressed  ui.Color
	accentText                             ui.Color
	danger, warning, success               ui.Color
	selection, focus, inverse, inverseText ui.Color
	scrollbar                              ui.Color
}

func AppearanceOptions() []AppearanceOption {
	return append([]AppearanceOption(nil), appearanceOptions...)
}

func NormalizeAppearance(appearance string) string {
	switch appearance {
	case "system":
		return "system"
	case "light":
		return "paper"
	case "dark":
		return "midnight"
	case "paper", "midnight", "mist", "clay", "ink", "aurora", "rose", "sunset", "ocean", "lavender", "citrus":
		return appearance
	default:
		return "system"
	}
}

func paletteFor(appearance string) themePalette {
	switch appearance {
	case "midnight":
		return themePalette{
			dark:       true,
			background: ui.Hex("#151D26"), backgroundEnd: ui.Hex("#29443E"), surface: ui.Hex("#26343A"), surfaceEnd: ui.Hex("#304C45"), surfaceHover: ui.Hex("#3A514B"),
			surfacePressed: ui.Hex("#3F4F41"), border: ui.Hex("#4D5C4F"),
			text: ui.Hex("#F3F4ED"), textMuted: ui.Hex("#B4BDB0"),
			accent: ui.Hex("#9AE6C2"), accentEnd: ui.Hex("#5BAAA1"), accentHover: ui.Hex("#B7F1D1"), accentPressed: ui.Hex("#7CC8B5"), accentText: ui.Hex("#14241F"),
			danger: ui.Hex("#E08C7F"), warning: ui.Hex("#E8B673"), success: ui.Hex("#9FD2AE"),
			selection: ui.RGBA(168, 209, 180, .25), focus: ui.Hex("#9AE6C2"),
			inverse: ui.Hex("#F3F4ED"), inverseText: ui.Hex("#202722"), scrollbar: ui.RGBA(255, 255, 255, .35),
		}
	case "mist":
		return themePalette{
			background: ui.Hex("#F4F8FF"), backgroundEnd: ui.Hex("#DCE9FA"), surface: ui.Hex("#E7F0FA"), surfaceEnd: ui.Hex("#E0E7F5"), surfaceHover: ui.Hex("#D4E5F2"),
			surfacePressed: ui.Hex("#C5DADD"), border: ui.Hex("#C4D6D9"),
			text: ui.Hex("#203037"), textMuted: ui.Hex("#5F7479"),
			accent: ui.Hex("#3F81D9"), accentEnd: ui.Hex("#5E66C9"), accentHover: ui.Hex("#5B9BE9"), accentPressed: ui.Hex("#326DBB"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#B95C62"), warning: ui.Hex("#9A6B28"), success: ui.Hex("#2E806C"),
			selection: ui.RGBA(45, 104, 117, .18), focus: ui.Hex("#3F81D9"),
			inverse: ui.Hex("#203037"), inverseText: ui.Hex("#EEF5F5"), scrollbar: ui.RGBA(32, 48, 55, .28),
		}
	case "clay":
		return themePalette{
			background: ui.Hex("#FFF8F2"), backgroundEnd: ui.Hex("#F0D5D8"), surface: ui.Hex("#F9E8E3"), surfaceEnd: ui.Hex("#F3DEE9"), surfaceHover: ui.Hex("#EFD8D6"),
			surfacePressed: ui.Hex("#E3CAC2"), border: ui.Hex("#DDC9C1"),
			text: ui.Hex("#332A2A"), textMuted: ui.Hex("#766B69"),
			accent: ui.Hex("#D66A5F"), accentEnd: ui.Hex("#A84E76"), accentHover: ui.Hex("#E17D70"), accentPressed: ui.Hex("#AF4F58"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#B85251"), warning: ui.Hex("#9A642E"), success: ui.Hex("#5A866A"),
			selection: ui.RGBA(158, 91, 87, .18), focus: ui.Hex("#D66A5F"),
			inverse: ui.Hex("#332A2A"), inverseText: ui.Hex("#FAF2EE"), scrollbar: ui.RGBA(51, 42, 42, .28),
		}
	case "ink":
		return themePalette{
			background: ui.Hex("#FFF9E9"), backgroundEnd: ui.Hex("#EAD8B3"), surface: ui.Hex("#F5EBD4"), surfaceEnd: ui.Hex("#EFE0C4"), surfaceHover: ui.Hex("#E9D9B8"),
			surfacePressed: ui.Hex("#DCCCAD"), border: ui.Hex("#D8CBB5"),
			text: ui.Hex("#2E2922"), textMuted: ui.Hex("#766B5B"),
			accent: ui.Hex("#B47D35"), accentEnd: ui.Hex("#76502D"), accentHover: ui.Hex("#C3924A"), accentPressed: ui.Hex("#8C5B2A"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#AF5A4A"), warning: ui.Hex("#9A6B2A"), success: ui.Hex("#5F8067"),
			selection: ui.RGBA(138, 99, 56, .18), focus: ui.Hex("#B47D35"),
			inverse: ui.Hex("#2E2922"), inverseText: ui.Hex("#F7F2E7"), scrollbar: ui.RGBA(46, 41, 34, .28),
		}
	case "aurora":
		return themePalette{
			background: ui.Hex("#EAFBFF"), backgroundEnd: ui.Hex("#D8F2E7"), surface: ui.Hex("#E5F5F5"), surfaceEnd: ui.Hex("#D8EDEB"), surfaceHover: ui.Hex("#CBE5E5"),
			surfacePressed: ui.Hex("#B8D9DA"), border: ui.Hex("#BFD9D8"),
			text: ui.Hex("#1B2B33"), textMuted: ui.Hex("#5D7476"),
			accent: ui.Hex("#247FBA"), accentEnd: ui.Hex("#5864D0"), accentHover: ui.Hex("#3397D1"), accentPressed: ui.Hex("#1C659B"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#C65D67"), warning: ui.Hex("#9A6B28"), success: ui.Hex("#2E8567"),
			selection: ui.RGBA(36, 127, 186, .18), focus: ui.Hex("#247FBA"),
			inverse: ui.Hex("#1B2B33"), inverseText: ui.Hex("#EAFBFF"), scrollbar: ui.RGBA(27, 43, 51, .28),
		}
	case "rose":
		return themePalette{
			background: ui.Hex("#FFF5FB"), backgroundEnd: ui.Hex("#F0E1FF"), surface: ui.Hex("#F8EAF4"), surfaceEnd: ui.Hex("#EEE3F7"), surfaceHover: ui.Hex("#E9D5EC"),
			surfacePressed: ui.Hex("#DEC5E5"), border: ui.Hex("#DECBE3"),
			text: ui.Hex("#34233B"), textMuted: ui.Hex("#786878"),
			accent: ui.Hex("#D9578B"), accentEnd: ui.Hex("#805BD2"), accentHover: ui.Hex("#E56EA0"), accentPressed: ui.Hex("#B94172"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#BB5263"), warning: ui.Hex("#9A6B2E"), success: ui.Hex("#5C8A78"),
			selection: ui.RGBA(217, 87, 139, .18), focus: ui.Hex("#D9578B"),
			inverse: ui.Hex("#34233B"), inverseText: ui.Hex("#FFF5FB"), scrollbar: ui.RGBA(52, 35, 59, .28),
		}
	case "sunset":
		return themePalette{
			background: ui.Hex("#FFF7E9"), backgroundEnd: ui.Hex("#FFD6C0"), surface: ui.Hex("#FBEBD7"), surfaceEnd: ui.Hex("#F3DADF"), surfaceHover: ui.Hex("#F0D0C2"),
			surfacePressed: ui.Hex("#E6BBAF"), border: ui.Hex("#E5C9BA"),
			text: ui.Hex("#3A2925"), textMuted: ui.Hex("#78655E"),
			accent: ui.Hex("#E96B45"), accentEnd: ui.Hex("#B34183"), accentHover: ui.Hex("#F27D56"), accentPressed: ui.Hex("#C65348"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#B94E54"), warning: ui.Hex("#9B672B"), success: ui.Hex("#5E8969"),
			selection: ui.RGBA(233, 107, 69, .18), focus: ui.Hex("#E96B45"),
			inverse: ui.Hex("#3A2925"), inverseText: ui.Hex("#FFF7E9"), scrollbar: ui.RGBA(58, 41, 37, .28),
		}
	case "ocean":
		return themePalette{
			dark:       true,
			background: ui.Hex("#101B35"), backgroundEnd: ui.Hex("#164C68"), surface: ui.Hex("#1A2C4A"), surfaceEnd: ui.Hex("#224E60"), surfaceHover: ui.Hex("#2A5C6A"),
			surfacePressed: ui.Hex("#356C79"), border: ui.Hex("#3B6678"),
			text: ui.Hex("#EEF8FF"), textMuted: ui.Hex("#B2C8D2"),
			accent: ui.Hex("#61D8E9"), accentEnd: ui.Hex("#6A88FF"), accentHover: ui.Hex("#83E7F2"), accentPressed: ui.Hex("#4EB6DA"), accentText: ui.Hex("#11243A"),
			danger: ui.Hex("#F18C8F"), warning: ui.Hex("#EFC37B"), success: ui.Hex("#78E0B7"),
			selection: ui.RGBA(97, 216, 233, .25), focus: ui.Hex("#61D8E9"),
			inverse: ui.Hex("#EEF8FF"), inverseText: ui.Hex("#101B35"), scrollbar: ui.RGBA(255, 255, 255, .35),
		}
	case "lavender":
		return themePalette{
			background: ui.Hex("#F8F5FF"), backgroundEnd: ui.Hex("#EADDF9"), surface: ui.Hex("#F1EBFC"), surfaceEnd: ui.Hex("#E8DDF4"), surfaceHover: ui.Hex("#DDD0EE"),
			surfacePressed: ui.Hex("#D0C0E4"), border: ui.Hex("#D9CDE7"),
			text: ui.Hex("#29253C"), textMuted: ui.Hex("#716B83"),
			accent: ui.Hex("#7657D8"), accentEnd: ui.Hex("#C151B8"), accentHover: ui.Hex("#8C6BE7"), accentPressed: ui.Hex("#6044B7"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#B85C70"), warning: ui.Hex("#9B6D32"), success: ui.Hex("#5D8B78"),
			selection: ui.RGBA(118, 87, 216, .18), focus: ui.Hex("#7657D8"),
			inverse: ui.Hex("#29253C"), inverseText: ui.Hex("#F8F5FF"), scrollbar: ui.RGBA(41, 37, 60, .28),
		}
	case "citrus":
		return themePalette{
			background: ui.Hex("#FFFBEA"), backgroundEnd: ui.Hex("#FFE4B2"), surface: ui.Hex("#F9F0D6"), surfaceEnd: ui.Hex("#F7DCCB"), surfaceHover: ui.Hex("#EFD3B2"),
			surfacePressed: ui.Hex("#E4C29A"), border: ui.Hex("#E4D0A9"),
			text: ui.Hex("#3A2E20"), textMuted: ui.Hex("#75664F"),
			accent: ui.Hex("#E78C26"), accentEnd: ui.Hex("#C94C46"), accentHover: ui.Hex("#F1A044"), accentPressed: ui.Hex("#C76D20"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#B8534D"), warning: ui.Hex("#946624"), success: ui.Hex("#5D8560"),
			selection: ui.RGBA(231, 140, 38, .18), focus: ui.Hex("#E78C26"),
			inverse: ui.Hex("#3A2E20"), inverseText: ui.Hex("#FFFBEA"), scrollbar: ui.RGBA(58, 46, 32, .28),
		}
	default:
		return themePalette{
			background: ui.Hex("#FFF9F1"), backgroundEnd: ui.Hex("#E4F0EC"), surface: ui.Hex("#F1EEE8"), surfaceEnd: ui.Hex("#E1EEE9"), surfaceHover: ui.Hex("#E4EAE6"),
			surfacePressed: ui.Hex("#DCD8CE"), border: Line,
			text: Ink, textMuted: Muted,
			accent: ui.Hex("#2F806B"), accentEnd: ui.Hex("#174B4B"), accentHover: ui.Hex("#469A83"), accentPressed: ui.Hex("#216455"), accentText: ui.Hex("#FFFFFF"),
			danger: ui.Hex("#B94D43"), warning: ui.Hex("#8A4D16"), success: ui.Hex("#2E7856"),
			selection: ui.RGBA(36, 92, 70, .18), focus: ui.Hex("#2F806B"),
			inverse: Ink, inverseText: Paper, scrollbar: ui.RGBA(30, 33, 31, .30),
		}
	}
}

type VisualStyle struct {
	BackgroundFrom ui.Color
	BackgroundTo   ui.Color
	SurfaceFrom    ui.Color
	SurfaceTo      ui.Color
	AccentFrom     ui.Color
	AccentTo       ui.Color
	GlassEdge      ui.Color
	GlassGlow      ui.Color
}

func resolveAppearance(appearance string, dark bool) string {
	appearance = NormalizeAppearance(appearance)
	if appearance == "system" {
		if dark {
			return "midnight"
		}
		return "paper"
	}
	return appearance
}

func Visuals(appearance string, dark bool) VisualStyle {
	p := paletteFor(resolveAppearance(appearance, dark))
	return VisualStyle{
		BackgroundFrom: p.background,
		BackgroundTo:   p.backgroundEnd,
		SurfaceFrom:    p.surface,
		SurfaceTo:      p.surfaceEnd,
		AccentFrom:     p.accent,
		AccentTo:       p.accentEnd,
		GlassEdge:      ui.Hex("#FFFFFF").Alpha(.42),
		GlassGlow:      p.accentEnd.Alpha(.16),
	}
}

// Theme preserves system control semantics and accessibility preferences.
func Theme(appearance string, dark bool) *ui.Theme {
	p := paletteFor(resolveAppearance(appearance, dark))
	t := &ui.Theme{Dark: p.dark}
	t.Background = p.background
	t.Surface = p.surface
	t.SurfaceHover = p.surfaceHover
	t.SurfacePressed = p.surfacePressed
	t.Border = p.border
	t.Text = p.text
	t.TextMuted = p.textMuted
	t.Accent = p.accent
	t.AccentHover = p.accentHover
	t.AccentPressed = p.accentPressed
	t.AccentText = p.accentText
	t.Danger = p.danger
	t.Warning = p.warning
	t.Success = p.success
	t.Selection = p.selection
	t.Focus = p.focus
	t.Inverse = p.inverse
	t.InverseText = p.inverseText
	t.Scrollbar = p.scrollbar
	return t
}
