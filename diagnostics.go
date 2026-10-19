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
	// EdgeThroughNode counts edges passing through a node, once per edge
	// and node: a node other than their own ends, or their own end past
	// the segment that leaves or reaches it, see Shafts for that one.
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
	// Corners counts the bends of all edges together.
	Corners int
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
	// Detours counts edges that head away from their end before turning
	// to it, by more than two arrowhead lengths: readers expect an edge to
	// go towards where it ends (Huang, Eades and Hong, "A graph reading
	// behavior: geodesic-path tendency", 2009).
	Detours int
	// EdgeLength is the total length of all edge paths, in points.
	EdgeLength Length
	// Area is the width times height of the drawing, in points.
	Area Length
	// Unbalance is how far forks and joins are off center, in points: for
	// every node with two or more neighbors in the ranks before it, and
	// again after it, how far along its rank it is from their median, as
	// positioning places it, where a neighbor counts once for every edge
	// to it.
	Unbalance Length
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
	return fmt.Sprintf("nodes=%d through=%d near=%d crossings=%d shallow=%d overlaps=%d parallel=%d ends=%d shafts=%d jagged=%d bends=%d corners=%d wavy=%d back=%d detours=%d labels=%d far=%d length=%.0f area=%.0f unbalance=%.0f score=%d",
		diag.NodeOverlaps, diag.EdgeThroughNode, diag.EdgeNearNode, diag.EdgeCrossings, diag.ShallowCrossings, diag.EdgeOverlaps, diag.ParallelEdges, diag.EndOverlaps, diag.Shafts,
		diag.JaggedEdges, diag.BendyEdges, diag.Corners, diag.WavyEdges, diag.BackEdges, diag.Detours, diag.LabelOverlaps, diag.FarLabels, diag.EdgeLength, diag.Area, diag.Unbalance, diag.Score())
}

// Score weighs the counted defects into one number, lower is better, for
// comparing layouts of the same graph. Broken drawings (overlaps, edges
// through nodes) weigh most, then crossings, which user studies find hurt
// reading the most (Purchase, "Which aesthetic has the greatest effect on
// human understanding?", 1997), more so when shallow (Huang, Eades and
// Hong, "Larger crossing angles make graphs easier to read", 2014), then
// what breaks following a path by eye (Ware et al., "Cognitive
// measurements of graph aesthetics", 2002): hooks, detours, wobbles and
// bends.
// Length, area and unbalance are left out, they are in points and
// matter less.
func (diag Diagnostics) Score() int {
	return 100*diag.NodeOverlaps +
		50*diag.EdgeThroughNode +
		20*(diag.EdgeOverlaps+diag.LabelOverlaps) +
		10*(diag.EdgeCrossings+diag.Shafts+diag.EndOverlaps) +
		5*(diag.ShallowCrossings+diag.JaggedEdges+diag.BackEdges+diag.FarLabels) +
		4*diag.Detours +
		3*(diag.WavyEdges+diag.EdgeNearNode) +
		2*(diag.BendyEdges+diag.ParallelEdges) +
		diag.Corners
}

// Diagnose computes Diagnostics for a layout.
func Diagnose(l *Layout) Diagnostics {
	d := newDiagnosis(l.work())
	d.nodeOverlaps()
	d.throughNodes()
	d.segmentPairs()
	d.shafts()
	d.endOverlaps()
	d.paths()
	tl, br := d.graph.Bounds()
	d.diag.Area = (br.X - tl.X) * (br.Y - tl.Y)
	d.unbalance()
	d.farLabels()
	d.labelOverlaps()
	return d.diag
}

const (
	// diagEps is how far apart two lengths are the same in the
	// diagnostics
	diagEps = 1e-3
	// segments closer than a stroke width over more than an arrowhead
	// length draw as one line
	diagStroke, diagArrow = 1.5 * Point, 6 * Point
)

// diagnosis is the drawn part of a layout that Diagnose measures, and the
// diagnostics so far
type diagnosis struct {
	graph *lgraph
	diag  Diagnostics
	// what isn't drawn can't cross, overlap or crowd anything
	nodes    []*lnode
	edges    []*ledge
	segments []segment
}

