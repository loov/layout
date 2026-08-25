package layout

import (
	"fmt"
	"math"
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
	// EndOverlaps counts pairs of edge ends (starts or ends) closer than an
	// arrowhead, where arrowheads draw on top of each other.
	EndOverlaps int
	// JaggedEdges counts edges whose path winds by more than 90 degrees
	// net: hooks and U-turns (an S-bend nets out to zero, see BendyEdges).
	JaggedEdges int
	// BendyEdges counts edges with more than three bends.
	BendyEdges int
	// LabelOverlaps counts labels that intersect a node, an edge segment or
	// another label.
	LabelOverlaps int

	// Details names every counted finding, one per line.
	Details []string
}

// String formats the diagnostics as one line of key=value pairs.
func (m Diagnostics) String() string {
	return fmt.Sprintf("nodes=%d through=%d crossings=%d overlaps=%d ends=%d jagged=%d bends=%d labels=%d",
		m.NodeOverlaps, m.EdgeThroughNode, m.EdgeCrossings, m.EdgeOverlaps, m.EndOverlaps, m.JaggedEdges, m.BendyEdges, m.LabelOverlaps)
}

// Diagnose computes Diagnostics for a laid out graph.
func Diagnose(graph *Graph) Diagnostics {
	var m Diagnostics
	const eps = 1e-3

	type segment struct {
		a, b Vector
		edge *Edge
	}
	var segments []segment
	for _, edge := range graph.Edges {
		path := edge.Path
		if graph.Splines == SplinesRounded {
			path = flattenPath(path, 2*graph.RowPadding)
		}
		for i := 0; i+1 < len(path); i++ {
			segments = append(segments, segment{path[i], path[i+1], edge})
		}
	}

	boxes := func(node *Node) (tl, br Vector) {
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
				m.NodeOverlaps++
				m.Details = append(m.Details, fmt.Sprintf("nodes %v and %v overlap", a, b))
			}
		}
	}

	type edgeNode struct {
		edge *Edge
		node *Node
	}
	through := map[edgeNode]bool{}
	for _, s := range segments {
		for _, node := range graph.Nodes {
			if node == s.edge.From || node == s.edge.To || through[edgeNode{s.edge, node}] {
				continue
			}
			if segmentHitsNode(s.a, s.b, node, -eps) {
				through[edgeNode{s.edge, node}] = true
				m.EdgeThroughNode++
				m.Details = append(m.Details, fmt.Sprintf("edge %v through node %v at %v-%v", s.edge, node, s.a, s.b))
			}
		}
	}

	// segments closer than a stroke width over more than an arrowhead
	// length draw as one line
	const stroke, arrow = 1.5 * Point, 6 * Point
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
	type edgePair struct{ a, b *Edge }
	overlaps := map[edgePair]bool{}
	for i, p := range segments {
		for _, q := range segments[i+1:] {
			if p.edge == q.edge {
				continue
			}
			crosses, collinear := cross(p, q)
			if crosses {
				m.EdgeCrossings++
			}
			if collinear && !overlaps[edgePair{p.edge, q.edge}] {
				overlaps[edgePair{p.edge, q.edge}] = true
				m.EdgeOverlaps++
				m.Details = append(m.Details, fmt.Sprintf("edges %v and %v overlap at %v-%v", p.edge, q.edge, p.a, p.b))
			}
		}
	}

	// ends closer than an arrowhead stack their arrowheads
	type endpoint struct {
		edge *Edge
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
				m.EndOverlaps++
				m.Details = append(m.Details, fmt.Sprintf("ends of %v and %v overlap at %v", a.edge, b.edge, a.p))
			}
		}
	}

	for _, edge := range graph.Edges {
		if edge.From == edge.To {
			continue
		}
		turn, bends := 0.0, 0
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
			}
		}
		if math.Abs(turn) > math.Pi/2 {
			m.JaggedEdges++
			m.Details = append(m.Details, fmt.Sprintf("edge %v winds %.0f degrees", edge, math.Abs(turn)*180/math.Pi))
		}
		if bends > 3 {
			m.BendyEdges++
			m.Details = append(m.Details, fmt.Sprintf("edge %v has %d bends", edge, bends))
		}
	}

	type box struct{ tl, br Vector }
	var labels []box
	var labelEdges []*Edge
	for _, edge := range graph.Edges {
		if edge.Label == "" {
			continue
		}
		labelEdges = append(labelEdges, edge)
		labels = append(labels, box{edge.LabelPos.Add(Vector{-edge.LabelRadius.X, -edge.LabelRadius.Y}), edge.LabelPos.Add(edge.LabelRadius)})
	}
	for i, l := range labels {
		hit := false
		for _, node := range graph.Nodes {
			tl, br := boxes(node)
			hit = hit || overlap(l.tl, l.br, tl, br)
		}
		for _, s := range segments {
			hit = hit || segmentHitsRect(s.a, s.b, l.tl.Add(Vector{eps, eps}), l.br.Add(Vector{-eps, -eps}))
		}
		for _, o := range labels[:i] {
			hit = hit || overlap(l.tl, l.br, o.tl, o.br)
		}
		if hit {
			m.LabelOverlaps++
			m.Details = append(m.Details, fmt.Sprintf("label %q overlaps", labelEdges[i].Label))
		}
	}
	return m
}

// segmentHitsNode reports whether segment ab enters the node's outline
// grown by pad (negative shrinks, so touching does not count).
func segmentHitsNode(a, b Vector, node *Node, pad Length) bool {
	switch node.Shape {
	case Box, Square, Record, None:
		return segmentHitsRect(a, b, Vector{node.Left() - pad, node.Top() - pad}, Vector{node.Right() + pad, node.Bottom() + pad})
	}
	// ellipse: scale to a unit circle and test the distance from the center
	rx, ry := float64(node.Radius.X+pad), float64(node.Radius.Y+pad)
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
