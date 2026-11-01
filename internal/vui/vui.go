// Package vui contains the small drawing vocabulary used by Peck's brand
// artwork. It deliberately sits on top of Viva's retained Canvas primitives;
// the application never links a second painter or widget runtime.
package vui

import (
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"github.com/vivaui/viva/ui"
)

type Color struct{ Red, Green, Blue, A float32 }

func Hex(value string) Color {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 3 {
		value = string([]byte{value[0], value[0], value[1], value[1], value[2], value[2]})
	}
	if len(value) != 6 && len(value) != 8 {
		return Color{}
	}
	n, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return Color{}
	}
	if len(value) == 6 {
		return Color{Red: float32((n>>16)&255) / 255, Green: float32((n>>8)&255) / 255, Blue: float32(n&255) / 255, A: 1}
	}
	return Color{Red: float32((n>>24)&255) / 255, Green: float32((n>>16)&255) / 255, Blue: float32((n>>8)&255) / 255, A: float32(n&255) / 255}
}

func RGBA(r, g, b uint8, a float32) Color {
	return Color{Red: float32(r) / 255, Green: float32(g) / 255, Blue: float32(b) / 255, A: a}
}

var Transparent = Color{}

func (c Color) Alpha(a float32) Color { c.A = a; return c }
func (c Color) Mix(other Color, t float32) Color {
	if t <= 0 {
		return c
	}
	if t >= 1 {
		return other
	}
	return Color{Red: c.Red + (other.Red-c.Red)*t, Green: c.Green + (other.Green-c.Green)*t, Blue: c.Blue + (other.Blue-c.Blue)*t, A: c.A + (other.A-c.A)*t}
}

func toUI(c Color) ui.Color { return ui.Color{Red: c.Red, Green: c.Green, Blue: c.Blue, Alpha: c.A} }

type Rect struct{ X, Y, W, H float32 }
type Point struct{ X, Y float32 }
type Fit uint8

const (
	Contain Fit = iota
	FillBox
)

type GradientStop struct {
	Color Color
	At    float32
}
type LinearGradient struct {
	From, To Color
	Angle    float32
	Stops    []GradientStop
}

type Theme struct {
	Dark                                                                        bool
	Background, Surface, SurfaceHover, SurfacePressed, Border                   Color
	Text, TextMuted, Accent, AccentHover, AccentPressed, AccentText             Color
	Danger, Warning, Success, Selection, Focus, Inverse, InverseText, Scrollbar Color
}

func LightTheme() *Theme { return &Theme{} }
func DarkTheme() *Theme  { return &Theme{Dark: true} }

type pathCommand struct {
	kind      byte
	p, c1, c2 Point
}
type Path struct{ commands []pathCommand }

func (p *Path) Reset() *Path { p.commands = p.commands[:0]; return p }
func (p *Path) MoveTo(x, y float32) *Path {
	p.commands = append(p.commands, pathCommand{kind: 'M', p: Point{x, y}})
	return p
}
func (p *Path) LineTo(x, y float32) *Path {
	p.commands = append(p.commands, pathCommand{kind: 'L', p: Point{x, y}})
	return p
}
func (p *Path) QuadTo(cx, cy, x, y float32) *Path {
	p.commands = append(p.commands, pathCommand{kind: 'Q', p: Point{x, y}, c1: Point{cx, cy}})
	return p
}
func (p *Path) CubeTo(c1x, c1y, c2x, c2y, x, y float32) *Path {
	p.commands = append(p.commands, pathCommand{kind: 'C', p: Point{x, y}, c1: Point{c1x, c1y}, c2: Point{c2x, c2y}})
	return p
}
func (p *Path) Close() *Path { p.commands = append(p.commands, pathCommand{kind: 'Z'}); return p }
func (p *Path) Circle(cx, cy, radius float32) *Path {
	const k = .55228475
	p.MoveTo(cx+radius, cy).
		CubeTo(cx+radius, cy+radius*k, cx+radius*k, cy+radius, cx, cy+radius).
		CubeTo(cx-radius*k, cy+radius, cx-radius, cy+radius*k, cx-radius, cy).
		CubeTo(cx-radius, cy-radius*k, cx-radius*k, cy-radius, cx, cy-radius).
		CubeTo(cx+radius*k, cy-radius, cx+radius, cy-radius*k, cx+radius, cy)
	return p.Close()
}

