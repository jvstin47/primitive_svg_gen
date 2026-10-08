package primitive

import (
	"fmt"
	"math"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/raster"
)

type Line struct {
	Worker *Worker
	X1, Y1 float64
	X2, Y2 float64
	Width  float64
}

func NewRandomLine(worker *Worker) *Line {
	rnd := worker.Rnd
	x1 := rnd.Float64() * float64(worker.W)
	y1 := rnd.Float64() * float64(worker.H)
	x2 := x1 + rnd.Float64()*60 - 30
	y2 := y1 + rnd.Float64()*60 - 30
	width := 1.5
	l := &Line{worker, x1, y1, x2, y2, width}
	l.Mutate()
	return l
}

func (l *Line) Draw(dc *gg.Context, scale float64) {
	dc.MoveTo(l.X1, l.Y1)
	dc.LineTo(l.X2, l.Y2)
	dc.SetLineWidth(l.Width * scale)
	dc.SetLineCap(gg.LineCapRound)
	dc.Stroke()
}

func (l *Line) SVG(attrs string) string {
	attrs = strings.Replace(attrs, "fill", "stroke", -1)
	return fmt.Sprintf(
		"<line %s stroke-linecap=\"round\" stroke-width=\"%.2f\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" />",
		attrs, l.Width, l.X1, l.Y1, l.X2, l.Y2)
}

func (l *Line) Copy() Shape {
	a := *l
	return &a
}

func (l *Line) Mutate() {
	const m = 16
	w := l.Worker.W
	h := l.Worker.H
	rnd := l.Worker.Rnd
	for {
		switch rnd.Intn(3) {
		case 0:
			l.X1 = clamp(l.X1+rnd.NormFloat64()*16, -m, float64(w-1+m))
			l.Y1 = clamp(l.Y1+rnd.NormFloat64()*16, -m, float64(h-1+m))
		case 1:
			l.X2 = clamp(l.X2+rnd.NormFloat64()*16, -m, float64(w-1+m))
			l.Y2 = clamp(l.Y2+rnd.NormFloat64()*16, -m, float64(h-1+m))
		case 2:
			l.Width = clamp(l.Width+rnd.NormFloat64()*0.5, 0.5, 12)
		}
		if l.Valid() {
			break
		}
	}
}

func (l *Line) Valid() bool {
	dx := l.X1 - l.X2
	dy := l.Y1 - l.Y2
	d := math.Sqrt(dx*dx + dy*dy)
	return d >= 3.0
}

func (l *Line) Rasterize() []Scanline {
	var path raster.Path
	p1 := fixp(l.X1, l.Y1)
	p2 := fixp(l.X2, l.Y2)
	path.Start(p1)
	path.Add1(p2)
	width := fix(l.Width)
	return strokePath(l.Worker, path, width, raster.RoundCapper, raster.RoundJoiner)
}
