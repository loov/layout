package layout

import (
	"iter"
	"math"
	"slices"

	"github.com/loov/layout/internal/draw"
)

// flattenPath approximates the rounded corners drawn by the writers
// (quadratic curves of the given radius) with two extra points per corner.
func flattenPath(path Path, radius, maxDeviation Length) Path {
	return appendFlatPath(nil, path, radius, maxDeviation)
}

// flatLen returns the length of the path flattenPath returns
func flatLen(path Path) int {
	if len(path) < 3 {
		return len(path)
	}
	return 3*len(path) - 4
}

// appendFlatPath appends the flattened path to out, see flattenPath; a
// path without corners is returned as it is
func appendFlatPath(out, path Path, radius, maxDeviation Length) Path {
	if len(path) < 3 {
		return path
	}
	towards := func(a, b Vector, d Length) Vector {
		l := b.Sub(a).Len()
		if l == 0 {
			return a
		}
		return Vector{X: a.X + (b.X-a.X)*d/l, Y: a.Y + (b.Y-a.Y)*d/l}
	}
	out = append(out, path[0])
	for i := 1; i+1 < len(path); i++ {
		prev, p, next := path[i-1], path[i], path[i+1]
		r := Length(draw.CornerRadius(drawPoint(prev), drawPoint(p), drawPoint(next), float64(radius), float64(maxDeviation)))
		in, exit := towards(p, prev, r), towards(p, next, r)
		mid := Vector{X: (in.X + 2*p.X + exit.X) / 4, Y: (in.Y + 2*p.Y + exit.Y) / 4}
		out = append(out, in, mid, exit)
	}
	return append(out, path[len(path)-1])
}

// drawPoint converts v for the draw package
func drawPoint(v Vector) draw.Point { return draw.Point{X: float64(v.X), Y: float64(v.Y)} }

// offsetPath shifts the path sideways by dx and re-clips the ends to the
// nodes; a straight edge between neighbors along a rank, along, runs
// across x, so it moves across that, in y
func offsetPath(path Path, dx Length, from, to *lnode, along bool) Path {
	out := make(Path, len(path))
	for i, p := range path {
		out[i] = Vector{X: p.X + dx, Y: p.Y}
		if along {
			out[i] = Vector{X: p.X, Y: p.Y + dx}
		}
	}
	switch {
	case along:
		// straight across, at its own height on both
		out[0] = from.sideAt(out[0].Y, to.Center.X)
		out[1] = to.sideAt(out[1].Y, from.Center.X)
	case len(out) >= 2:
		out[0] = from.Boundary(out[1])
		out[len(out)-1] = to.Boundary(out[len(out)-2])
	}
	return out
}

// reversed returns the path in reverse order
func (path Path) reversed() Path {
	rs := make(Path, 0, len(path))
	for _, p := range slices.Backward(path) {
		rs = append(rs, p)
	}
	return rs
}

// Path is a polyline, from its first point to its last.
type Path []Vector

// Segments yields the ends of every segment of path, in order.
func (path Path) Segments() iter.Seq2[Vector, Vector] {
	return func(yield func(a, b Vector) bool) {
		for i := 0; i+1 < len(path); i++ {
			if !yield(path[i], path[i+1]) {
				return
			}
		}
	}
}

// Length returns the length of path along its segments.
func (path Path) Length() (length Length) {
	for a, b := range path.Segments() {
		length += b.Sub(a).Len()
	}
	return length
}

// nearest returns the point of path closest to p
func (path Path) nearest(p Vector) (Vector, bool) {
	best, bestDist := Vector{}, math.Inf(1)
	for a, b := range path.Segments() {
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, (float64(p.X-a.X)*dx+float64(p.Y-a.Y)*dy)/l))
		}
		q := Vector{a.X + Length(t*dx), a.Y + Length(t*dy)}
		if d := float64(p.Sub(q).Len()); d < bestDist {
			best, bestDist = q, d
		}
	}
	return best, bestDist < math.Inf(1)
}
