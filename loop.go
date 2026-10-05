package layout

import (
	"cmp"
	"math"
	"slices"
)

// loopCount numbers the self-loops without ports of each node, which
// stack down its side
type loopCount struct {
	count, index map[*lnode]int
}

func countLoops(edges []*ledge) *loopCount {
	loops := &loopCount{count: map[*lnode]int{}, index: map[*lnode]int{}}
	for _, edge := range edges {
		if edge.From == edge.To && edge.FromPort == CompassAuto && edge.ToPort == CompassAuto {
			loops.count[edge.From]++
		}
	}
	return loops
}

// next returns the place of the self-loop edge among its node's loops
func (loops *loopCount) next(edge *ledge) int {
	if edge.FromPort != CompassAuto || edge.ToPort != CompassAuto {
		return 0
	}
	k := loops.index[edge.From]
	loops.index[edge.From]++
	return k
}

func loopPath(edge *ledge, width, height Length, k, n int) []Vector {
	node := edge.From
	right := node.Right() + width
	// the n loops without ports stack down the right side, loop k in the
	// k-th share of it; a single loop spans the middle half
	share := 2 * node.Radius.Y / Length(max(n, 1))
	top := node.Center.Y - node.Radius.Y + Length(k)*share
	up := Vector{X: node.Right(), Y: top + share/4}
	down := Vector{X: node.Right(), Y: top + 3*share/4}
	if edge.FromPort == CompassAuto && edge.ToPort == CompassAuto {
		// leave and return horizontally from where the outline is
		from, to := node.Boundary(up), node.Boundary(down)
		return []Vector{from, {X: right, Y: from.Y}, {X: right, Y: to.Y}, to}
	}

	// With ports, leave each end straight out to the node box grown by
	// width and height, then go around it on the shorter side.
	from, to := node.Boundary(up), node.Boundary(down)
	if edge.FromPort != CompassAuto {
		from = node.CompassPoint(edge.FromPort)
	}
	if edge.ToPort != CompassAuto {
		to = node.CompassPoint(edge.ToPort)
	}
	outward := func(p Vector) Vector {
		d := Vector{X: 1} // the center port leaves to the right
		if p != node.Center {
			v := p.Sub(node.Center)
			n := Length(math.Hypot(float64(v.X), float64(v.Y)))
			d = Vector{X: v.X / n, Y: v.Y / n}
		}
		// where the ray along d leaves the grown box
		t := Length(math.Inf(1))
		if d.X != 0 {
			t = (node.Radius.X + width) / Length(math.Abs(float64(d.X)))
		}
		if d.Y != 0 {
			t = min(t, (node.Radius.Y+height)/Length(math.Abs(float64(d.Y))))
		}
		return node.Center.Add(Vector{X: d.X * t, Y: d.Y * t})
	}
	a, b := outward(from), outward(to)
	if from == to {
		// both ends on one port: split the loop sideways
		d := a.Sub(from)
		side := Vector{X: -d.Y / 2, Y: d.X / 2}
		return []Vector{from, a.Add(side), a.Sub(side), to}
	}
	angle := func(p Vector) float64 {
		return math.Atan2(float64(p.Y-node.Center.Y), float64(p.X-node.Center.X))
	}
	start := angle(a)
	sweep := math.Remainder(angle(b)-start, 2*math.Pi)
	type corner struct {
		at float64
		p  Vector
	}
	var corners []corner
	for _, p := range []Vector{
		{X: node.Left() - width, Y: node.Top() - height},
		{X: node.Right() + width, Y: node.Top() - height},
		{X: node.Right() + width, Y: node.Bottom() + height},
		{X: node.Left() - width, Y: node.Bottom() + height},
	} {
		at := math.Remainder(angle(p)-start, 2*math.Pi)
		if (sweep > 0 && at > 0 && at < sweep) || (sweep < 0 && at < 0 && at > sweep) {
			corners = append(corners, corner{math.Abs(at), p})
		}
	}
	slices.SortFunc(corners, func(x, y corner) int { return cmp.Compare(x.at, y.at) })
	path := []Vector{from, a}
	for _, c := range corners {
		path = append(path, c.p)
	}
	return append(path, b, to)
}