type Transform struct{ a, b, c, d, e, f float32 }

func identity() Transform                { return Transform{a: 1, d: 1} }
func Translation(x, y float32) Transform { return Transform{a: 1, d: 1, e: x, f: y} }
func Scaling(x, y float32) Transform     { return Transform{a: x, d: y} }
func Rotation(degrees float32) Transform {
	r := float64(degrees) * math.Pi / 180
	s, c := math.Sincos(r)
	return Transform{a: float32(c), b: float32(s), c: float32(-s), d: float32(c)}
}
func (t Transform) Then(next Transform) Transform {
	return Transform{a: next.a*t.a + next.c*t.b, b: next.b*t.a + next.d*t.b, c: next.a*t.c + next.c*t.d, d: next.b*t.c + next.d*t.d, e: next.a*t.e + next.c*t.f + next.e, f: next.b*t.e + next.d*t.f + next.f}
}
func (t Transform) point(p Point) Point {
	return Point{X: t.a*p.X + t.c*p.Y + t.e, Y: t.b*p.X + t.d*p.Y + t.f}
}

type ImageSource interface{ draw(*Painter, Rect) }

// BitmapSource adapts an immutable Viva bitmap to the same image vocabulary as
// the vector brand assets. The bitmap is retained by the canvas and must not be
// mutated after construction.
type BitmapSource struct {
	bitmap ui.Bitmap
}

func NewBitmapSource(bitmap ui.Bitmap) *BitmapSource {
	return &BitmapSource{bitmap: bitmap}
}

func (source *BitmapSource) draw(p *Painter, r Rect) {
	if source == nil {
		return
	}
	p.canvas.DrawBitmap(source.bitmap, p.bounds(r))
}

// IconSource adapts a compile-time Viva icon to the image vocabulary used by
// Peck's native scenes. The icon's own layers keep their brand colors; tint is
// used for the alpha and for one-color paths.
type IconSource struct {
	icon ui.Icon
	tint ui.Color
}

func NewIconSource(icon ui.Icon, tint Color) *IconSource {
	return &IconSource{icon: icon, tint: toUI(tint)}
}

func (source *IconSource) draw(p *Painter, r Rect) {
	if source == nil {
		return
	}
	// Canvas.DrawIcon fits the icon into an axis-aligned rectangle. That is
	// exactly what we want for a still icon, but it causes a transformed icon
	// to be fitted again after its corners rotate. The result is a small scale
	// and position jump on every animation frame. Paint the compiled paths in
	// the painter's transformed coordinate space instead.
	if p.isIdentity() {
		p.canvas.DrawIcon(source.icon, p.bounds(r), source.tint)
		return
	}
	p.drawTransformedIcon(source.icon, r, source.tint)
}

func (p *Painter) isIdentity() bool {
	return p.transform == identity()
}

func (p *Painter) drawTransformedIcon(icon ui.Icon, r Rect, tint ui.Color) {
	if icon.ViewBox <= 0 || (len(icon.Commands) == 0 && len(icon.Layers) == 0) || tint.Alpha <= 0 {
		return
	}
	height := icon.ViewHeight
	if height <= 0 {
		height = icon.ViewBox
	}
	scale := min(r.W/icon.ViewBox, r.H/height)
	if scale <= 0 {
		return
	}
	target := Rect{
		X: r.X + (r.W-icon.ViewBox*scale)/2,
		Y: r.Y + (r.H-height*scale)/2,
		W: icon.ViewBox * scale,
		H: height * scale,
	}

	if len(icon.Commands) > 0 {
		path, bounds, ok := p.transformedIconPath(icon.Commands, target, scale)
		if ok {
			style := ui.PathStyle{Stroke: tint, StrokeWidth: icon.Stroke * scale, Cap: icon.Cap, Join: icon.Join}
			if icon.Stroke <= 0 {
				style.Stroke = ui.Color{}
				style.Fill = tint
			}
			p.canvas.DrawPath(path, bounds, style)
		}
	}
	for _, layer := range icon.Layers {
		path, bounds, ok := p.transformedIconPath(layer.Commands, target, scale)
		if !ok {
			continue
		}
		fill := layer.Fill
		fill.Alpha *= tint.Alpha
		p.canvas.DrawPath(path, bounds, ui.PathStyle{Fill: fill, FillRule: layer.FillRule})
	}
}

