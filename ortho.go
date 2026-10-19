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
func orthoEdges(graph *lgraph, rows rowSpans, pad Length, pack bool) {
	stepOut(graph, pad)
	arcEnds := slotEnds(graph, pad, pack)

	// arcs go straight up from where their ends moved
	for _, edge := range arcEnds {
		edge.Path[1].X, edge.Path[2].X = edge.Path[0].X, edge.Path[3].X
	}

	channels := channelJogs(graph, rows)
	arcs := arcRoom(graph, rows)

	// tracks per channel: overlapping jogs get distinct tracks
	for k, jogs := range channels {
		if len(jogs) == 0 {
			continue
		}
		jogs, shared := shareJogs(graph, jogs)
		through := func() []Length { return throughRuns(graph, rows, k) }
		jogs = orderJogs(splitOpposite(jogs), shared, through, pad, &graph.ortho.over)
		channels[k] = append(jogs, placeTracks(graph, rows, arcs[k], k, jogs, shared, pad, pack)...)

	}

	insertJogs(graph, channels)

	// whatever is still diagonal (ports, offsets) gets a jog at mid height
	for _, edge := range graph.Edges {
		if edge.loop() || len(edge.Path) < 2 || edge.Path[0].Y == edge.Path[len(edge.Path)-1].Y {
			continue
		}
		for i := 0; i+1 < len(edge.Path); i++ {
			a, b := edge.Path[i], edge.Path[i+1]
			if diagonal(a, b) {
				mid := (a.Y + b.Y) / 2
				edge.Path = slices.Insert(edge.Path, i+1, Vector{a.X, mid}, Vector{b.X, mid})
				i += 2
			}
		}
	}
}

// orthoScratch holds the buffers of routing from one route of a graph
// to the next, which straightening's trials repeat
type orthoScratch struct {
	ends     map[nodeSide][]slotEnd
	parallel map[[2]*lnode]int
	channels [][]*jog
	block    []jog
	xs       []Length
	byEdge   map[*ledge][]*jog
	// the ends of a side in slot order, see slotSide
	list        []slotEnd
	want, slots []Length
	weight      []float64
	// whether a jog goes above another, see orderJogs
	over []bool
}

// jog is a horizontal segment of an edge in a channel between ranks
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

// rightward reports whether the jog heads right, entering at its left end
func (j *jog) rightward() bool { return j.xin == j.x0 }

// leftward reports whether the jog heads left, entering at its right end
func (j *jog) leftward() bool { return j.xin == j.x1 }

// routed reports whether ortho routes the edge between ranks
func routed(edge *ledge) bool { return !edge.flat && len(edge.Path) >= 2 }

// stepOut makes ends on side ports step out sideways before turning,
// instead of running along the node outline, and ends on the top or
// bottom facing away from where the edge goes step out and around the
// node, instead of running through it
func stepOut(graph *lgraph, pad Length) {
	offset := map[Compass]Length{West: -pad, East: pad}
	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		if dx := offset[edge.FromPort]; dx != 0 {
			p := edge.Path[0]
			edge.Path = slices.Insert(edge.Path, 1, Vector{p.X + dx, p.Y})
		}
		if dx := offset[edge.ToPort]; dx != 0 {
			p := edge.Path[len(edge.Path)-1]
			edge.Path = slices.Insert(edge.Path, len(edge.Path)-1, Vector{p.X + dx, p.Y})
		}
		if around := aroundNode(edge.From, edge.FromPort, edge.Path[0], edge.Path[1], edge.Path[len(edge.Path)-1], pad); around != nil {
			edge.Path = slices.Insert(edge.Path, 1, around...)
		}
		last := len(edge.Path) - 1
		if around := aroundNode(edge.To, edge.ToPort, edge.Path[last], edge.Path[last-1], edge.Path[0], pad); around != nil {
			slices.Reverse(around)
			edge.Path = slices.Insert(edge.Path, last, around...)
		}
	}
}

// slotEnd is an end of an edge on a side of a node, see slotEnds
type slotEnd struct {
	edge    *ledge
	start   bool
	towards Length
	x       Length // where the end attaches before spreading
}

