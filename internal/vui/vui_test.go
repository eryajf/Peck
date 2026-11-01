package vui

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestTransformUsesTheRequestedPivot(t *testing.T) {
	p := &Painter{transform: identity()}
	var got Point
	p.Transform(Rotation(90), 10, 10, func() { got = p.mapped(Point{X: 10, Y: 5}) })
	if got.X < 14.99 || got.X > 15.01 || got.Y < 9.99 || got.Y > 10.01 {
		t.Fatalf("rotation moved around the wrong point: got (%v, %v)", got.X, got.Y)
	}
}

func TestTransformRestoresTheParentAfterPainting(t *testing.T) {
	p := &Painter{transform: Translation(4, 7)}
	want := p.transform
	p.Transform(Scaling(2, 3), 10, 10, func() {})
	if p.transform != want {
		t.Fatalf("transform leaked outside nested paint: got %#v want %#v", p.transform, want)
	}
}

func TestTransformedIconKeepsItsRotationPivot(t *testing.T) {
	icon := ui.Icon{
		ViewBox: 10,
		Layers: []ui.IconLayer{{
			Fill: ui.Color{Red: 1, Alpha: 1},
			Commands: []ui.PathCommand{
				ui.MoveTo(0, 0), ui.LineTo(10, 0), ui.LineTo(0, 10), ui.ClosePath(),
			},
		}},
	}
	canvas := &ui.Canvas{}
	painter := NewPainter(canvas)
	source := &IconSource{icon: icon, tint: ui.Color{Alpha: 1}}
	painter.Transform(Rotation(45), 5, 5, func() {
		source.draw(painter, Rect{W: 10, H: 10})
	})
	if len(canvas.Triangles()) == 0 {
		t.Fatal("transformed icon did not produce vector geometry")
	}
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	for _, triangle := range canvas.Triangles() {
		for _, point := range triangle.Points {
			minX, minY = min(minX, point.X), min(minY, point.Y)
			maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
		}
	}
	// The transformed triangle has a 10√2 horizontal envelope and a 5√2
	// vertical envelope. The old DrawIcon path instead re-fitted it into the
	// rotated square bounds, which made the mark visibly jump between frames.
	wantWidth, wantHeight := float32(10*1.41421356), float32(5*1.41421356)
	if maxX-minX < wantWidth-.15 || maxX-minX > wantWidth+.15 || maxY-minY < wantHeight-.15 || maxY-minY > wantHeight+.15 {
		t.Fatalf("rotated icon was refitted: bounds %.3fx%.3f, want %.3fx%.3f", maxX-minX, maxY-minY, wantWidth, wantHeight)
	}
}