// transformedIconPath converts a compiled icon path to the painter's current
// transform and then normalizes it for Canvas.DrawPath. Normalizing is needed
// because Canvas.DrawPath accepts a view box plus bounds; it does not expose a
// mutable transform stack of its own.
func (p *Painter) transformedIconPath(commands []ui.PathCommand, target Rect, scale float32) (ui.Path, ui.Rect, bool) {
	if len(commands) == 0 {
		return ui.Path{}, ui.Rect{}, false
	}
	mapPoint := func(point ui.Point) Point {
		return p.mapped(Point{X: target.X + point.X*scale, Y: target.Y + point.Y*scale})
	}
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	add := func(point Point) {
		minX, minY = min(minX, point.X), min(minY, point.Y)
		maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
	}
	for _, command := range commands {
		if command.Kind == ui.PathCommandClose {
			continue
		}
		add(mapPoint(command.Point))
		if command.Kind == ui.PathCommandQuad || command.Kind == ui.PathCommandCubic {
			add(mapPoint(command.Control1))
		}
		if command.Kind == ui.PathCommandCubic {
			add(mapPoint(command.Control2))
		}
	}
	if minX > maxX || minY > maxY {
		return ui.Path{}, ui.Rect{}, false
	}
	w, h := maxX-minX, maxY-minY
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	converted := make([]ui.PathCommand, 0, len(commands))
	for _, command := range commands {
		if command.Kind == ui.PathCommandClose {
			converted = append(converted, ui.ClosePath())
			continue
		}
		point := mapPoint(command.Point)
		point.X -= minX
		point.Y -= minY
		switch command.Kind {
		case ui.PathCommandMove:
			converted = append(converted, ui.MoveTo(point.X, point.Y))
		case ui.PathCommandLine:
			converted = append(converted, ui.LineTo(point.X, point.Y))
		case ui.PathCommandQuad:
			control := mapPoint(command.Control1)
			control.X -= minX
			control.Y -= minY
			converted = append(converted, ui.QuadTo(control.X, control.Y, point.X, point.Y))
		case ui.PathCommandCubic:
			control1, control2 := mapPoint(command.Control1), mapPoint(command.Control2)
			control1.X -= minX
			control1.Y -= minY
			control2.X -= minX
			control2.Y -= minY
			converted = append(converted, ui.CubicTo(control1.X, control1.Y, control2.X, control2.Y, point.X, point.Y))
		}
	}
	return ui.NewPath(w, h, converted...), ui.Rect{X: minX, Y: minY, Width: w, Height: h}, true
}

type SVG struct {
	seed uint32
	raw  string
	kind svgKind
}

type svgKind uint8

const (
	svgGeneric svgKind = iota
	svgTray
	svgApp
	svgInsect0
	svgInsect1
	svgInsect2
	svgInsect3
)

