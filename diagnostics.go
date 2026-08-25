package layout

import (
	"fmt"
	"math"
	"sort"
)

// Diagnostics counts layout defects that are otherwise only visible by eye.
// Edge paths are measured as drawn, with rounded corners flattened.
type Diagnostics struct {
	// NodeOverlaps counts pairs of node boxes that intersect.
	NodeOverlaps int
	// EdgeThroughNode counts edge segments that pass through a node box
	// other than the edge's own ends.
	EdgeThroughNode int
	// EdgeCrossings counts pairs of edge segments that cross.
	EdgeCrossings int
	// EdgeOverlaps counts pairs of collinear edge segments that share more
	// than a point.
	EdgeOverlaps int
	// EndOverlaps counts pairs of edge ends (starts or ends) at the same
	// point, where arrowheads would draw on top of each other.
	EndOverlaps int
	// LabelOverlaps counts labels that intersect a node, an edge segment or
	// another label.
	LabelOverlaps int
}

// String formats the diagnostics as one line of key=value pairs.
func (m Diagnostics) String() string {
	return fmt.Sprintf("nodes=%d through=%d crossings=%d overlaps=%d ends=%d labels=%d",
		m.NodeOverlaps, m.EdgeThroughNode, m.EdgeCrossings, m.EdgeOverlaps, m.EndOverlaps, m.LabelOverlaps)
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
			}
		}
	}

	for _, s := range segments {
		for _, node := range graph.Nodes {
			if node == s.edge.From || node == s.edge.To {
				continue
			}
			tl, br := boxes(node)
			// shrink so that touching the outline is not a hit
			tl, br = tl.Add(Vector{eps, eps}), br.Add(Vector{-eps, -eps})
			if segmentHitsRect(s.a, s.b, tl, br) {
				m.EdgeThroughNode++
			}
		}
	}

	cross := func(p, q segment) (crosses, collinear bool) {
		d := func(a, b, c Vector) float64 {
			return float64((b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X))
		}
		d1, d2 := d(p.a, p.b, q.a), d(p.a, p.b, q.b)
		d3, d4 := d(q.a, q.b, p.a), d(q.a, q.b, p.b)
		if math.Abs(d1) < eps && math.Abs(d2) < eps {
			// collinear: overlapping if the projections share a stretch
			dx, dy := float64(p.b.X-p.a.X), float64(p.b.Y-p.a.Y)
			proj := func(v Vector) float64 { return float64(v.X-p.a.X)*dx + float64(v.Y-p.a.Y)*dy }
			lo, hi := math.Min(proj(q.a), proj(q.b)), math.Max(proj(q.a), proj(q.b))
			length := dx*dx + dy*dy
			return false, math.Min(hi, length)-math.Max(lo, 0) > eps*length
		}
		// a proper crossing has each segment's ends on opposite sides of
		// the other; touching at an end does not count
		return d1*d2 < -eps && d3*d4 < -eps, false
	}
	for i, p := range segments {
		for _, q := range segments[i+1:] {
			if p.edge == q.edge {
				continue
			}
			crosses, collinear := cross(p, q)
			if crosses {
				m.EdgeCrossings++
			}
			if collinear {
				m.EdgeOverlaps++
			}
		}
	}

	var ends []Vector
	for _, edge := range graph.Edges {
		if len(edge.Path) > 0 {
			ends = append(ends, edge.Path[0], edge.Path[len(edge.Path)-1])
		}
	}
	sort.Slice(ends, func(i, k int) bool {
		if ends[i].X != ends[k].X {
			return ends[i].X < ends[k].X
		}
		return ends[i].Y < ends[k].Y
	})
	for i := 1; i < len(ends); i++ {
		if math.Abs(float64(ends[i].X-ends[i-1].X)) < eps && math.Abs(float64(ends[i].Y-ends[i-1].Y)) < eps {
			m.EndOverlaps++
		}
	}

	type box struct{ tl, br Vector }
	var labels []box
	for _, edge := range graph.Edges {
		if edge.Label == "" {
			continue
		}
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
		}
	}
	return m
}