// segment is a straight piece of an edge as drawn, see newDiagnosis
type segment struct {
	a, b        Vector
	edge        *ledge
	first, last bool // the segment at the start, at the end
}

// dir returns the segment as a vector from a to b
func (s segment) dir() Vector { return s.b.Sub(s.a) }

// overlap returns how far q runs along p, with both projected onto p's line
func (s segment) overlap(q segment) float64 {
	d := s.dir()
	length := float64(d.Len())
	proj := func(v Vector) float64 { return float64(v.Sub(s.a).Dot(d)) / length }
	lo, hi := math.Min(proj(q.a), proj(q.b)), math.Max(proj(q.a), proj(q.b))
	return math.Min(hi, length) - math.Max(lo, 0)
}

func newDiagnosis(graph *lgraph) *diagnosis {
	d := &diagnosis{graph: graph}
	for _, node := range graph.Nodes {
		if !node.Invisible {
			d.nodes = append(d.nodes, node)
		}
	}
	for _, edge := range graph.Edges {
		if !edge.Invisible {
			d.edges = append(d.edges, edge)
		}
	}
	for _, edge := range d.edges {
		path := d.drawn(edge)
		for i := 0; i+1 < len(path); i++ {
			d.segments = append(d.segments, segment{path[i], path[i+1], edge, i == 0, i+2 == len(path)})
		}
	}

	return d
}

// drawn returns the path of edge as drawn, rounded corners flattened
func (d *diagnosis) drawn(edge *ledge) Path {
	if d.graph.Splines == SplinesRounded {
		return flattenPath(edge.Path, 2*d.graph.RowPadding, d.graph.EdgePadding)
	}
	return edge.Path
}

// detail adds a line to the details
func (d *diagnosis) detail(format string, args ...any) {
	d.diag.Details = append(d.diag.Details, fmt.Sprintf(format, args...))
}

func (d *diagnosis) nodeOverlaps() {
	for i, a := range d.nodes {
		for _, b := range d.nodes[i+1:] {
			if a.rect().overlaps(b.rect(), diagEps) {
				d.diag.NodeOverlaps++
				d.detail("nodes %v and %v overlap", a, b)
			}
		}
	}
}

// throughNodes counts the edges through nodes and near them
func (d *diagnosis) throughNodes() {
	type edgeNode struct {
		edge *ledge
		node *lnode
	}
	through := map[edgeNode]bool{}
	near := map[edgeNode]bool{}
	for _, s := range d.segments {
		for _, node := range d.nodes {
			own := node == s.edge.From || node == s.edge.To
			// an edge leaves and reaches its own ends, and a loop runs
			// beside its node all along
			atEnd := node == s.edge.From && s.first || node == s.edge.To && s.last
			if own && (s.edge.loop() || atEnd) {
				continue
			}
			key := edgeNode{s.edge, node}
			if !through[key] && segmentHitsNode(s.a, s.b, node, -diagEps) {
				through[key] = true
				d.diag.EdgeThroughNode++
				d.detail("edge %v through node %v at %v-%v", s.edge, node, s.a, s.b)
			} else if !own && !through[key] && !near[key] && segmentHitsNode(s.a, s.b, node, d.graph.EdgePadding) {
				near[key] = true
				d.diag.EdgeNearNode++
				d.detail("edge %v near node %v at %v-%v", s.edge, node, s.a, s.b)
			}
		}
	}
}

// alongside reports whether q runs within dist of p's line for more than
// minLen
func alongside(p, q segment, dist, minLen Length) bool {
	length := float64(p.dir().Len())
	if length < diagEps {
		return false
	}
	if math.Abs(side(p.a, p.b, q.a))/length >= float64(dist) || math.Abs(side(p.a, p.b, q.b))/length >= float64(dist) {
		return false
	}
	return p.overlap(q) > float64(minLen)
}

// cross reports whether p and q cross, and whether they run as one line
// instead, within a stroke of each other for more than an arrowhead
func cross(p, q segment) (crosses, collinear bool) {
	length := float64(p.dir().Len())
	if length < diagEps {
		return false, false
	}
	d1, d2 := side(p.a, p.b, q.a), side(p.a, p.b, q.b)
	d3, d4 := side(q.a, q.b, p.a), side(q.a, q.b, p.b)

	// distance of q's ends from p's line
	if math.Abs(d1)/length < float64(diagStroke) && math.Abs(d2)/length < float64(diagStroke) {
		return false, p.overlap(q) > float64(diagArrow)
	}

	// a proper crossing has each segment's ends on opposite sides of
	// the other; touching at an end does not count
	return d1*d2 < -diagEps && d3*d4 < -diagEps, false
}