func MustParseSVG(data []byte) *SVG {
	h := fnv.New32a()
	_, _ = h.Write(data)
	raw := string(data)
	kind := svgGeneric
	switch {
	case strings.Contains(raw, `viewBox="0 0 32 36"`) && strings.Contains(raw, "m11 7-2-5"):
		kind = svgTray
	case strings.Contains(raw, `viewBox="0 0 64 64"`):
		kind = svgApp
	case strings.Contains(raw, `viewBox="0 0 32 32"`):
		// The four insect assets are stable compile-time vectors. Their first
		// distinctive path is enough to retain their identity without pulling
		// an SVG parser into the shipped application.
		for i, marker := range []string{"m6 22-1 3", "m10 12-5-3", "M5 24c7 3", "M15 15C3-1"} {
			if strings.Contains(raw, marker) {
				kind = svgKind(int(svgInsect0) + i)
				break
			}
		}
	}
	return &SVG{seed: h.Sum32(), raw: raw, kind: kind}
}

func (s *SVG) draw(p *Painter, r Rect) {
	if s == nil {
		return
	}
	// The small set of brand assets are drawn as native paths so they remain
	// resolution independent on Viva's GPU canvas. The coordinates below mirror
	// the source vectors used by the former native painter.
	switch s.kind {
	case svgApp:
		p.fillRound(r, r.W*14/64, Hex("#DDE8D9"))
		var a Path
		a.MoveTo(r.X+r.W*.25, r.Y+r.H*.69).LineTo(r.X+r.W*.47, r.Y+r.H*.30).LineTo(r.X+r.W*.56, r.Y+r.H*.30).LineTo(r.X+r.W*.78, r.Y+r.H*.69).LineTo(r.X+r.W*.64, r.Y+r.H*.69).LineTo(r.X+r.W*.57, r.Y+r.H*.58).LineTo(r.X+r.W*.42, r.Y+r.H*.58).LineTo(r.X+r.W*.36, r.Y+r.H*.69).Close()
		p.FillPath(&a, Hex("#245C46"))
		// The source mark has a transparent triangular counter. Paint it back
		// with the icon's paper so the glyph keeps its open A at small sizes.
		var counter Path
		counter.MoveTo(r.X+r.W*.455, r.Y+r.H*.52).LineTo(r.X+r.W*.535, r.Y+r.H*.52).LineTo(r.X+r.W*.495, r.Y+r.H*.39).Close()
		p.FillPath(&counter, Hex("#DDE8D9"))
	case svgTray:
		p.Transform(Scaling(r.W/32, r.H/36).Then(Translation(r.X, r.Y)), 0, 0, func() {
			var bird Path
			bird.MoveTo(11, 7).LineTo(9, 2).LineTo(15, 4).LineTo(17, 0).LineTo(22, 6).
				CubeTo(28, 8, 29, 12, 27, 17).LineTo(32, 18).LineTo(25, 22).LineTo(23, 26).
				CubeTo(20, 32, 15, 33, 10, 29).LineTo(3, 34).LineTo(7, 22).
				CubeTo(5, 16, 6, 11, 11, 7).Close()
			p.FillPath(&bird, Hex("#1E211F"))
			var beak Path
			beak.MoveTo(24, 16).LineTo(31, 18).LineTo(25, 20).Close()
			p.FillPath(&beak, Hex("#1E211F"))
			var featherCut Path
			featherCut.MoveTo(12, 17).LineTo(11, 18).LineTo(16, 29).LineTo(18, 28).Close()
			p.FillPath(&featherCut, Hex("#F5F3EE"))
			var eye Path
			eye.Circle(24.2, 12, 1.25)
			p.FillPath(&eye, Hex("#F5F3EE"))
		})
	case svgInsect0, svgInsect1, svgInsect2, svgInsect3:
		p.Transform(Scaling(r.W/32, r.H/32).Then(Translation(r.X, r.Y)), 0, 0, func() { drawInsect(p, int(s.kind-svgInsect0)) })
	default:
		// Unknown auxiliary vectors use a stable, quiet placeholder.
		colors := []Color{Hex("#8AA66C"), Hex("#BF7657"), Hex("#D3B477"), Hex("#88B7A0")}
		base := colors[int(s.seed)%len(colors)]
		p.fillRound(Rect{X: r.X + r.W*.2, Y: r.Y + r.H*.26, W: r.W * .6, H: r.H * .48}, r.W*.22, base)
		p.fillRound(Rect{X: r.X + r.W*.38, Y: r.Y + r.H*.10, W: r.W * .24, H: r.H * .28}, r.W*.12, base.Mix(Hex("#FFFFFF"), .18))
	}
}

