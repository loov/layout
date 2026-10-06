package layout

import (
	"math"
	"slices"

	"github.com/loov/layout/internal/draw"
)

// flattenPath approximates the rounded corners drawn by the writers
// (quadratic curves of the given radius) with two extra points per corner.
func flattenPath(path []Vector, radius, maxDeviation Length) []Vector {
	return appendFlatPath(nil, path, radius, maxDeviation)
}

// flatLen returns the length of the path flattenPath returns
func flatLen(path []Vector) int {
	if len(path) < 3 {
		return len(path)
	}
	return 3*len(path) - 4
}

// appendFlatPath appends the flattened path to out, see flattenPath; a
// path without corners is returned as it is
func appendFlatPath(out, path []Vector, radius, maxDeviation Length) []Vector {
	if len(path) < 3 {
		return path
	}
	length := func(a, b Vector) Length {
		return Length(math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y)))
	}
	towards := func(a, b Vector, d Length) Vector {
		l := length(a, b)
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

// offsetPath shifts the path sideways by dx and re-clips the ends to the nodes
func offsetPath(path []Vector, dx Length, from, to *lnode) []Vector {
	// a straight edge between neighbors along a rank runs across x, so
	// it moves across that, in y
	along := len(path) == 2 && from.Center.Y == to.Center.Y
	out := make([]Vector, len(path))
	for i, p := range path {
		out[i] = Vector{X: p.X + dx, Y: p.Y}
		if along {
			out[i] = Vector{X: p.X, Y: p.Y + dx}
		}
	}
	if len(out) >= 2 {
		out[0] = from.Boundary(out[1])
		out[len(out)-1] = to.Boundary(out[len(out)-2])
	}
	return out
}

// reversePath returns the path in reverse order
func reversePath(path []Vector) []Vector {
	rs := make([]Vector, 0, len(path))
	for _, p := range slices.Backward(path) {
		rs = append(rs, p)
	}
	return rs
}

// nearestOnPath returns the point of the polyline closest to p
func nearestOnPath(path []Vector, p Vector) (Vector, bool) {
	best, bestDist := Vector{}, math.Inf(1)
	for i := 0; i+1 < len(path); i++ {
		a, b := path[i], path[i+1]
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, (float64(p.X-a.X)*dx+float64(p.Y-a.Y)*dy)/l))
		}
		q := Vector{a.X + Length(t*dx), a.Y + Length(t*dy)}
		if d := math.Hypot(float64(p.X-q.X), float64(p.Y-q.Y)); d < bestDist {
			best, bestDist = q, d
		}
	}
	return best, bestDist < math.Inf(1)
}
