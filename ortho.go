package layout

import (
	"cmp"
	"math"
	"slices"
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
		// where the jog, and the merged jogs that take its track, enter
		// from above and leave below
		ins, outs []Length
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
	stepOut := map[Compass]Length{West: -pad, East: pad}
	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		if dx := stepOut[edge.FromPort]; dx != 0 {
			p := edge.Path[0]
			edge.Path = slices.Insert(edge.Path, 1, Vector{p.X + dx, p.Y})
		}
		if dx := stepOut[edge.ToPort]; dx != 0 {
			p := edge.Path[len(edge.Path)-1]
			edge.Path = slices.Insert(edge.Path, len(edge.Path)-1, Vector{p.X + dx, p.Y})
		}
	}

	// an edge along a rank arcs over it, see the flat edges in layout.go,
	// with both ends on the tops of its nodes, which it shares with the
	// ends of other edges
	arc := func(edge *ledge) bool {
		p := edge.Path
		return edge.From != edge.To && len(p) == 4 && p[0].Y == p[3].Y && p[1].Y < p[0].Y && p[0].X == p[1].X && p[2].X == p[3].X
	}
	var arcEnds []*ledge
	for _, edge := range graph.Edges {
		if !arc(edge) {
			continue
		}
		arcEnds = append(arcEnds, edge)
		if edge.FromPort == CompassAuto {
			k := side{edge.From, false}
			ends[k] = append(ends[k], end{edge, true, edge.Path[2].X, edge.Path[0].X})
		}
		if edge.ToPort == CompassAuto {
			k := side{edge.To, false}
			ends[k] = append(ends[k], end{edge, false, edge.Path[1].X, edge.Path[3].X})
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
		slices.SortStableFunc(list, func(a, b end) int { return cmp.Compare(a.towards, b.towards) })
		// merged ends take one slot a group, that of its middle end
		followers := map[*ledge][]end{}
		groups := map[int][]end{}
		list = slices.DeleteFunc(list, func(e end) bool {
			side := 1
			if e.start {
				side = 0
			}
			g := graph.merged[e.edge][side]
			if g != 0 {
				groups[g] = append(groups[g], e)
			}
			return g != 0
		})
		for _, group := range groups {
			mid := group[len(group)/2]
			followers[mid.edge] = slices.Delete(group, len(group)/2, len(group)/2+1)
			list = append(list, mid)
		}
		slices.SortStableFunc(list, func(a, b end) int { return cmp.Compare(a.towards, b.towards) })
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
		// that x, so the edge needs no jog; the rest take the slots, and so
		// do merged ends, which fork from their slot
		want := make([]Length, len(list))
		weight := make([]float64, len(list))
		for i, e := range list {
			want[i], weight[i] = slot(i), 1
			if absLength(e.towards-e.x) < 0.01 && e.towards >= lo && e.towards <= hi && len(followers[e.edge]) == 0 {
				want[i], weight[i] = e.towards, 1e9
			}
		}
		// a straight end of edges merged at their other end takes the slot
		// nearest it, and the rest the other slots in order: an end heading
		// past it crosses its line rather than push it off it, where the
		// merged edges run on together and the end would come in along them
		firm := func(i int) bool {
			return weight[i] > 1 && graph.merged[list[i].edge][map[bool]int{true: 1, false: 0}[list[i].start]] != 0
		}
		at := make([]int, len(list))
		taken := make([]bool, len(list))
		for i := range list {
			if !firm(i) {
				continue
			}
			best := -1
			for j := range list {
				if !taken[j] && (best < 0 || absLength(slot(j)-want[i]) < absLength(slot(best)-want[i])) {
					best = j
				}
			}
			at[i], taken[best] = best, true
		}
		j := 0
		for i := range list {
			if !firm(i) {
				for taken[j] {
					j++
				}
				at[i], taken[j] = j, true
				if weight[i] == 1 {
					want[i] = slot(j)
				}
			}
		}
		order := make([]int, len(list))
		for i := range order {
			order[at[i]] = i
		}
		list, want, weight = permute(list, order), permute(want, order), permute(weight, order)
		xs := separate(want, weight, spacing, lo, hi)
		for i, e := range list {
			if weight[i] > 1 && absLength(xs[i]-want[i]) < spacing/1e3 {
				xs[i] = want[i] // undo rounding, the run must stay exactly vertical
			}
			p := outline(k.node, xs[i], k.bottom)
			for _, f := range append([]end{e}, followers[e.edge]...) {
				if f.start {
					f.edge.Path[0] = p
				} else {
					f.edge.Path[len(f.edge.Path)-1] = p
				}
			}
		}
	}

	// arcs go straight up from where their ends moved
	for _, edge := range arcEnds {
		edge.Path[1].X, edge.Path[2].X = edge.Path[0].X, edge.Path[3].X
	}

	// the jogs, and where they enter and leave, in blocks; at most one
	// for every segment, so that the pointers into them stay put
	segments := 0
	for _, edge := range graph.Edges {
		if routed(edge) {
			segments += max(len(edge.Path)-1, 0)
		}
	}
	block := make([]jog, 0, segments)
	xs := make([]Length, 0, 2*segments)
	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		path := edge.Path
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
			// each end its own slice, which appending copies out
			xs = append(xs, top.X, bottom.X)
			n := len(xs)
			block = append(block, jog{edge: edge, index: i, x0: min(a.X, b.X), x1: max(a.X, b.X), xin: top.X,
				ins: xs[n-2 : n-1 : n-1], outs: xs[n-1 : n : n]})
			channels[k] = append(channels[k], &block[len(block)-1])
		}
	}

	// edges along a rank arc over it, in the bottom of the channel above;
	// tracks keep above the arcs, see the flat edges in layout.go
	arcs := make([]Length, len(rows))
	over := make([][]*ledge, len(rows)) // the arcs in each channel
	for _, edge := range graph.Edges {
		if edge.From == edge.To || edge.From.Center.Y != edge.To.Center.Y || len(edge.Path) != 4 {
			continue
		}
		// the channel above the rank of the edge, wherever the arc reaches
		if y := edge.Path[1].Y; y < edge.Path[0].Y {
			r := slices.IndexFunc(rows, func(row [2]Length) bool { return row[0] <= edge.From.Center.Y && edge.From.Center.Y <= row[1] })
			if k := r - 1; k >= 0 {
				arcs[k] = max(arcs[k], rows[k+1][0]-y)
				over[k] = append(over[k], edge)
			}
		}
	}
	if graph.ForText {
		// text draws the arcs on rows of their own, below the nodes of
		// the rank above with a row between; the rank of the arcs, and
		// what is below it, moves down to make the room
		for k := range arcs {
			if arcs[k] == 0 {
				continue
			}
			if short := rows[k][1] + graph.LineHeight - (rows[k+1][0] - arcs[k]); short > 0 {
				below := rows[k][1] + graph.LineHeight/2
				graph.shiftBelow(below, short)
				for _, edge := range over[k] {
					for i := range edge.Path {
						if edge.Path[i].Y <= below {
							edge.Path[i].Y += short // above the cut, as high as the arc reaches
						}
					}
				}
				for r := k + 1; r < len(rows); r++ {
					rows[r][0], rows[r][1] = rows[r][0]+short, rows[r][1]+short
				}
			}
		}
	}

	// tracks per channel: overlapping jogs get distinct tracks. Top to
	// bottom: jogs heading right by entry x descending, then jogs heading
	// left by entry x ascending, so continuations don't cut through the
	// jogs below them
	for k, jogs := range channels {
		if len(jogs) == 0 {
			continue
		}
		// merged edges turn along one track: the first jog of a group
		// spans the rest, which take its track afterwards
		shared := map[*jog][]*jog{}
		first := map[int]*jog{}
		jogs = slices.DeleteFunc(jogs, func(j *jog) bool {
			g := graph.merged[j.edge]
			id := 0
			switch {
			case j.index == 0 && g[0] != 0:
				id = g[0]
			case j.index == len(j.edge.Path)-2 && g[1] != 0:
				id = g[1]
			default:
				return false
			}
			f, ok := first[id]
			if !ok {
				first[id] = j
				return false
			}
			f.x0, f.x1 = min(f.x0, j.x0), max(f.x1, j.x1)
			f.ins, f.outs = append(f.ins, j.ins...), append(f.outs, j.outs...)
			shared[f] = append(shared[f], j)
			return true
		})
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
					r.next = &jog{edge: r.edge, index: r.index, x0: mid, x1: r.x1, xin: mid, split: true,
						ins: []Length{mid}, outs: r.outs}
					r.x1, r.outs = mid, []Length{mid}
					jogs = append(jogs, r.next)
					break
				}
			}
		}
		slices.SortStableFunc(jogs, func(a, b *jog) int {
			ra, rb := a.xin == a.x0, b.xin == b.x0 // heading right
			if ra != rb {
				if ra {
					return -1
				}
				return 1
			}
			if ra {
				return cmp.Compare(b.xin, a.xin)
			}
			return cmp.Compare(a.xin, b.xin)
		})
		// a jog entering where another exits goes above it, or the exit
		// would run down its stub; otherwise keep the sorted order. Where
		// is within half a pad, as text draws closer runs in one cell
		near := func(xs, ys []Length) bool {
			for _, x := range xs {
				for _, y := range ys {
					if absLength(x-y) < pad/2 {
						return true
					}
				}
			}
			return false
		}
		above := func(a, b *jog) bool {
			return b == a.next || !a.split && b.next == nil && near(a.ins, b.outs)
		}
		// jogs that must go above each other around a cycle, as longer
		// chains and swaps over spans that differ can, have no order: the
		// widest of a cycle splits into two steps, on two tracks, at a
		// point where no other jog enters or leaves. The first step enters
		// as the jog did and the second leaves as it did, so nothing needs
		// to go above the first, which breaks the cycle
		for range len(jogs) {
			cycle := jogCycle(jogs, func(a, b *jog) bool { return a != b && above(a, b) })
			var widest *jog
			for _, j := range cycle {
				if !j.split && j.next == nil && (widest == nil || j.x1-j.x0 > widest.x1-widest.x0) {
					widest = j
				}
			}
			if widest == nil {
				break
			}
			apart := func(x Length) bool {
				return !slices.ContainsFunc(jogs, func(o *jog) bool {
					return near(o.ins, []Length{x}) || near(o.outs, []Length{x})
				})
			}
			mid, found := Length(0), false
			for _, f := range []Length{1.0 / 2, 1.0 / 3, 2.0 / 3, 1.0 / 4, 3.0 / 4} {
				if mid = widest.x0 + f*(widest.x1-widest.x0); apart(mid) {
					found = true
					break
				}
			}
			if !found {
				break
			}
			span := func(xs []Length) (Length, Length) { return min(mid, slices.Min(xs)), max(mid, slices.Max(xs)) }
			second := &jog{edge: widest.edge, index: widest.index, xin: mid, split: true, ins: []Length{mid}, outs: widest.outs}
			second.x0, second.x1 = span(widest.outs)
			widest.x0, widest.x1 = span(widest.ins)
			widest.next, widest.outs = second, []Length{mid}
			jogs = append(jogs, second)
		}
		ordered := make([]*jog, 0, len(jogs))
		for len(jogs) > 0 {
			pick := 0 // on a cycle
			for i, j := range jogs {
				if !slices.ContainsFunc(jogs, func(o *jog) bool { return o != j && above(o, j) }) {
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
		top, bottom := rows[k][1], rows[k+1][0]-arcs[k]
		// tracks are a pad apart, or a cell apart in text turned sideways,
		// where the channel runs across the columns of cells
		step := pad
		if graph.ForText && sideways(graph.RankDir) {
			step = graph.cellWidth()
		}
		// above arcs, the arrows are below them, so tracks go right above,
		// high in the channel; and next to the top or bottom of a cluster,
		// which text draws on a row of its own, where the arrows or bends
		// are past it, high above a top and low below a bottom
		margin := 3
		high, low := arcs[k] > 0, false
		if graph.ForText && !sideways(graph.RankDir) {
			crossed := func(x0, x1 Length) bool {
				return slices.ContainsFunc(jogs, func(j *jog) bool { return j.x0 <= x1 && x0 <= j.x1 })
			}
			var lower, upper bool
			top, bottom, lower, upper = offFrames(graph, crossed, top, bottom)
			high, low = high || lower, !high && !lower && upper
		}
		if high || low {
			margin = 1
		}
		if graph.ForText && tracks > 0 {
			// text needs a row a track, and two rows between the outer
			// tracks and the nodes for the bends and the arrows; what is
			// below moves down to make the room
			if short := Length(tracks+margin)*step - (bottom - top); short > 0 {
				graph.shiftBelow((top+bottom)/2, short)
				for r := k + 1; r < len(rows); r++ {
					rows[r][0], rows[r][1] = rows[r][0]+short, rows[r][1]+short
				}
				bottom += short
			}
		}
		spacing := min(step, (bottom-top)/Length(tracks+1))
		for i, j := range jogs {
			j.y = (top+bottom)/2 + (Length(track[i])-Length(tracks-1)/2)*spacing
			switch {
			case high:
				j.y = bottom - Length(tracks-track[i])*spacing
			case low:
				j.y = top + Length(track[i]+1)*spacing
			}
			for _, o := range shared[j] {
				o.y = j.y
				channels[k] = append(channels[k], o)
			}
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
		slices.SortFunc(js, func(a, b *jog) int { return cmp.Compare(b.index, a.index) })
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

// jogCycle returns the jogs around a cycle of jogs that must go above
// each other, or none
func jogCycle[J comparable](jogs []J, above func(a, b J) bool) []J {
	const (
		unseen = iota
		open
		done
	)
	state := map[J]int{}
	var stack, cycle []J
	var visit func(j J) bool
	visit = func(j J) bool {
		state[j] = open
		stack = append(stack, j)
		for _, o := range jogs {
			if !above(j, o) {
				continue
			}
			switch state[o] {
			case open:
				cycle = stack[slices.Index(stack, o):]
				return true
			case unseen:
				if visit(o) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[j] = done
		return false
	}
	for _, j := range jogs {
		if state[j] == unseen && visit(j) {
			return cycle
		}
	}
	return nil
}

// permute returns the items in the order of the indices
func permute[T any](items []T, order []int) []T {
	out := make([]T, len(order))
	for i, j := range order {
		out[i] = items[j]
	}
	return out
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
	blocks := make([]block, 0, len(want))
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

// mergeEdges adds the edge ends that merge, see Graph.MergeEdges, to
// merged by a group id, after the ids already there; 0 for an end that
// doesn't merge, at a port or alone. Ends
// merge when they leave or enter a node on the same side, of edges that
// look the same. An edge that would merge at both ends merges at
// neither, or its line would join the sources of one group to the
// targets of the other. Flat edges and loops don't merge, nor do edges
// with labels, which could end up beside a stretch they share.
func mergeEdges(merged map[*ledge][2]int, edges []*ledge, rank func(*lnode) int) {
	type look struct {
		head, tail Arrow
		style      LineStyle
		width      Length
		color      [5]uint8
	}
	type group struct {
		node         *lnode
		below, start bool
		look         look
	}
	keys := map[*ledge][2]group{}
	count := map[group]int{}
	for _, e := range edges {
		from, to := rank(e.From), rank(e.To)
		if from == to || e.Label != "" {
			continue
		}
		l := look{head: e.ArrowHead, tail: e.ArrowTail, style: e.LineStyle, width: e.LineWidth}
		if e.LineColor != nil {
			r, g, b, a := e.LineColor.RGBA8()
			l.color = [5]uint8{r, g, b, a, 1}
		}
		below := from < to
		k := [2]group{{e.From, below, true, l}, {e.To, !below, false, l}}
		if e.FromPort != CompassAuto {
			k[0].node = nil
		}
		if e.ToPort != CompassAuto {
			k[1].node = nil
		}
		keys[e] = k
		for _, g := range k {
			count[g]++
		}
	}
	merges := func(g group) bool { return g.node != nil && count[g] > 1 }
	var both []*ledge
	for _, e := range edges {
		if k, ok := keys[e]; ok && merges(k[0]) && merges(k[1]) {
			both = append(both, e)
		}
	}
	// all at once, as dropping one leaves another merging; groups only
	// shrink, so no edge comes to merge at both ends
	for _, e := range both {
		count[keys[e][0]]--
		count[keys[e][1]]--
		delete(keys, e)
	}
	ids := map[group]int{}
	base := 0
	for _, m := range merged {
		base = max(base, m[0], m[1])
	}
	for _, e := range edges {
		k, ok := keys[e]
		if !ok {
			continue
		}
		var m [2]int
		for i, g := range k {
			if merges(g) {
				if ids[g] == 0 {
					ids[g] = base + len(ids) + 1
				}
				m[i] = ids[g]
			}
		}
		if m != [2]int{} {
			merged[e] = m
		}
	}
}

// shiftBelow moves what lies below y down by d: nodes, edge paths and
// labels, and the sides of clusters, which stretch across y
func (graph *lgraph) shiftBelow(y, d Length) {
	for _, node := range graph.Nodes {
		if node.Center.Y > y {
			node.Center.Y += d
		}
	}
	for _, edge := range graph.Edges {
		for i := range edge.Path {
			if edge.Path[i].Y > y {
				edge.Path[i].Y += d
			}
		}
		if edge.Label != "" && edge.LabelPos.Y > y {
			edge.LabelPos.Y += d
		}
	}
	for _, cluster := range graph.Clusters {
		if cluster.TopLeft.Y > y {
			cluster.TopLeft.Y += d
		}
		if cluster.BottomRight.Y > y {
			cluster.BottomRight.Y += d
		}
	}
}

// offFrames narrows the channel from top to bottom to the side of the
// top or bottom of a cluster that a jog of the channel crosses, as
// crossed reports for the cluster's span, which text draws on a row of
// its own: tracks would run along it; the side with the middle of the
// channel stays. It reports whether it narrowed the bottom and the top.
func offFrames(graph *lgraph, crossed func(x0, x1 Length) bool, top, bottom Length) (_, _ Length, lower, upper bool) {
	mid := (top + bottom) / 2
	for _, cluster := range graph.Clusters {
		if !crossed(cluster.TopLeft.X, cluster.BottomRight.X) {
			continue
		}
		for _, frame := range []Length{cluster.TopLeft.Y, cluster.BottomRight.Y} {
			switch {
			case frame <= top || bottom <= frame:
			case frame > mid:
				bottom, lower = min(bottom, frame), true
			default:
				top, upper = max(top, frame), true
			}
		}
	}
	return top, bottom, lower, upper
}