func drawInsect(p *Painter, kind int) {
	green, light, dark := Hex("#8AA66C"), Hex("#B4C88C"), Hex("#4E6844")
	line := func(c Color, width float32, points ...float32) {
		var path Path
		for i := 0; i+3 < len(points); i += 4 {
			path.MoveTo(points[i], points[i+1]).LineTo(points[i+2], points[i+3])
		}
		p.StrokePath(&path, width, c)
	}
	ellipse := func(cx, cy, rx, ry float32, c Color) {
		const k = float32(.55228475)
		var path Path
		path.MoveTo(cx+rx, cy).
			CubeTo(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry).
			CubeTo(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy).
			CubeTo(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry).
			CubeTo(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy).Close()
		p.FillPath(&path, c)
	}
	switch kind {
	case 0:
		line(dark, 1.5, 6, 22, 5, 25, 11, 22, 11, 25, 17, 19, 18, 22, 24, 14, 26, 16)
		for _, c := range []struct{ x, y, r float32 }{{7, 19, 4}, {13, 18, 4.7}, {19, 16, 4.9}, {24, 12, 5.5}} {
			ellipse(c.x, c.y, c.r, c.r, green.Mix(light, (c.x-7)/18))
		}
		line(dark, 1.5, 22, 7, 21, 4, 28, 7, 26, 8, 11, 11, 13, 8, 15, 14, 2, 16, 1, 26, 3, 30, 6, 28)
		// The tiny eye and mouth are what make this read as the same friendly
		// larva as the original SVG at the small scene scale.
		ellipse(26, 11, 1, 1, Hex("#273C2E"))
		line(dark, 1.2, 26, 15, 28, 14)
	case 1:
		line(Hex("#5B4436"), 1.5, 10, 12, 5, 9, 10, 18, 4, 19, 12, 24, 7, 28, 25, 15, 30, 12, 25, 21, 31, 22, 23, 25, 28, 29, 13, 7, 11, 4, 21, 6, 23, 3, 17, 3, 15, 1, 25, 1, 27, 9, 17, 9, 19)
		var body Path
		body.MoveTo(16, 9).CubeTo(8, 9, 7, 25, 16, 28).CubeTo(25, 25, 24, 9, 16, 9).Close()
		p.FillPath(&body, Hex("#BF7657"))
		ellipse(16, 8, 5, 4, Hex("#4E4537"))
		ellipse(13, 7, .8, .8, Hex("#F4E9D4"))
		ellipse(19, 7, .8, .8, Hex("#F4E9D4"))
		line(Hex("#79523C"), 1.4, 16, 9, 16, 27)
		ellipse(11.5, 17, 1.7, 1.7, Hex("#875038"))
		ellipse(20.5, 21, 1.8, 1.8, Hex("#875038"))
	case 2:
		var worm Path
		worm.MoveTo(5, 23).CubeTo(12, 26, 10, 14, 16, 17).CubeTo(22, 20, 17, 9, 24, 6)
		p.StrokePath(&worm, 8, Hex("#AA7D48"))
		p.StrokePath(&worm, 5, Hex("#D3B477"))
		line(Hex("#AA7D48"), 1.2, 8, 22, 11, 24, 13, 15, 15, 19, 19, 12, 22, 13)
		ellipse(25, 6, 4, 4, Hex("#D3B477"))
		ellipse(26, 5, .9, .9, Hex("#4B4932"))
		line(Hex("#8B754E"), 1.2, 24, 2, 23, 0, 29, 3, 31, 1)
	case 3:
		var wings Path
		wings.MoveTo(15, 15).CubeTo(3, -1, -2, 15, 13, 20).CubeTo(1, 26, 11, 32, 16, 21).CubeTo(22, 32, 31, 26, 19, 20).CubeTo(34, 15, 29, -1, 17, 15).Close()
		p.FillPath(&wings, Hex("#88B7A0"))
		var body Path
		body.MoveTo(16, 10).CubeTo(13, 14, 13, 25, 16, 27).CubeTo(19, 25, 19, 14, 16, 10).Close()
		p.FillPath(&body, Hex("#4B6755"))
		ellipse(16, 10, 3, 3, Hex("#4B6755"))
		line(Hex("#4B6755"), 1.3, 15, 8, 11, 4, 17, 8, 21, 4)
	}
}

