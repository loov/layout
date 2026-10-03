package layout

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Diagnostics counts layout defects that are otherwise only visible by eye.
// Edge paths are measured as drawn, with rounded corners flattened.
type Diagnostics struct {
	// NodeOverlaps counts pairs of node boxes that intersect.
	NodeOverlaps int
	// EdgeThroughNode counts edges passing through a node other than
	// their own ends, once per edge and node.
	EdgeThroughNode int
	// EdgeCrossings counts pairs of edge segments that cross.
	EdgeCrossings int
	// EdgeOverlaps counts pairs of edges running closer than a stroke
	// width for more than an arrowhead length, which draw as one line.
	EdgeOverlaps int
	// ParallelEdges counts pairs of edges running within an arrowhead
	// width of each other for more than four arrowhead lengths, which read
	// as a bundle.
	ParallelEdges int
	// Shafts counts edge ends whose shaft passes through their own node
	// before reaching the outline, so the arrowhead draws over the node.
	Shafts int
	// EndOverlaps counts pairs of edge ends (starts or ends) closer than an
	// arrowhead, where arrowheads draw on top of each other.
	EndOverlaps int
	// JaggedEdges counts edges whose path winds by more than 90 degrees
	// net: hooks and U-turns (an S-bend nets out to zero, see BendyEdges).
	JaggedEdges int
	// BendyEdges counts edges with more than three bends.
	BendyEdges int
	// WavyEdges counts edges whose turns change direction at least twice
	// (left, right, left), which reads as a wobble.
	WavyEdges int
	// EdgeNearNode counts edges passing within EdgePadding of a node
	// they don't end at, once per edge and node.
	EdgeNearNode int
	// ShallowCrossings counts crossings at less than 30 degrees, which read
	// as merging lines.
	ShallowCrossings int
	// BackEdges counts edges drawn against the rank direction.
	BackEdges int
	// EdgeLength is the total length of all edge paths, in points.
	EdgeLength Length
	// Area is the width times height of the drawing, in points.
	Area Length
	// FarLabels counts labels further from their own edge than the text
	// height, which makes them hard to attribute.
	FarLabels int
	// LabelOverlaps counts labels that intersect a node, an edge segment or
	// another label.
	LabelOverlaps int

	// Details names every counted finding, one per line.
	Details []string
}

// String formats the diagnostics as one line of key=value pairs.
func (diag Diagnostics) String() string {
	return fmt.Sprintf("nodes=%d through=%d near=%d crossings=%d shallow=%d overlaps=%d parallel=%d ends=%d shafts=%d jagged=%d bends=%d wavy=%d back=%d labels=%d far=%d length=%.0f area=%.0f",
		diag.NodeOverlaps, diag.EdgeThroughNode, diag.EdgeNearNode, diag.EdgeCrossings, diag.ShallowCrossings, diag.EdgeOverlaps, diag.ParallelEdges, diag.EndOverlaps, diag.Shafts,
		diag.JaggedEdges, diag.BendyEdges, diag.WavyEdges, diag.BackEdges, diag.LabelOverlaps, diag.FarLabels, diag.EdgeLength, diag.Area)
}