// nodeSide is the top or the bottom of a node
type nodeSide struct {
	node   *lnode
	bottom bool
}

// slotEnds spreads the edge ends on each node side across the node width
// in the order of where they head, so that stubs neither coincide nor
// cross, and returns the edges along a rank that arc over it
func slotEnds(graph *lgraph, pad Length, pack bool) (arcEnds []*ledge) {
	// the lists by side are kept from the last route, emptied
	ends := graph.ortho.ends
	if ends == nil {
		ends = make(map[nodeSide][]slotEnd, 2*len(graph.Nodes))
		graph.ortho.ends = ends
	}
	for k, list := range ends {
		ends[k] = list[:0]
	}

	// an edge along a rank arcs over it, see the flat edges in layout.go,
	// with both ends on the tops of its nodes, which it shares with the
	// ends of other edges
	arc := func(edge *ledge) bool {
		p := edge.Path
		if !edge.arcShaped() {
			return false
		}
		rises := vertical(p[0], p[1]) && p[1].Y < p[0].Y // straight up from the first node
		lands := vertical(p[2], p[3])                    // straight down onto the second
		return rises && lands
	}
	for _, edge := range graph.Edges {
		if !arc(edge) {
			continue
		}
		arcEnds = append(arcEnds, edge)
		if edge.freeStart() {
			k := nodeSide{edge.From, false}
			ends[k] = append(ends[k], slotEnd{edge, true, edge.Path[2].X, edge.Path[0].X})
		}
		if edge.freeEnd() {
			k := nodeSide{edge.To, false}
			ends[k] = append(ends[k], slotEnd{edge, false, edge.Path[1].X, edge.Path[3].X})
		}
	}

	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		down := edge.Path[0].Y < edge.Path[len(edge.Path)-1].Y
		if edge.freeStart() {
			k := nodeSide{edge.From, down}
			ends[k] = append(ends[k], slotEnd{edge, true, edge.Path[1].X, edge.Path[0].X})
		}
		if edge.freeEnd() {
			k := nodeSide{edge.To, !down}
			ends[k] = append(ends[k], slotEnd{edge, false, edge.Path[len(edge.Path)-2].X, edge.Path[len(edge.Path)-1].X})
		}
	}

	// the edges don't change between routes
	parallel := graph.ortho.parallel
	if parallel == nil {
		parallel = make(map[[2]*lnode]int, len(graph.Edges))
		for _, edge := range graph.Edges {
			parallel[unorderedNodes(edge)]++
		}
		graph.ortho.parallel = parallel
	}
	for k, list := range ends {
		if len(list) > 0 {
			slotSide(graph, k, list, parallel, pad, pack)
		}
	}
	return arcEnds
}