type Painter struct {
	canvas    *ui.Canvas
	transform Transform
}

func NewPainter(canvas *ui.Canvas) *Painter { return &Painter{canvas: canvas, transform: identity()} }
func (p *Painter) mapped(v Point) Point     { return p.transform.point(v) }
func (p *Painter) Transform(t Transform, x, y float32, fn func()) {
	old := p.transform
	// Then composes as next ∘ current. Move the pivot to the origin first,
	// apply the transform, then move it back; reversing these translations
	// makes every rotated feature orbit the wrong point.
	operation := Translation(-x, -y).Then(t).Then(Translation(x, y))
	// The parent transform acts after the local operation. Since Then is
	// defined as next ∘ current, reverse the call here to obtain parent ∘
	// operation, matching the nested painter semantics used by the source
	// renderer this adapter replaces.
	p.transform = operation.Then(old)
	fn()
	p.transform = old
}
func (p *Painter) bounds(rect Rect) ui.Rect {
	points := []Point{{rect.X, rect.Y}, {rect.X + rect.W, rect.Y}, {rect.X, rect.Y + rect.H}, {rect.X + rect.W, rect.Y + rect.H}}
	minX, minY, maxX, maxY := float32(1e9), float32(1e9), float32(-1e9), float32(-1e9)
	for _, point := range points {
		point = p.mapped(point)
		minX, minY = min(minX, point.X), min(minY, point.Y)
		maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
	}
	return ui.Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}
func (p *Painter) Fill(r Rect, c Color, radius float32) {
	p.canvas.Rounded(p.bounds(r), radius, toUI(c))
}
func (p *Painter) fillRound(r Rect, radius float32, c Color) { p.Fill(r, c, radius) }
func (p *Painter) FillGradient(r Rect, g LinearGradient, radius float32) {
	b := p.bounds(r)
	if radius <= 0 {
		p.canvas.FillLinearGradient(b, gradient(g, b))
		return
	}
	path := roundedRectPath(r, radius)
	p.FillPathGradient(&path, g)
}
func (p *Painter) Shadow(r Rect, radius, ox, oy, blur, spread float32, c Color) {
	b := p.bounds(r)
	p.canvas.Shadow(b, radius, ui.ShadowStyle{OffsetX: ox, OffsetY: oy, Blur: blur, Spread: spread}, toUI(c))
}
func (p *Painter) Clip(r Rect, radius float32, fn func()) {
	p.canvas.Clip(p.bounds(r))
	fn()
	p.canvas.Unclip()
}
func (p *Painter) Layer(layer uint16, fn func()) {
	p.canvas.PushLayer(layer)
	defer p.canvas.PopLayer()
	fn()
}
func (p *Painter) Image(source ImageSource, r Rect, _ Fit) {
	if source != nil {
		source.draw(p, r)
	}
}