// Diagnose computes Diagnostics for a layout.
func Diagnose(l *Layout) Diagnostics {
	graph := l.work()
	var diag Diagnostics
	const eps = 1e-3

	type segment struct {
		a, b Vector
		edge *ledge
	}
	var segments []segment
	for _, edge := range graph.Edges {
		path := edge.Path
		if graph.Splines == SplinesRounded {
			path = flattenPath(path, 2*graph.RowPadding, graph.EdgePadding)
		}
		for i := 0; i+1 < len(path); i++ {
			segments = append(segments, segment{path[i], path[i+1], edge})
		}
	}

	boxes := func(node *lnode) (tl, br Vector) {
		return Vector{node.Left(), node.Top()}, Vector{node.Right(), node.Bottom()}
	}
	overlap := func(tl1, br1, tl2, br2 Vector) bool {
		return tl1.X < br2.X-eps && tl2.X < br1.X-eps && tl1.Y < br2.Y-eps && tl2.Y < br1.Y-eps
	}

	for i, a := range graph.Nodes {
		for _, b := range graph.Nodes[i+1:] {
			tl1, br1 := boxes(a)
			tl2, br2 := boxes(b)
			if overlap(tl1, br1, tl2, br2) {
				diag.NodeOverlaps++
				diag.Details = append(diag.Details, fmt.Sprintf("nodes %v and %v overlap", a, b))
			}
		}
	}

	type edgeNode struct {
		edge *ledge
		node *lnode
	}
	through := map[edgeNode]bool{}
	near := map[edgeNode]bool{}
	for _, s := range segments {
		for _, node := range graph.Nodes {
			if node == s.edge.From || node == s.edge.To {
				continue
			}
			key := edgeNode{s.edge, node}
			if !through[key] && segmentHitsNode(s.a, s.b, node, -eps) {
				through[key] = true
				diag.EdgeThroughNode++
				diag.Details = append(diag.Details, fmt.Sprintf("edge %v through node %v at %v-%v", s.edge, node, s.a, s.b))
			} else if !through[key] && !near[key] && segmentHitsNode(s.a, s.b, node, graph.EdgePadding) {
				near[key] = true
				diag.EdgeNearNode++
				diag.Details = append(diag.Details, fmt.Sprintf("edge %v near node %v at %v-%v", s.edge, node, s.a, s.b))
			}
		}
	}

	// segments closer than a stroke width over more than an arrowhead
	// length draw as one line
	const stroke, arrow = 1.5 * Point, 6 * Point
	// alongside reports whether q runs within dist of p's line for more
	// than minLen
	alongside := func(p, q segment, dist, minLen Length) bool {
		d := func(a, b, c Vector) float64 {
			return float64((b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X))
		}
		dx, dy := float64(p.b.X-p.a.X), float64(p.b.Y-p.a.Y)
		length := math.Hypot(dx, dy)
		if length < eps {
			return false
		}
		if math.Abs(d(p.a, p.b, q.a))/length >= float64(dist) || math.Abs(d(p.a, p.b, q.b))/length >= float64(dist) {
			return false
		}
		proj := func(v Vector) float64 { return (float64(v.X-p.a.X)*dx + float64(v.Y-p.a.Y)*dy) / length }
		lo, hi := math.Min(proj(q.a), proj(q.b)), math.Max(proj(q.a), proj(q.b))
		return math.Min(hi, length)-math.Max(lo, 0) > float64(minLen)
	}
	cross := func(p, q segment) (crosses, collinear bool) {
		d := func(a, b, c Vector) float64 {
			return float64((b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X))
		}
		dx, dy := float64(p.b.X-p.a.X), float64(p.b.Y-p.a.Y)
		length := math.Hypot(dx, dy)
		if length < eps {
			return false, false
		}
		d1, d2 := d(p.a, p.b, q.a), d(p.a, p.b, q.b)
		d3, d4 := d(q.a, q.b, p.a), d(q.a, q.b, p.b)
		// distance of q's ends from p's line
		if math.Abs(d1)/length < float64(stroke) && math.Abs(d2)/length < float64(stroke) {
			proj := func(v Vector) float64 { return (float64(v.X-p.a.X)*dx + float64(v.Y-p.a.Y)*dy) / length }
			lo, hi := math.Min(proj(q.a), proj(q.b)), math.Max(proj(q.a), proj(q.b))
			return false, math.Min(hi, length)-math.Max(lo, 0) > float64(arrow)
		}
		// a proper crossing has each segment's ends on opposite sides of
		// the other; touching at an end does not count
		return d1*d2 < -eps && d3*d4 < -eps, false
	}
	type edgePair struct{ a, b *ledge }
	overlaps := map[edgePair]bool{}
	parallel := map[edgePair]bool{}
	for i, p := range segments {
		for _, q := range segments[i+1:] {
			if p.edge == q.edge {
				continue
			}
			if pair := (edgePair{p.edge, q.edge}); !parallel[pair] && alongside(p, q, arrow+stroke, 4*arrow) {
				parallel[pair] = true
				diag.ParallelEdges++
				diag.Details = append(diag.Details, fmt.Sprintf("edges %v and %v run alongside at %v-%v", p.edge, q.edge, p.a, p.b))
			}
			crosses, collinear := cross(p, q)
			if crosses {
				diag.EdgeCrossings++
				// angle between the segments
				ux, uy := float64(p.b.X-p.a.X), float64(p.b.Y-p.a.Y)
				vx, vy := float64(q.b.X-q.a.X), float64(q.b.Y-q.a.Y)
				angle := math.Abs(math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy))
				angle = math.Min(angle, math.Pi-angle)
				if angle < 30*math.Pi/180 {
					diag.ShallowCrossings++
					diag.Details = append(diag.Details, fmt.Sprintf("edges %v and %v cross at %.0f degrees", p.edge, q.edge, angle*180/math.Pi))
				}
			}
			if collinear && !overlaps[edgePair{p.edge, q.edge}] {
				overlaps[edgePair{p.edge, q.edge}] = true
				diag.EdgeOverlaps++
				diag.Details = append(diag.Details, fmt.Sprintf("edges %v and %v overlap at %v-%v", p.edge, q.edge, p.a, p.b))
			}
		}
	}

	// a shaft that enters its own node before the tip draws the arrowhead
	// over the node; test the segment before the tip, trimmed at the tip
	for _, edge := range graph.Edges {
		if edge.From == edge.To {
			continue
		}
		path := edge.Path
		if graph.Splines == SplinesRounded {
			path = flattenPath(path, 2*graph.RowPadding, graph.EdgePadding)
		}
		if len(path) < 2 {
			continue
		}
		trim := func(inner, tip Vector) (Vector, Vector) {
			return inner, Vector{tip.X + (inner.X-tip.X)*0.05, tip.Y + (inner.Y-tip.Y)*0.05}
		}
		// the tip itself inside the node counts too
		inside := func(p Vector, node *lnode) bool { return segmentHitsNode(p, p, node, -0.5) }
		if a, b := trim(path[1], path[0]); segmentHitsNode(a, b, edge.From, -eps) || inside(path[0], edge.From) {
			diag.Shafts++
			diag.Details = append(diag.Details, fmt.Sprintf("edge %v starts inside node %v", edge, edge.From))
		}
		if a, b := trim(path[len(path)-2], path[len(path)-1]); segmentHitsNode(a, b, edge.To, -eps) || inside(path[len(path)-1], edge.To) {
			diag.Shafts++
			diag.Details = append(diag.Details, fmt.Sprintf("edge %v ends inside node %v", edge, edge.To))
		}
	}

	// ends closer than an arrowhead stack their arrowheads
	type endpoint struct {
		edge *ledge
		p    Vector
	}
	var ends []endpoint
	for _, edge := range graph.Edges {
		if len(edge.Path) > 0 {
			ends = append(ends, endpoint{edge, edge.Path[0]}, endpoint{edge, edge.Path[len(edge.Path)-1]})
		}
	}
	for i, a := range ends {
		for _, b := range ends[i+1:] {
			if a.edge != b.edge && math.Hypot(float64(a.p.X-b.p.X), float64(a.p.Y-b.p.Y)) < float64(arrow) {
				diag.EndOverlaps++
				diag.Details = append(diag.Details, fmt.Sprintf("ends of %v and %v overlap at %v", a.edge, b.edge, a.p))
			}
		}
	}

	// rank direction as a unit vector
	flow := Vector{0, 1}
	switch graph.RankDir {
	case LeftToRight:
		flow = Vector{1, 0}
	case RightToLeft:
		flow = Vector{-1, 0}
	case BottomToTop:
		flow = Vector{0, -1}
	}
	for _, edge := range graph.Edges {
		for i := 0; i+1 < len(edge.Path); i++ {
			d := edge.Path[i+1].Sub(edge.Path[i])
			diag.EdgeLength += Length(math.Hypot(float64(d.X), float64(d.Y)))
		}
		if edge.From == edge.To {
			continue
		}
		if d := edge.To.Center.Sub(edge.From.Center); d.X*flow.X+d.Y*flow.Y < 0 {
			diag.BackEdges++
			diag.Details = append(diag.Details, fmt.Sprintf("edge %v points against the rank direction", edge))
		}
		turn, bends, flips, last := 0.0, 0, 0, 0.0
		path := edge.Path
		for i := 1; i+1 < len(path); i++ {
			a, b := path[i].Sub(path[i-1]), path[i+1].Sub(path[i])
			if a == (Vector{}) || b == (Vector{}) {
				continue
			}
			angle := math.Atan2(float64(a.X*b.Y-a.Y*b.X), float64(a.X*b.X+a.Y*b.Y))
			turn += angle
			if math.Abs(angle) > 5*math.Pi/180 {
				bends++
				if last != 0 && (angle > 0) != (last > 0) {
					flips++
				}
				last = angle
			}
		}
		if flips >= 2 {
			diag.WavyEdges++
			diag.Details = append(diag.Details, fmt.Sprintf("edge %v changes turn direction %d times", edge, flips))
		}
		if math.Abs(turn) > math.Pi/2 {
			diag.JaggedEdges++
			diag.Details = append(diag.Details, fmt.Sprintf("edge %v winds %.0f degrees", edge, math.Abs(turn)*180/math.Pi))
		}
		if bends > 3 {
			diag.BendyEdges++
			diag.Details = append(diag.Details, fmt.Sprintf("edge %v has %d bends", edge, bends))
		}
	}

	tl, br := graph.Bounds()
	diag.Area = (br.X - tl.X) * (br.Y - tl.Y)

	// distance from a label box to its own path
	for _, edge := range graph.Edges {
		if edge.Label == "" || len(edge.Path) < 2 {
			continue
		}
		path := edge.Path
		if graph.Splines == SplinesRounded {
			path = flattenPath(path, 2*graph.RowPadding, graph.EdgePadding)
		}
		ltl := edge.LabelPos.Sub(edge.LabelRadius)
		lbr := edge.LabelPos.Add(edge.LabelRadius)
		best := math.Inf(1)
		for i := 0; i+1 < len(path); i++ {
			best = math.Min(best, rectSegmentDistance(ltl, lbr, path[i], path[i+1]))
		}
		if limit := float64(2 * edge.LabelRadius.Y); best > limit {
			diag.FarLabels++
			diag.Details = append(diag.Details, fmt.Sprintf("label %q of %v is %.0f from its edge", edge.Label, edge, best))
		}
	}

	type box struct{ tl, br Vector }
	var labels []box
	var labelEdges []*ledge
	for _, edge := range graph.Edges {
		if edge.Label == "" {
			continue
		}
		labelEdges = append(labelEdges, edge)
		labels = append(labels, box{edge.LabelPos.Add(Vector{-edge.LabelRadius.X, -edge.LabelRadius.Y}), edge.LabelPos.Add(edge.LabelRadius)})
	}
	for i, l := range labels {
		var hits []string
		for _, node := range graph.Nodes {
			tl, br := boxes(node)
			if overlap(l.tl, l.br, tl, br) {
				hits = append(hits, "node "+node.String())
			}
		}
		for _, s := range segments {
			if segmentHitsRect(s.a, s.b, l.tl.Add(Vector{eps, eps}), l.br.Add(Vector{-eps, -eps})) {
				hits = append(hits, "edge "+s.edge.String())
			}
		}
		for k, o := range labels[:i] {
			if overlap(l.tl, l.br, o.tl, o.br) {
				hits = append(hits, "label "+labelEdges[k].Label)
			}
		}
		if len(hits) > 0 {
			diag.LabelOverlaps++
			diag.Details = append(diag.Details, fmt.Sprintf("label %q of %v overlaps %s", labelEdges[i].Label, labelEdges[i], strings.Join(slices.Compact(hits), ", ")))
		}
	}
	return diag
}

