package layout

import (
	"math"
	"slices"
	"sort"
)

// orthoEdges rewrites edge paths into vertical and horizontal segments.
// Vertical runs stay at the waypoint x; every horizontal jog is placed in
// the channel between the two ranks, on its own track when it overlaps
// another jog there. Loops and flat edges keep their paths.
func orthoEdges(graph *Graph, rows [][2]Length, pad Length) {
	type jog struct {
		edge   *Edge
		index  int // index of the segment start in edge.Path
		x0, x1 Length
		xin    Length // x where the edge enters the channel from above
		y      Length
	}
	channels := make([][]*jog, len(rows))

	// channel below the row that contains y
	channelAt := func(y Length) int {
		for k := 0; k+1 < len(rows); k++ {
			if y < rows[k+1][0] {
				return k
			}
		}
		return -1
	}

	// point on the node outline at x, on its bottom or top side
	outline := func(node *Node, x Length, bottom bool) Vector {
		dx := float64(x - node.Center.X)
		dy := float64(node.Radius.Y)
		switch node.Shape {
		case Box, Square, Record, None:
		default: // ellipse
			if rx := float64(node.Radius.X); rx > 0 {
				dy *= math.Sqrt(math.Max(0, 1-(dx/rx)*(dx/rx)))
			}
		}
		if !bottom {
			dy = -dy
		}
		return Vector{x, node.Center.Y + Length(dy)}
	}

	// edge ends per node side, spread across the node width in the order
	// of where they head so that stubs neither coincide nor cross
	type end struct {
		edge    *Edge
		start   bool
		towards Length
	}
	type side struct {
		node   *Node
		bottom bool
	}
	ends := map[side][]end{}
	routed := func(edge *Edge) bool {
		return edge.From != edge.To && len(edge.Path) >= 2 && edge.Path[0].Y != edge.Path[len(edge.Path)-1].Y
	}
	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		down := edge.Path[0].Y < edge.Path[len(edge.Path)-1].Y
		if edge.FromPort == CompassAuto {
			k := side{edge.From, down}
			ends[k] = append(ends[k], end{edge, true, edge.Path[1].X})
		}
		if edge.ToPort == CompassAuto {
			k := side{edge.To, !down}
			ends[k] = append(ends[k], end{edge, false, edge.Path[len(edge.Path)-2].X})
		}
	}
	for k, list := range ends {
		sort.SliceStable(list, func(i, j int) bool { return list[i].towards < list[j].towards })
		spacing := min(2*pad, 2*k.node.Radius.X/Length(len(list)+1))
		for i, e := range list {
			x := k.node.Center.X + (Length(i)-Length(len(list)-1)/2)*spacing
			p := outline(k.node, x, k.bottom)
			if e.start {
				e.edge.Path[0] = p
			} else {
				e.edge.Path[len(e.edge.Path)-1] = p
			}
		}
	}

	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		path := slices.Clone(edge.Path)
		for i := 0; i+1 < len(path); i++ {
			a, b := path[i], path[i+1]
			if a.X == b.X {
				continue
			}
			top, bottom := a, b
			if top.Y > bottom.Y {
				top, bottom = bottom, top
			}
			k := channelAt(top.Y)
			if k < 0 || rows[k+1][0] > bottom.Y+0.5 {
				continue // not a rank-to-rank segment
			}
			channels[k] = append(channels[k], &jog{edge: edge, index: i, x0: min(a.X, b.X), x1: max(a.X, b.X), xin: top.X})
		}
		edge.Path = path
	}

	// tracks per channel: overlapping jogs get distinct tracks. Top to
	// bottom: jogs heading right by entry x descending, then jogs heading
	// left by entry x ascending, so continuations don't cut through the
	// jogs below them
	for k, jogs := range channels {
		if len(jogs) == 0 {
			continue
		}
		// a rightward jog exiting at the same x where a leftward jog
		// enters would run down its stub; then the leftward group goes on top
		rightOnTop := true
		for _, r := range jogs {
			for _, l := range jogs {
				if r.xin == r.x0 && l.xin == l.x1 && r.x1 == l.x1 {
					rightOnTop = false
				}
			}
		}
		sort.SliceStable(jogs, func(i, j int) bool {
			a, b := jogs[i], jogs[j]
			ra, rb := a.xin == a.x0, b.xin == b.x0 // heading right
			if ra != rb {
				return ra == rightOnTop
			}
			if ra {
				return a.xin > b.xin
			}
			return a.xin < b.xin
		})
		var trackEnd []Length // right end of the last jog on each track
		track := make([]int, len(jogs))
		for i, j := range jogs {
			// below every overlapping jog placed before, then first fit
			first := 0
			for o := range i {
				if jogs[o].x0 < j.x1+pad && j.x0 < jogs[o].x1+pad {
					first = max(first, track[o]+1)
				}
			}
			track[i] = -1
			for t := first; t < len(trackEnd); t++ {
				if trackEnd[t]+pad <= j.x0 {
					track[i], trackEnd[t] = t, j.x1
					break
				}
			}
			if track[i] < 0 {
				track[i] = len(trackEnd)
				trackEnd = append(trackEnd, j.x1)
			}
		}
		top, bottom := rows[k][1], rows[k+1][0]
		spacing := min(pad, (bottom-top)/Length(len(trackEnd)+1))
		for i, j := range jogs {
			j.y = (top+bottom)/2 + (Length(track[i])-Length(len(trackEnd)-1)/2)*spacing
		}
	}

	// insert the jog points per edge, later segments first so that
	// earlier indices stay valid
	byEdge := map[*Edge][]*jog{}
	for _, jogs := range channels {
		for _, j := range jogs {
			byEdge[j.edge] = append(byEdge[j.edge], j)
		}
	}
	for edge, js := range byEdge {
		sort.Slice(js, func(a, b int) bool { return js[a].index > js[b].index })
		for _, j := range js {
			a, b := edge.Path[j.index], edge.Path[j.index+1]
			edge.Path = slices.Insert(edge.Path, j.index+1, Vector{a.X, j.y}, Vector{b.X, j.y})
		}
	}

	// whatever is still diagonal (ports, offsets) gets a jog at mid height
	for _, edge := range graph.Edges {
		if edge.From == edge.To || len(edge.Path) < 2 || edge.Path[0].Y == edge.Path[len(edge.Path)-1].Y {
			continue
		}
		for i := 0; i+1 < len(edge.Path); i++ {
			a, b := edge.Path[i], edge.Path[i+1]
			if a.X != b.X && a.Y != b.Y {
				mid := (a.Y + b.Y) / 2
				edge.Path = slices.Insert(edge.Path, i+1, Vector{a.X, mid}, Vector{b.X, mid})
				i += 2
			}
		}
	}
}