// side returns twice the signed area of a, b and c: which side of the
// line through a and b c is on, and how far
func side(a, b, c Vector) float64 {
	return float64(b.Sub(a).Cross(c.Sub(a)))
}

// segmentPairs counts the edges that run alongside, cross and overlap
func (d *diagnosis) segmentPairs() {
	type edgePair struct{ a, b *ledge }
	overlaps := map[edgePair]bool{}
	parallel := map[edgePair]bool{}
	for i, p := range d.segments {
		for _, q := range d.segments[i+1:] {
			if p.edge == q.edge {
				continue
			}
			// merged edges run together on purpose, as one line
			together := d.mergedWith(p.edge, q.edge)
			if pair := (edgePair{p.edge, q.edge}); !together && !parallel[pair] && alongside(p, q, diagArrow+diagStroke, 4*diagArrow) {
				parallel[pair] = true
				d.diag.ParallelEdges++
				d.detail("edges %v and %v run alongside at %v-%v", p.edge, q.edge, p.a, p.b)
			}
			crosses, collinear := cross(p, q)
			if crosses {
				d.diag.EdgeCrossings++
				// angle between the segments
				u, v := p.dir(), q.dir()
				angle := math.Abs(turn(u, v))
				angle = math.Min(angle, math.Pi-angle)
				if angle < 30*degree {
					d.diag.ShallowCrossings++
					d.detail("edges %v and %v cross at %.0f degrees", p.edge, q.edge, degrees(angle))
				}
			}
			if collinear && !together && !overlaps[edgePair{p.edge, q.edge}] {
				overlaps[edgePair{p.edge, q.edge}] = true
				d.diag.EdgeOverlaps++
				d.detail("edges %v and %v overlap at %v-%v", p.edge, q.edge, p.a, p.b)
			}
		}
	}
}

// mergedWith reports whether a and b merge at an end, see
// Graph.MergeEdges
func (d *diagnosis) mergedWith(a, b *ledge) bool {
	ma, mb := d.graph.merged[a], d.graph.merged[b]
	for _, x := range ma {
		if x != 0 && (x == mb[0] || x == mb[1]) {
			return true
		}
	}
	return false
}

// shafts counts the ends whose shaft enters their own node before the
// tip, which draws the arrowhead over the node: the segment before the
// tip, trimmed at the tip, or the tip itself inside the node
func (d *diagnosis) shafts() {
	trim := func(inner, tip Vector) (Vector, Vector) {
		return inner, Vector{tip.X + (inner.X-tip.X)*0.05, tip.Y + (inner.Y-tip.Y)*0.05}
	}
	inside := func(p Vector, node *lnode) bool { return segmentHitsNode(p, p, node, -0.5) }
	for _, edge := range d.edges {
		if edge.loop() {
			continue
		}
		path := d.drawn(edge)
		if len(path) < 2 {
			continue
		}
		if a, b := trim(path[1], path[0]); segmentHitsNode(a, b, edge.From, -diagEps) || inside(path[0], edge.From) {
			d.diag.Shafts++
			d.detail("edge %v starts inside node %v", edge, edge.From)
		}
		if a, b := trim(path[len(path)-2], path[len(path)-1]); segmentHitsNode(a, b, edge.To, -diagEps) || inside(path[len(path)-1], edge.To) {
			d.diag.Shafts++
			d.detail("edge %v ends inside node %v", edge, edge.To)
		}
	}
}