// segmentHitsNode reports whether segment ab enters the node's outline
// grown by pad (negative shrinks, so touching does not count).
func segmentHitsNode(a, b Vector, node *lnode, pad Length) bool {
	switch node.Shape {
	case Box, Square, Record, None:
		return segmentHitsRect(a, b, Vector{node.Left() - pad, node.Top() - pad}, Vector{node.Right() + pad, node.Bottom() + pad})
	}
	// ellipse: scale to a unit circle and test the distance from the center;
	// circles are drawn with the larger radius
	rx, ry := float64(node.Radius.X+pad), float64(node.Radius.Y+pad)
	if node.Shape == Circle {
		rx = math.Max(rx, ry)
		ry = rx
	}
	if rx <= 0 || ry <= 0 {
		return false
	}
	ax, ay := float64(a.X-node.Center.X)/rx, float64(a.Y-node.Center.Y)/ry
	bx, by := float64(b.X-node.Center.X)/rx, float64(b.Y-node.Center.Y)/ry
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l))
	}
	px, py := ax+t*dx, ay+t*dy
	return px*px+py*py < 1
}

// rectSegmentDistance returns the distance between the rectangle tl-br and
// segment ab, zero when they touch.
func rectSegmentDistance(tl, br, a, b Vector) float64 {
	if segmentHitsRect(a, b, tl, br) {
		return 0
	}
	pointSegment := func(p, a, b Vector) float64 {
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, (float64(p.X-a.X)*dx+float64(p.Y-a.Y)*dy)/l))
		}
		return math.Hypot(float64(p.X-a.X)-t*dx, float64(p.Y-a.Y)-t*dy)
	}
	pointRect := func(p Vector) float64 {
		dx := math.Max(0, math.Max(float64(tl.X-p.X), float64(p.X-br.X)))
		dy := math.Max(0, math.Max(float64(tl.Y-p.Y), float64(p.Y-br.Y)))
		return math.Hypot(dx, dy)
	}
	best := math.Min(pointRect(a), pointRect(b))
	for _, c := range []Vector{tl, {br.X, tl.Y}, br, {tl.X, br.Y}} {
		best = math.Min(best, pointSegment(c, a, b))
	}
	return best
}