// slotSide moves the ends in list, on side k, to their slots
func slotSide(graph *lgraph, k nodeSide, list []slotEnd, parallel map[[2]*lnode]int, pad Length, pack bool) {
	input := make(map[*ledge]int, len(list))
	for i, e := range list {
		input[e.edge] = i
	}
	slices.SortStableFunc(list, func(a, b slotEnd) int { return cmp.Compare(a.towards, b.towards) })

	// merged ends take one slot a group, that of its middle end
	followers := map[*ledge][]slotEnd{}
	groups := map[int][]slotEnd{}
	var ids []int // the groups in the order of their first end
	list = slices.DeleteFunc(list, func(e slotEnd) bool {
		g := graph.mergeGroup(e.edge, e.start)
		if g != 0 {
			if groups[g] == nil {
				ids = append(ids, g)
			}
			groups[g] = append(groups[g], e)
		}
		return g != 0
	})
	for _, g := range ids {
		group := groups[g]
		mid := group[len(group)/2]
		followers[mid.edge] = slices.Delete(group, len(group)/2, len(group)/2+1)
		list = append(list, mid)
	}

	slices.SortStableFunc(list, func(a, b slotEnd) int { return cmp.Compare(a.towards, b.towards) })

	// edges between the same nodes take the places they sorted to in
	// the order of the edges, the same at both ends; sorted by where
	// earlier routes left them, they could swap and cross between
	pairs := map[[2]*lnode][]int{}
	for i, e := range list {
		if pair := unorderedNodes(e.edge); parallel[pair] > 1 {
			pairs[pair] = append(pairs[pair], i)
		}
	}
	for _, at := range pairs {
		ends := make([]slotEnd, len(at))
		for j, i := range at {
			ends[j] = list[i]
		}
		slices.SortStableFunc(ends, func(a, b slotEnd) int { return cmp.Compare(input[a.edge], input[b.edge]) })
		for j, i := range at {
			list[i] = ends[j]
		}
	}

	// slots evenly spread around the center, or packed from the left
	// an edge padding apart
	n := Length(len(list))
	spacing := min(2*pad, 2*k.node.Radius.X/(n+1))
	center, reach := k.node.Center.X, k.node.Radius.X-spacing/2
	lo, hi := center-reach, center+reach
	slot := func(i int) Length { return center + centeredOffset(i, len(list), spacing) }
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
	// do merged ends, which fork from their slot. In text, edges
	// between the same nodes take slots too, as their middle one runs
	// straight but often between two cells.
	want := make([]Length, len(list))
	weight := make([]float64, len(list))
	for i, e := range list {
		want[i] = slot(i)
		weight[i] = 1
		if graph.ForText && parallel[unorderedNodes(e.edge)] > 1 {
			continue
		}
		if (e.towards-e.x).Abs() < 0.01 && e.towards >= lo && e.towards <= hi && len(followers[e.edge]) == 0 {
			want[i] = e.towards
			weight[i] = 1e9
		}
	}

	// a straight end of edges merged at their other end takes the slot
	// nearest it, and the rest the other slots in order: an end heading
	// past it crosses its line rather than push it off it, where the
	// merged edges run on together and the end would come in along them
	firm := func(i int) bool {
		// merged at the other end
		return weight[i] > 1 && graph.mergeGroup(list[i].edge, !list[i].start) != 0
	}
	at := make([]int, len(list))
	taken := make([]bool, len(list))
	for i := range list {
		if !firm(i) {
			continue
		}
		best := -1
		for j := range list {
			if !taken[j] && (best < 0 || (slot(j)-want[i]).Abs() < (slot(best)-want[i]).Abs()) {
				best = j
			}
		}
		at[i] = best
		taken[best] = true
	}

	j := 0
	for i := range list {
		if !firm(i) {
			for taken[j] {
				j++
			}
			at[i] = j
			taken[j] = true
			if weight[i] == 1 {
				want[i] = slot(j)
			}
		}
	}

	order := make([]int, len(list))
	for i := range order {
		order[at[i]] = i
	}
	o := &graph.ortho
	o.list = permute(o.list, list, order)
	o.want = permute(o.want, want, order)
	o.weight = permute(o.weight, weight, order)
	list = o.list
	want = o.want
	weight = o.weight
	o.slots = separate(o.slots, want, weight, spacing, lo, hi)
	xs := o.slots
	for i, e := range list {
		if weight[i] > 1 && (xs[i]-want[i]).Abs() < spacing/1e3 {
			xs[i] = want[i] // undo rounding, the run must stay exactly vertical
		}
		p := k.node.outlineAt(xs[i], k.bottom)
		for _, f := range append([]slotEnd{e}, followers[e.edge]...) {
			f.edge.setEnd(f.start, p)
		}
	}
}

