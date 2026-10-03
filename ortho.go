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
func orthoEdges(graph *lgraph, rows [][2]Length, pad Length, pack bool) {
	type jog struct {
		edge   *ledge
		index  int // index of the segment start in edge.Path
		x0, x1 Length
		xin    Length // x where the edge enters the channel from above
		y      Length
		next   *jog // second step of a jog split in two, on a lower track
		split  bool // this is the second step
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
	outline := func(node *lnode, x Length, bottom bool) Vector {
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
		edge    *ledge
		start   bool
		towards Length
		x       Length // where the end attaches before spreading
	}
	type side struct {
		node   *lnode
		bottom bool
	}
	ends := map[side][]end{}
	routed := func(edge *ledge) bool {
		return edge.From != edge.To && len(edge.Path) >= 2 && edge.Path[0].Y != edge.Path[len(edge.Path)-1].Y
	}

	// ends on side ports step out sideways before turning, instead of
	// running along the node outline
	sideways := map[Compass]Length{West: -pad, East: pad}
	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		if dx := sideways[edge.FromPort]; dx != 0 {
			p := edge.Path[0]
			edge.Path = slices.Insert(edge.Path, 1, Vector{p.X + dx, p.Y})
		}
		if dx := sideways[edge.ToPort]; dx != 0 {
			p := edge.Path[len(edge.Path)-1]
			edge.Path = slices.Insert(edge.Path, len(edge.Path)-1, Vector{p.X + dx, p.Y})
		}
	}

	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		down := edge.Path[0].Y < edge.Path[len(edge.Path)-1].Y
		if edge.FromPort == CompassAuto {
			k := side{edge.From, down}
			ends[k] = append(ends[k], end{edge, true, edge.Path[1].X, edge.Path[0].X})
		}
		if edge.ToPort == CompassAuto {
			k := side{edge.To, !down}
			ends[k] = append(ends[k], end{edge, false, edge.Path[len(edge.Path)-2].X, edge.Path[len(edge.Path)-1].X})
		}
	}
	for k, list := range ends {
		sort.SliceStable(list, func(i, j int) bool { return list[i].towards < list[j].towards })
		// slots evenly spread around the center, or packed from the left
		// an edge padding apart
		n := Length(len(list))
		spacing := min(2*pad, 2*k.node.Radius.X/(n+1))
		center, reach := k.node.Center.X, k.node.Radius.X-spacing/2
		lo, hi := center-reach, center+reach
		slot := func(i int) Length { return center + (Length(i)-(n-1)/2)*spacing }
		if pack {
			inset := min(pad, k.node.Radius.X)
			lo, hi = center-k.node.Radius.X+inset, center+k.node.Radius.X-inset
			spacing = pad
			if n > 1 {
				spacing = min(pad, (hi-lo)/(n-1))
			}
			slot = func(i int) Length { return lo + Length(i)*spacing }
		}
		// ends whose route already runs straight across the side keep
		// that x, so the edge needs no jog; the rest take the slots
		want := make([]Length, len(list))
		weight := make([]float64, len(list))
		for i, e := range list {
			want[i], weight[i] = slot(i), 1
			if absLength(e.towards-e.x) < 0.01 && e.towards >= lo && e.towards <= hi {
				want[i], weight[i] = e.towards, 1e9
			}
		}
		xs := separate(want, weight, spacing, lo, hi)
		for i, e := range list {
			if weight[i] > 1 && absLength(xs[i]-want[i]) < spacing/1e3 {
				xs[i] = want[i] // undo rounding, the run must stay exactly vertical
			}
			p := outline(k.node, xs[i], k.bottom)
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
		// a rightward and a leftward jog over the same span run down each
		// other's stubs on any tracks; split the rightward one into two
		// steps that cross the leftward one in the middle
		for _, r := range jogs {
			if r.xin != r.x0 || r.next != nil {
				continue
			}
			for _, l := range jogs {
				if l.xin == l.x1 && l.x0 == r.x0 && l.x1 == r.x1 {
					mid := (r.x0 + r.x1) / 2
					r.next = &jog{edge: r.edge, index: r.index, x0: mid, x1: r.x1, xin: mid, split: true}
					r.x1 = mid
					jogs = append(jogs, r.next)
					break
				}
			}
		}
		sort.SliceStable(jogs, func(i, j int) bool {
			a, b := jogs[i], jogs[j]
			ra, rb := a.xin == a.x0, b.xin == b.x0 // heading right
			if ra != rb {
				return ra
			}
			if ra {
				return a.xin > b.xin
			}
			return a.xin < b.xin
		})
		// a jog entering where another exits goes above it, or the exit
		// would run down its stub; otherwise keep the sorted order
		above := func(a, b *jog) bool {
			return b == a.next || !a.split && b.next == nil && a.xin == b.x0+b.x1-b.xin
		}
		ordered := make([]*jog, 0, len(jogs))
		for len(jogs) > 0 {
			pick := 0 // on a cycle
			for i, j := range jogs {
				if !slices.ContainsFunc(jogs, func(o *jog) bool { return above(o, j) }) {
					pick = i
					break
				}
			}
			ordered = append(ordered, jogs[pick])
			jogs = slices.Delete(jogs, pick, pick+1)
		}
		jogs, channels[k] = ordered, ordered
		// each jog goes just below the jogs placed before that it comes
		// within half a pad of; ends that close still turn apart. Packed
		// ends are a pad apart, so their jogs keep that
		gap := pad / 2
		if pack {
			gap = pad
		}
		track := make([]int, len(jogs))
		tracks := 0
		for i, j := range jogs {
			for o := range i {
				if jogs[o].x0 < j.x1+gap && j.x0 < jogs[o].x1+gap {
					track[i] = max(track[i], track[o]+1)
				}
			}
			tracks = max(tracks, track[i]+1)
		}
		top, bottom := rows[k][1], rows[k+1][0]
		spacing := min(pad, (bottom-top)/Length(tracks+1))
		for i, j := range jogs {
			j.y = (top+bottom)/2 + (Length(track[i])-Length(tracks-1)/2)*spacing
		}
	}

	// insert the jog points per edge, later segments first so that
	// earlier indices stay valid
	byEdge := map[*ledge][]*jog{}
	for _, jogs := range channels {
		for _, j := range jogs {
			byEdge[j.edge] = append(byEdge[j.edge], j)
		}
	}
	for edge, js := range byEdge {
		sort.Slice(js, func(a, b int) bool { return js[a].index > js[b].index })
		for _, j := range js {
			if j.split {
				continue
			}
			a, b := edge.Path[j.index], edge.Path[j.index+1]
			steps := []Vector{{a.X, j.y}, {b.X, j.y}}
			if n := j.next; n != nil && a.Y < b.Y {
				steps = []Vector{{a.X, j.y}, {n.xin, j.y}, {n.xin, n.y}, {b.X, n.y}}
			} else if n != nil {
				steps = []Vector{{a.X, n.y}, {n.xin, n.y}, {n.xin, j.y}, {b.X, j.y}}
			}
			edge.Path = slices.Insert(edge.Path, j.index+1, steps...)
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

// separate moves the ordered positions want as little as possible, by
// weighted squared distance, so that neighbors are at least gap apart and
// all lie within [lo, hi]. Shifting position i by i*gap turns the gaps
// into an order constraint, solved by pooling adjacent violators.
func separate(want []Length, weight []float64, gap, lo, hi Length) []Length {
	type block struct {
		sum, weight float64
		n           int
	}
	var blocks []block
	for i, x := range want {
		blocks = append(blocks, block{(float64(x) - float64(i)*float64(gap)) * weight[i], weight[i], 1})
		for len(blocks) > 1 {
			a, b := blocks[len(blocks)-2], blocks[len(blocks)-1]
			if a.sum/a.weight <= b.sum/b.weight {
				break
			}
			blocks = append(blocks[:len(blocks)-2], block{a.sum + b.sum, a.weight + b.weight, a.n + b.n})
		}
	}
	xs := make([]Length, 0, len(want))
	for _, b := range blocks {
		y := min(max(b.sum/b.weight, float64(lo)), float64(hi-Length(len(want)-1)*gap))
		for range b.n {
			xs = append(xs, Length(y+float64(len(xs))*float64(gap)))
		}
	}
	return xs
}