// endOverlaps counts the ends closer than an arrowhead, which stack their
// arrowheads
func (d *diagnosis) endOverlaps() {
	type endpoint struct {
		edge *ledge
		p    Vector
	}
	var ends []endpoint
	for _, edge := range d.edges {
		if len(edge.Path) > 0 {
			ends = append(ends, endpoint{edge, edge.Path[0]}, endpoint{edge, edge.Path[len(edge.Path)-1]})
		}
	}
	for i, a := range ends {
		for _, b := range ends[i+1:] {
			if a.edge != b.edge && !d.mergedWith(a.edge, b.edge) && a.p.Sub(b.p).Len() < diagArrow {
				d.diag.EndOverlaps++
				d.detail("ends of %v and %v overlap at %v", a.edge, b.edge, a.p)
			}
		}
	}
}

// paths measures every edge: its length, whether it points against the
// rank direction, and how it turns
func (d *diagnosis) paths() {
	// rank direction as a unit vector
	flow := Vector{0, 1}
	switch d.graph.RankDir {
	case LeftToRight:
		flow = Vector{1, 0}
	case RightToLeft:
		flow = Vector{-1, 0}
	case BottomToTop:
		flow = Vector{0, -1}
	}

	for _, edge := range d.edges {
		for a, b := range edge.Path.Segments() {
			d.diag.EdgeLength += b.Sub(a).Len()
		}
		if edge.loop() {
			continue
		}
		if v := edge.To.Center.Sub(edge.From.Center); v.Dot(flow) < 0 {
			d.diag.BackEdges++
			d.detail("edge %v points against the rank direction", edge)
		}
		winds := 0.0
		bends := 0
		flips := 0
		last := 0.0
		path := edge.Path
		for i := 1; i+1 < len(path); i++ {
			a, b := path[i].Sub(path[i-1]), path[i+1].Sub(path[i])
			if a.IsZero() || b.IsZero() {
				continue
			}
			angle := turn(a, b)
			winds += angle
			if math.Abs(angle) > 5*degree {
				bends++
				if last != 0 && (angle > 0) != (last > 0) {
					flips++
				}
				last = angle
			}
		}

		if away := detour(path); away > 2*diagArrow {
			d.diag.Detours++
			d.detail("edge %v heads %.0f away from its end", edge, away)
		}
		if flips >= 2 {
			d.diag.WavyEdges++
			d.detail("edge %v changes turn direction %d times", edge, flips)
		}
		if math.Abs(winds) > 90*degree {
			d.diag.JaggedEdges++
			d.detail("edge %v winds %.0f degrees", edge, degrees(math.Abs(winds)))
		}
		d.diag.Corners += bends
		if bends > 3 {
			d.diag.BendyEdges++
			d.detail("edge %v has %d bends", edge, bends)
		}
	}
}

// detour returns how far the path gets further from its end than it
// has already been, at most
func detour(path Path) Length {
	end := path[len(path)-1]
	closest, away := path[0].Sub(end).Len(), Length(0)
	for a, b := range path.Segments() {
		// along a segment, the distance falls to the nearest point and
		// then rises to b
		t, dx, dy := projection(end, a, b)
		near := Vector{a.X + Length(t*dx), a.Y + Length(t*dy)}
		closest = min(closest, near.Sub(end).Len())
		away = max(away, b.Sub(end).Len()-closest)
	}
	return away
}

// unbalance adds up how far forks and joins are off the median of their
// neighbors, before and after along the ranks, whichever way their edges
// go
func (d *diagnosis) unbalance() {
	across, along := func(v Vector) Length { return v.Y }, func(v Vector) Length { return v.X }
	if d.graph.RankDir.Sideways() {
		across, along = along, across
	}
	for _, node := range d.nodes {
		var before, after []*lnode
		for _, edge := range d.edges {
			other := edge.To
			if edge.To == node {
				other = edge.From
			} else if edge.From != node {
				continue
			}

			// a neighbor counts once for every edge to it
			switch v := across(other.Center) - across(node.Center); {
			case v < -diagEps:
				before = append(before, other)
			case v > diagEps:
				after = append(after, other)
			}
		}

		for _, side := range [][]*lnode{before, after} {
			if !slices.ContainsFunc(side, func(other *lnode) bool { return other != side[0] }) {
				continue // no fork or join
			}
			at := make([]Length, len(side))
			for i, other := range side {
				at[i] = along(other.Center)
			}
			slices.Sort(at)
			median := (at[(len(at)-1)/2] + at[len(at)/2]) / 2
			if off := (along(node.Center) - median).Abs(); off > diagEps {
				d.diag.Unbalance += off
			}
		}
	}
}