// outlineAt returns the point on the node outline at x, on its bottom or
// top side
func (node *lnode) outlineAt(x Length, bottom bool) Vector {
	dx := float64(x - node.Center.X)
	dy := float64(node.Radius.Y)
	switch node.Shape {
	case Box, Square, Record, None:
	case Diamond, Octagon:
		dy = float64(node.halfHeightAt(x))
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

// rowSpans are the top and the bottom of each rank, top to bottom; the
// channel k is the space between ranks k and k+1
type rowSpans [][2]Length

// channel returns the top and the bottom of channel k
func (rows rowSpans) channel(k int) (top, bottom Length) { return rows[k][1], rows[k+1][0] }

// channelAt returns the channel that y is above the bottom of, -1 when
// y is at or below the top of the last rank
func (rows rowSpans) channelAt(y Length) int {
	for k := 0; k+1 < len(rows); k++ {
		if y < rows[k+1][0] {
			return k
		}
	}
	return -1
}

// rankAt returns the rank whose row holds y, -1 for none
func (rows rowSpans) rankAt(y Length) int {
	return slices.IndexFunc(rows, func(row [2]Length) bool { return row[0] <= y && y <= row[1] })
}

// shiftFrom moves the rows from rank r on down by d
func (rows rowSpans) shiftFrom(r int, d Length) {
	for ; r < len(rows); r++ {
		rows[r][0], rows[r][1] = rows[r][0]+d, rows[r][1]+d
	}
}

// channelJogs returns the jogs of the routed edges in each channel, the
// one below rows[k] at k
func channelJogs(graph *lgraph, rows rowSpans) [][]*jog {
	channels := slices.Grow(graph.ortho.channels[:0], len(rows))[:len(rows)]
	for k := range channels {
		channels[k] = channels[k][:0]
	}
	graph.ortho.channels = channels

	// the jogs, and where they enter and leave, in blocks; at most one
	// for every segment that isn't vertical, so that the pointers into
	// them stay put
	segments := 0
	for _, edge := range graph.Edges {
		for i := 1; routed(edge) && i < len(edge.Path); i++ {
			if edge.Path[i-1].X != edge.Path[i].X {
				segments++
			}
		}
	}

	block := slices.Grow(graph.ortho.block[:0], segments)
	xs := slices.Grow(graph.ortho.xs[:0], 2*segments)
	graph.ortho.block = block
	graph.ortho.xs = xs
	for _, edge := range graph.Edges {
		if !routed(edge) {
			continue
		}
		path := edge.Path
		for i := 0; i+1 < len(path); i++ {
			a, b := path[i], path[i+1]
			if vertical(a, b) {
				continue
			}
			top, bottom := a, b
			if top.Y > bottom.Y {
				top, bottom = bottom, top
			}
			k := rows.channelAt(top.Y)
			if k < 0 {
				continue // not a rank-to-rank segment
			}
			if _, end := rows.channel(k); end > bottom.Y+0.5 {
				continue
			}

			// each end its own slice, which appending copies out
			xs = append(xs, top.X, bottom.X)
			n := len(xs)
			block = append(block, jog{
				edge: edge, index: i, x0: min(a.X, b.X), x1: max(a.X, b.X), xin: top.X,
				ins: xs[n-2 : n-1 : n-1], outs: xs[n-1 : n : n],
			})
			channels[k] = append(channels[k], &block[len(block)-1])
		}
	}

	return channels
}

// throughRuns returns the x of the vertical runs of edges that pass
// channel k without turning in it
func throughRuns(graph *lgraph, rows rowSpans, k int) []Length {
	top, bottom := rows.channel(k)
	var xs []Length
	for _, edge := range graph.Edges {
		for i := 0; routed(edge) && i+1 < len(edge.Path); i++ {
			if a, b := edge.Path[i], edge.Path[i+1]; vertical(a, b) && min(a.Y, b.Y) <= top && max(a.Y, b.Y) >= bottom {
				xs = append(xs, a.X)
			}
		}
	}
	return xs
}

// arcRoom returns how far the arcs of edges along a rank reach up into
// the bottom of each channel; tracks keep above them, see the flat edges
// in layout.go. In text, the rank of the arcs and what is below moves
// down to make them room.
func arcRoom(graph *lgraph, rows rowSpans) []Length {
	arcs := make([]Length, len(rows))
	over := make([][]*ledge, len(rows)) // the arcs in each channel
	for _, edge := range graph.Edges {
		if !edge.arcShaped() {
			continue
		}

		// the channel above the rank of the edge, wherever the arc reaches
		if y := edge.Path[1].Y; y < edge.Path[0].Y {
			if k := rows.rankAt(edge.From.Center.Y) - 1; k >= 0 {
				_, bottom := rows.channel(k)
				arcs[k] = max(arcs[k], bottom-y)
				over[k] = append(over[k], edge)
			}
		}
	}

	if !graph.ForText {
		return arcs
	}

	// text draws the arcs on rows of their own, below the nodes of the
	// rank above with a row between
	for k := range arcs {
		if arcs[k] == 0 {
			continue
		}
		top, bottom := rows.channel(k)
		if short := top + graph.LineHeight - (bottom - arcs[k]); short > 0 {
			below := top + graph.LineHeight/2
			graph.shiftBelow(below, short)
			for _, edge := range over[k] {
				for i := range edge.Path {
					if edge.Path[i].Y <= below {
						edge.Path[i].Y += short // above the cut, as high as the arc reaches
					}
				}
			}
			rows.shiftFrom(k+1, short)
		}
	}

	return arcs
}

// shareJogs takes the jogs of merged edges out of jogs: they turn along
// one track, the first jog of a group spans the rest, which take its track
// afterwards, as shared lists them
func shareJogs(graph *lgraph, jogs []*jog) (_ []*jog, shared map[*jog][]*jog) {
	shared = map[*jog][]*jog{}
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
	return jogs, shared
}

// splitOpposite splits a rightward jog over the same span as a leftward
// one into two steps that cross the leftward one in the middle, as the
// two would run down each other's stubs on any tracks
func splitOpposite(jogs []*jog) []*jog {
	for _, r := range jogs {
		if !r.rightward() || r.next != nil {
			continue
		}
		for _, l := range jogs {
			if l.leftward() && l.x0 == r.x0 && l.x1 == r.x1 {
				mid := (r.x0 + r.x1) / 2
				r.next = &jog{
					edge: r.edge, index: r.index, x0: mid, x1: r.x1, xin: mid, split: true,
					ins: []Length{mid}, outs: r.outs,
				}
				r.x1 = mid
				r.outs = []Length{mid}
				jogs = append(jogs, r.next)
				break
			}
		}
	}

	return jogs
}

// orderJogs returns the jogs of a channel in the order of their tracks,
// top to bottom: jogs heading right by entry x descending, then jogs
// heading left by entry x ascending, so continuations don't cut through
// the jogs below them
func orderJogs(jogs []*jog, shared map[*jog][]*jog, through func() []Length, pad Length, buf *[]bool) []*jog {
	sortByEntry(jogs)
	o := &jogOrder{jogs: jogs, shared: shared, through: through, pad: pad, buf: buf}
	o.findAbove()
	o.splitCycles()
	order := o.leastCrossings(o.topological())
	return permute(make([]*jog, 0, len(o.jogs)), o.jogs, order)
}

// sortByEntry sorts jogs heading right by entry x descending, then jogs
// heading left by entry x ascending
func sortByEntry(jogs []*jog) {
	slices.SortStableFunc(jogs, func(a, b *jog) int {
		ra, rb := a.rightward(), b.rightward()
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
}

// jogOrder orders the jogs of a channel, see orderJogs. A jog entering
// where another exits goes above it, or the exit would run down its stub;
// otherwise the sorted order stays. Where is within half a pad, as text
// draws closer runs in one cell.
type jogOrder struct {
	jogs   []*jog
	shared map[*jog][]*jog
	// through returns the x of the edges that pass the channel without
	// turning in it, see throughRuns
	through func() []Length
	pad     Length
	buf     *[]bool
	// whether jogs[i] goes above jogs[k], at i*n+k, see findAbove
	above []bool
	exits []jogExit
}

// goesAbove reports whether jogs[i] goes above jogs[k], see findAbove
func (o *jogOrder) goesAbove(i, k int) bool { return o.above[i*len(o.jogs)+k] }

// jogExit is where a jog leaves its channel, see findAbove
type jogExit struct {
	x Length
	k int
}

// near reports whether any of xs and ys are within half a pad
func (o *jogOrder) near(xs, ys []Length) bool {
	for _, x := range xs {
		for _, y := range ys {
			if (x - y).Abs() < o.pad/2 {
				return true
			}
		}
	}

	return false
}

// findAbove finds which jogs go above which, as the loops after ask it
// again and again, in buf: a jog above the second step of its own, and,
// unless it is a second step, above the jogs that end where it enters,
// unless they have a second step. The exits are sorted, so that a jog
// looks only at those around where it enters.
func (o *jogOrder) findAbove() {
	jogs := o.jogs
	pad := o.pad
	n := len(jogs)
	over := slices.Grow((*o.buf)[:0], n*n)[:n*n]
	*o.buf = over
	clear(over)
	o.above = over
	exits := o.exits[:0]
	for k, b := range jogs {
		if b.next == nil {
			for _, y := range b.outs {
				exits = append(exits, jogExit{y, k})
			}
		}
	}

	slices.SortFunc(exits, func(a, b jogExit) int { return cmp.Compare(a.x, b.x) })
	o.exits = exits
	for i, a := range jogs {
		if k := slices.Index(jogs, a.next); a.next != nil && k >= 0 && k != i {
			over[i*n+k] = true
		}
		if a.split {
			continue
		}
		for _, x := range a.ins {
			// a pad either way holds all within half a pad
			lo, _ := slices.BinarySearchFunc(exits, x-pad, func(e jogExit, x Length) int { return cmp.Compare(e.x, x) })
			for _, e := range exits[lo:] {
				if e.x > x+pad {
					break
				}
				if e.k != i && (x-e.x).Abs() < pad/2 {
					over[i*n+e.k] = true
				}
			}
		}
	}
}

// splitCycles splits jogs that must go above each other around a cycle,
// as longer chains and swaps over spans that differ can, and have no
// order: the widest of a cycle splits into two steps, on two tracks, at a
// point where no other jog enters or leaves, nor an edge passes through
// the channel, which the run between the steps would lie along. The
// first step enters as the jog did and the second leaves as it did, so
// nothing needs to go above the first, which breaks the cycle. The jog of
// merged edges stays whole, as the others of its group take its track
// alone, see shareJogs, unless the cycle has no other.
func (o *jogOrder) splitCycles() {
	for range len(o.jogs) {
		cycle := jogCycle(o.jogs, o.goesAbove)
		var widest *jog
		for _, whole := range []bool{true, false} {
			for _, j := range cycle {
				free := !j.split && j.next == nil && (!whole || len(o.shared[j]) == 0)
				if free && (widest == nil || j.x1-j.x0 > widest.x1-widest.x0) {
					widest = j
				}
			}
			if widest != nil {
				break
			}
		}

		if widest == nil {
			return
		}
		through := o.through()
		apart := func(x Length) bool {
			return !o.near(through, []Length{x}) && !slices.ContainsFunc(o.jogs, func(j *jog) bool {
				return o.near(j.ins, []Length{x}) || o.near(j.outs, []Length{x})
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
			return
		}
		span := func(xs []Length) (Length, Length) { return min(mid, slices.Min(xs)), max(mid, slices.Max(xs)) }
		second := &jog{edge: widest.edge, index: widest.index, xin: mid, split: true, ins: []Length{mid}, outs: widest.outs}
		second.x0, second.x1 = span(widest.outs)
		widest.x0, widest.x1 = span(widest.ins)
		widest.next = second
		widest.outs = []Length{mid}
		o.jogs = append(o.jogs, second)
		o.findAbove()
	}
}

// topological returns the jogs in order: each time the first jog that no
// jog left goes above, by how many do
func (o *jogOrder) topological() []int {
	n := len(o.jogs)
	below := make([]int, n)
	for i := range n {
		for k := range n {
			if o.goesAbove(i, k) {
				below[k]++
			}
		}
	}

	left := make([]int, n)
	for i := range left {
		left[i] = i
	}
	order := make([]int, 0, n)
	for len(left) > 0 {
		pick := 0 // on a cycle
		for i, k := range left {
			if below[k] == 0 {
				pick = i
				break
			}
		}

		p := left[pick]
		order = append(order, p)
		left = slices.Delete(left, pick, pick+1)
		for _, k := range left {
			if o.goesAbove(p, k) {
				below[k]--
			}
		}
	}

	return order
}

// leastCrossings moves the jogs of order to where they cross least: a
// jog above another crosses the other's track with its line down where it
// leaves inside the other's span, and the other's line in where it enters
// inside its own. The jog of merged edges spans them all and holds all
// their lines, see shareJogs, so it moves as the group it draws.
func (o *jogOrder) leastCrossings(order []int) []int {
	jogs, pad := o.jogs, o.pad
	inside := func(xs []Length, j *jog) (n int) {
		for _, x := range xs {
			if x > j.x0+pad/2 && x < j.x1-pad/2 {
				n++
			}
		}
		return n
	}

	return siftOrder(order, func(i, k int) int { return inside(jogs[i].outs, jogs[k]) + inside(jogs[k].ins, jogs[i]) },
		func(i, k int) bool { return i != k && o.goesAbove(i, k) })
}

// placeTracks puts the ordered jogs of channel k on tracks, arc being how
// far arcs reach into its bottom, and returns the jogs that share the
// tracks of others, see shareJogs, now placed too. In text the channel
// grows to fit its tracks, and what is below moves down.
func placeTracks(graph *lgraph, rows rowSpans, arc Length, k int, jogs []*jog, shared map[*jog][]*jog, pad Length, pack bool) (placed []*jog) {
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

	top, bottom := rows.channel(k)
	bottom -= arc

	// tracks are a pad apart, or a cell apart in text, a line or a
	// column sideways, where the channel runs across the columns
	step := pad
	if graph.ForText {
		step = graph.cell().Y
	}

	// above arcs, the arrows are below them, so tracks go right above,
	// high in the channel; and next to the top or bottom of a cluster,
	// which text draws on a row of its own, where the arrows or bends
	// are past it, high above a top and low below a bottom
	margin := 3
	high, low := arc > 0, false
	if graph.ForText && !graph.RankDir.Sideways() {
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
			rows.shiftFrom(k+1, short)
			bottom += short
		}
	}
	spacing := min(step, (bottom-top)/Length(tracks+1))
	for i, j := range jogs {
		j.y = (top+bottom)/2 + centeredOffset(track[i], tracks, spacing)
		switch {
		case high:
			j.y = bottom - Length(tracks-track[i])*spacing
		case low:
			j.y = top + Length(track[i]+1)*spacing
		}

		for _, o := range shared[j] {
			o.y = j.y
			placed = append(placed, o)
		}
	}

	return placed
}

// insertJogs inserts the jog points per edge, later segments first so
// that earlier indices stay valid
func insertJogs(graph *lgraph, channels [][]*jog) {
	// the lists by edge are kept from the last route, emptied
	byEdge := graph.ortho.byEdge
	if byEdge == nil {
		byEdge = make(map[*ledge][]*jog, len(graph.Edges))
		graph.ortho.byEdge = byEdge
	}
	for edge, js := range byEdge {
		byEdge[edge] = js[:0]
	}
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
}

// siftOrder moves each item of order to where cost, summed over every
// item above another, is least, keeping an item above another where
// fixed says it must go, until no move lowers the cost. O(n²) a round
func siftOrder[J any](order []J, cost func(above, below J) int, fixed func(above, below J) bool) []J {
	for range len(order) {
		moved := false
		for i := 0; i < len(order); i++ {
			j := order[i]
			rest := slices.Delete(slices.Clone(order), i, i+1)
			// total cost with j at p, and whether p keeps the fixed pairs
			at := make([]int, len(rest)+1)
			for _, o := range rest {
				at[0] += cost(j, o)
			}
			lo, hi := 0, len(rest)
			for p, o := range rest {
				at[p+1] = at[p] - cost(j, o) + cost(o, j)
				if fixed(o, j) {
					lo = p + 1
				}
				if fixed(j, o) && hi == len(rest) {
					hi = p
				}
			}
			best := i
			if i < lo || i > hi {
				continue // on a cycle, see jogCycle
			}
			for p := lo; p <= hi; p++ {
				if at[p] < at[best] {
					best = p
				}
			}
			if best != i {
				order = slices.Insert(rest, best, j)
				moved = true
			}
		}

		if !moved {
			break
		}
	}

	return order
}

// jogCycle returns the jogs around a cycle of jogs that must go above
// each other, as above tells by their indices, or none
func jogCycle[J comparable](jogs []J, above func(i, k int) bool) []J {
	const (
		unseen = iota
		open
		done
	)

	// by index in jogs, which holds each jog once
	state := make([]uint8, len(jogs))
	stack := make([]J, 0, len(jogs))
	cycle := []J(nil)
	var visit func(i int) bool
	visit = func(i int) bool {
		j := jogs[i]
		state[i] = open
		stack = append(stack, j)
		for k, o := range jogs {
			if !above(i, k) {
				continue
			}
			switch state[k] {
			case open:
				cycle = stack[slices.Index(stack, o):]
				return true
			case unseen:
				if visit(k) {
					return true
				}
			}
		}

		stack = stack[:len(stack)-1]
		state[i] = done
		return false
	}

	for i := range jogs {
		if state[i] == unseen && visit(i) {
			return cycle
		}
	}
	return nil
}

// permute returns the items in the order of the indices, in out
func permute[T any](out, items []T, order []int) []T {
	out = out[:0]
	for _, j := range order {
		out = append(out, items[j])
	}
	return out
}

// separate moves the ordered positions want as little as possible, by
// weighted squared distance, so that neighbors are at least gap apart and
// all lie within [lo, hi]. Shifting position i by i*gap turns the gaps
// into an order constraint, solved by pooling adjacent violators. The
// positions go in xs.
func separate(xs, want []Length, weight []float64, gap, lo, hi Length) []Length {
	type block struct {
		sum, weight float64
		n           int
	}

	// most sides have few ends
	var buf [16]block
	blocks := buf[:0]
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

	xs = xs[:0]
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
// targets of the other. Ends at different fields of a record stay
// apart. Flat edges and loops don't merge, nor do edges with labels,
// which could end up beside a stretch they share.
func mergeEdges(merged map[*ledge][2]int, edges []*ledge, rank func(*lnode) int) {
	type look struct {
		head, tail Arrow
		style      LineStyle
		width      Length
		color      [5]uint8
	}
	type group struct {
		node         *lnode
		field        string // ends at different fields stay apart
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
		k := [2]group{{e.From, e.FromField, below, true, l}, {e.To, e.ToField, !below, false, l}}
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
				bottom = min(bottom, frame)
				lower = true
			default:
				top = max(top, frame)
				upper = true
			}
		}
	}

	return top, bottom, lower, upper
}

// unorderedNodes returns the nodes of edge in an order that is the same
// both ways
func unorderedNodes(edge *ledge) [2]*lnode {
	if edge.From.ID > edge.To.ID {
		return [2]*lnode{edge.To, edge.From}
	}
	return [2]*lnode{edge.From, edge.To}
}

// aroundNode returns the points from the end at a port on the top or
// bottom of node out past it, beside the node, toward next, and on past
// its other side, when the edge goes to far on that side; nil when it
// goes the way it faces. Stopping beside the node would leave the jog
// toward next on the side the port faces, and the line from there would
// run back through the node.
func aroundNode(node *lnode, port Compass, end, next, far Vector, pad Length) []Vector {
	var y, past Length
	switch {
	case (port == North || port == NorthEast || port == NorthWest) && far.Y > node.Center.Y:
		y, past = node.Top()-pad, node.Bottom()+pad
	case (port == South || port == SouthEast || port == SouthWest) && far.Y < node.Center.Y:
		y, past = node.Bottom()+pad, node.Top()-pad
	default:
		return nil
	}

	// a pad further out than the ends at side ports step out, see
	// stepOut, which turn where they did
	x := node.Right() + 2*pad
	if next.X < node.Center.X {
		x = node.Left() - 2*pad
	}
	return []Vector{{end.X, y}, {x, y}, {x, past}}
}