func pathData(path *Path, t Transform) (ui.Path, ui.Rect) {
	if path == nil || len(path.commands) == 0 {
		return ui.Path{}, ui.Rect{}
	}
	minX, minY, maxX, maxY := float32(1e9), float32(1e9), float32(-1e9), float32(-1e9)
	add := func(q Point) {
		q = t.point(q)
		minX, minY = min(minX, q.X), min(minY, q.Y)
		maxX, maxY = max(maxX, q.X), max(maxY, q.Y)
	}
	for _, c := range path.commands {
		add(c.p)
		switch c.kind {
		case 'Q':
			add(c.c1)
		case 'C':
			add(c.c1)
			add(c.c2)
		}
	}
	w, h := maxX-minX, maxY-minY
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	commands := make([]ui.PathCommand, 0, len(path.commands))
	for _, c := range path.commands {
		q := t.point(c.p)
		q.X -= minX
		q.Y -= minY
		switch c.kind {
		case 'M':
			commands = append(commands, ui.MoveTo(q.X, q.Y))
		case 'L':
			commands = append(commands, ui.LineTo(q.X, q.Y))
		case 'Q':
			ctl := t.point(c.c1)
			ctl.X -= minX
			ctl.Y -= minY
			commands = append(commands, ui.QuadTo(ctl.X, ctl.Y, q.X, q.Y))
		case 'C':
			c1, c2 := t.point(c.c1), t.point(c.c2)
			c1.X -= minX
			c1.Y -= minY
			c2.X -= minX
			c2.Y -= minY
			commands = append(commands, ui.CubicTo(c1.X, c1.Y, c2.X, c2.Y, q.X, q.Y))
		case 'Z':
			commands = append(commands, ui.ClosePath())
		}
	}
	return ui.NewPath(w, h, commands...), ui.Rect{X: minX, Y: minY, Width: w, Height: h}
}

func roundedRectPath(r Rect, radius float32) Path {
	radius = min(radius, min(r.W, r.H)/2)
	const k = float32(.55228475)
	var path Path
	path.MoveTo(r.X+radius, r.Y).
		LineTo(r.X+r.W-radius, r.Y).
		CubeTo(r.X+r.W-radius+k*radius, r.Y, r.X+r.W, r.Y+radius-k*radius, r.X+r.W, r.Y+radius).
		LineTo(r.X+r.W, r.Y+r.H-radius).
		CubeTo(r.X+r.W, r.Y+r.H-radius+k*radius, r.X+r.W-radius+k*radius, r.Y+r.H, r.X+r.W-radius, r.Y+r.H).
		LineTo(r.X+radius, r.Y+r.H).
		CubeTo(r.X+radius-k*radius, r.Y+r.H, r.X, r.Y+r.H-radius+k*radius, r.X, r.Y+r.H-radius).
		LineTo(r.X, r.Y+radius).
		CubeTo(r.X, r.Y+radius-k*radius, r.X+radius-k*radius, r.Y, r.X+radius, r.Y).
		Close()
	return path
}
func gradient(g LinearGradient, b ui.Rect) ui.LinearGradient {
	from, to := toUI(g.From), toUI(g.To)
	if len(g.Stops) > 0 {
		from, to = toUI(g.Stops[0].Color), toUI(g.Stops[len(g.Stops)-1].Color)
	}
	angle := float64(g.Angle) * math.Pi / 180
	dx, dy := float32(math.Cos(angle)), float32(math.Sin(angle))
	half := float32(math.Sqrt(float64(b.Width*b.Width+b.Height*b.Height)) / 2)
	c := ui.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}
	return ui.LinearGradient{Start: ui.Point{X: c.X - dx*half, Y: c.Y - dy*half}, End: ui.Point{X: c.X + dx*half, Y: c.Y + dy*half}, From: from, To: to}
}
func (p *Painter) FillPath(path *Path, c Color) {
	q, b := pathData(path, p.transform)
	p.canvas.DrawPath(q, b, ui.PathStyle{Fill: toUI(c)})
}
func (p *Painter) FillPathGradient(path *Path, g LinearGradient) {
	q, b := pathData(path, p.transform)
	gg := gradient(g, b)
	p.canvas.DrawPath(q, b, ui.PathStyle{FillGradient: &gg})
}
func (p *Painter) StrokePath(path *Path, width float32, c Color) {
	q, b := pathData(path, p.transform)
	p.canvas.DrawPath(q, b, ui.PathStyle{Stroke: toUI(c), StrokeWidth: width, Cap: ui.CapRound, Join: ui.JoinRound})
}

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