// farLabels counts the labels further from their own edge than twice
// their height
func (d *diagnosis) farLabels() {
	for _, edge := range d.edges {
		if edge.Label == "" || len(edge.Path) < 2 {
			continue
		}
		path := d.drawn(edge)
		label := edge.labelRect()
		best := label.pathDistance(path)
		if limit := float64(2 * edge.LabelRadius.Y); best > limit {
			d.diag.FarLabels++
			d.detail("label %q of %v is %.0f from its edge", edge.Label, edge, best)
		}
	}
}

// labelOverlaps counts the labels over a node, an edge or another label
func (d *diagnosis) labelOverlaps() {
	var labels []rect
	var labelEdges []*ledge
	for _, edge := range d.edges {
		if edge.Label == "" {
			continue
		}
		labelEdges = append(labelEdges, edge)
		labels = append(labels, edge.labelRect())
	}

	for i, l := range labels {
		var hits []string
		for _, node := range d.nodes {
			if l.overlaps(node.rect(), diagEps) {
				hits = append(hits, "node "+node.String())
			}
		}
		for _, s := range d.segments {
			if l.grow(-diagEps).hitBy(s.a, s.b) {
				hits = append(hits, "edge "+s.edge.String())
			}
		}
		for k, o := range labels[:i] {
			if l.overlaps(o, diagEps) {
				hits = append(hits, "label "+labelEdges[k].Label)
			}
		}
		if len(hits) > 0 {
			d.diag.LabelOverlaps++
			d.detail("label %q of %v overlaps %s", labelEdges[i].Label, labelEdges[i], strings.Join(slices.Compact(hits), ", "))
		}
	}
}

// segmentHitsNode reports whether segment ab enters the node's outline
// grown by pad (negative shrinks, so touching does not count).
func segmentHitsNode(a, b Vector, node *lnode, pad Length) bool {
	switch node.Shape {
	case Auto, Box, Square, Record, None:
		return node.rect().grow(pad).hitBy(a, b)
	case Diamond, Octagon:
		// clip the segment, in parts of the grown half size, to every side
		rx, ry := float64(node.Radius.X+pad), float64(node.Radius.Y+pad)
		if rx <= 0 || ry <= 0 {
			return false
		}
		ax, ay := float64(a.X-node.Center.X)/rx, float64(a.Y-node.Center.Y)/ry
		dx, dy := float64(b.X-a.X)/rx, float64(b.Y-a.Y)/ry
		t0, t1 := 0.0, 1.0
		hit := true
		sides(node.Shape.polygon(), func(nx, ny, h float64) {
			num, den := h-(nx*ax+ny*ay), nx*dx+ny*dy
			switch {
			case den == 0:
				hit = hit && num > 0
			case den > 0:
				t1 = min(t1, num/den)
			default:
				t0 = max(t0, num/den)
			}
		})
		return hit && t0 < t1
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

// distance returns the distance between r and segment ab, zero when they
// touch.
func (r rect) distance(a, b Vector) float64 {
	if r.hitBy(a, b) {
		return 0
	}
	pointSegment := func(p, a, b Vector) float64 {
		t, dx, dy := projection(p, a, b)
		return math.Hypot(float64(p.X-a.X)-t*dx, float64(p.Y-a.Y)-t*dy)
	}

	pointRect := func(p Vector) float64 {
		dx := math.Max(0, math.Max(float64(r.Min.X-p.X), float64(p.X-r.Max.X)))
		dy := math.Max(0, math.Max(float64(r.Min.Y-p.Y), float64(p.Y-r.Max.Y)))
		return math.Hypot(dx, dy)
	}
	best := math.Min(pointRect(a), pointRect(b))
	for _, c := range r.corners() {
		best = math.Min(best, pointSegment(c, a, b))
	}
	return best
}

// degree is an angle of one degree in radians
const degree = math.Pi / 180

// turn returns the signed angle in radians from a to b, from -Pi to Pi
func turn(a, b Vector) float64 { return math.Atan2(float64(a.Cross(b)), float64(a.Dot(b))) }

// degrees returns rad in degrees
func degrees(rad float64) float64 { return rad * 180 / math.Pi }
