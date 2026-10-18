package hier

import (
	"cmp"
	"math/bits"
	"slices"
)

// maxGroup bounds the nodes of a group: every node of the graph tries its
// groups, and each try counts the crossings of the ranks it spans, so
// bigger groups make a pass quadratic in the size of the graph
const maxGroup = 32

// OrderRanksGroups moves the descendants or the ancestors of a node across
// all their ranks at once: it mirrors them in place, or swaps them with
// those of the node beside it. Sweeps and blocks move a node, or a column
// with one node a rank, at a time, and a subtree on the wrong side of
// another crosses it in every rank wherever any one of its nodes goes.
// A move stays when it crosses less. Groups start at real nodes, as a
// virtual node's mostly repeat its edge's ends, and are mirrored from
// where they fork.
func OrderRanksGroups(graph *Graph) (moves int) {
	if len(graph.Clusters) > 0 {
		// a group would have to stay inside its clusters
		return 0
	}
	graph.assignPos()
	n := len(graph.Nodes)
	g := &groupMover{
		graph: graph,
		in:    make([]int32, n), left: make([]int32, n),
		from: make([]int32, n), moved: make([]int32, n),
		ranked:    make([]Nodes, len(graph.ByRank)),
		tables:    make([]pairTable, len(graph.ByRank)),
		pairEdges: make([]int, len(graph.ByRank)),
	}
	for r, layer := range graph.ByRank {
		for _, node := range layer {
			g.from[node.ID] = int32(node.Pos)
			g.pairEdges[r] += len(node.Out)
		}
	}
	for _, layer := range graph.ByRank {
		for _, v := range slices.Clone(layer) {
			if v.Virtual {
				continue
			}
			for _, down := range [2]bool{true, false} {
				if len(v.Adjacent(!down)) >= 2 && g.mirror(v, down) {
					moves++
				}
				if v.Pos > 0 && g.swap(graph.ByRank[v.Rank][v.Pos-1], v, down) {
					moves++
				}
			}
		}
	}
	return moves
}

// groupMover moves groups, see OrderRanksGroups, reusing its buffers
type groupMover struct {
	graph *Graph
	// in marks the nodes of the groups by epoch
	in    []int32
	epoch int32

	// left marks the left group of a swap by epoch
	left []int32

	a, b, pair Nodes
	// the ranks a move changes, before and after, see try
	changed [][2]Nodes

	slots []int

	// from is where every node is before a move; moved marks the nodes a
	// move moves by tick, and ranked holds them by rank
	from   []int32
	moved  []int32
	tick   int32
	ranked []Nodes
	// tables index the edges between every rank and the next, as they
	// are before a move, see table; pairEdges counts them
	tables    []pairTable
	pairEdges []int
	edges     []groupEdge
}

// group returns in buf v and the nodes it reaches down or up, and
// whether they can move: none is a cluster border, and they are at most
// maxGroup
func (g *groupMover) group(v *Node, down bool, buf Nodes) (Nodes, bool) {
	g.epoch++
	buf = append(buf[:0], v)
	g.in[v.ID] = g.epoch
	for i := 0; i < len(buf); i++ {
		if buf[i].IsBorder() {
			return buf, false
		}
		for _, w := range buf[i].Adjacent(!down) {
			if g.in[w.ID] != g.epoch {
				g.in[w.ID] = g.epoch
				buf = append(buf, w)
				if len(buf) > maxGroup {
					return buf, false
				}
			}
		}
	}
	return buf, true
}

// mirror reverses the order of the group of v in every rank, keeping
// the places it takes
func (g *groupMover) mirror(v *Node, down bool) bool {
	var ok bool
	if g.a, ok = g.group(v, down, g.a); !ok || len(g.a) < 2 {
		return false
	}
	return g.try(g.a, func(rank Nodes) {
		slices.Reverse(rank)
	})
}

// swap puts the group of v, the right one, before that of u, the left
// one, in every rank, keeping the places they take; nodes in both stay
// where they are
func (g *groupMover) swap(u, v *Node, down bool) bool {
	var ok bool
	if g.a, ok = g.group(u, down, g.a); !ok {
		return false
	}
	left := g.epoch
	for _, w := range g.a {
		g.left[w.ID] = left
	}
	if g.b, ok = g.group(v, down, g.b); !ok {
		return false
	}
	right := g.epoch

	moved := g.pair[:0]
	for _, w := range g.a {
		if g.in[w.ID] != right {
			moved = append(moved, w)
		}
	}
	for _, w := range g.b {
		if g.left[w.ID] != left {
			moved = append(moved, w)
		}
	}
	g.pair = moved
	if len(moved) < 2 {
		return false
	}
	return g.try(moved, func(rank Nodes) {
		// the right group's nodes first, both in their order
		slices.SortStableFunc(rank, func(a, b *Node) int {
			return g.side(a, left) - g.side(b, left)
		})
	})
}

// side is 1 for the nodes of the left group of a swap, and 0 for the right
func (g *groupMover) side(w *Node, left int32) int {
	if g.left[w.ID] == left {
		return 1
	}
	return 0
}

// try reorders the nodes of group in every rank by reorder, given them in
// their order there, and keeps that when it crosses less. Only the ranks
// it changes are counted, around them.
func (g *groupMover) try(group Nodes, reorder func(rank Nodes)) bool {
	graph := g.graph
	slices.SortFunc(group, func(a, b *Node) int {
		if a.Rank != b.Rank {
			return a.Rank - b.Rank
		}
		return a.Pos - b.Pos
	})

	// the new order of the changed ranks, after their old one
	g.changed = g.changed[:0]
	for start := 0; start < len(group); {
		end := start + 1
		for end < len(group) && group[end].Rank == group[start].Rank {
			end++
		}
		rank := group[start:end]
		start = end
		g.slots = g.slots[:0]
		for _, w := range rank {
			g.slots = append(g.slots, w.Pos)
		}
		reorder(rank)
		same := true
		for i, w := range rank {
			same = same && w.Pos == g.slots[i]
		}
		if same {
			continue
		}
		layer := graph.ByRank[rank[0].Rank]
		k := len(g.changed)
		if k < cap(g.changed) {
			g.changed = g.changed[:k+1]
		} else {
			g.changed = append(g.changed, [2]Nodes{})
		}
		old := append(g.changed[k][0][:0], layer...)
		for i, w := range rank {
			layer[g.slots[i]] = w
		}
		g.changed[k] = [2]Nodes{old, append(g.changed[k][1][:0], layer...)}
	}
	if len(g.changed) == 0 {
		return false
	}

	set := func(i int) {
		for _, c := range g.changed {
			layer := graph.ByRank[c[i][0].Rank]
			copy(layer, c[i])
			layer.assignPos()
		}
	}

	// a move changes the crossings of the edges at the nodes it moves
	// only; when those are few against the edges around them, counting
	// just their change is cheaper than counting all again
	set(1)
	g.tick++
	touched := 0
	for _, c := range g.changed {
		r := c[1][0].Rank
		for _, w := range c[1] {
			if int32(w.Pos) != g.from[w.ID] {
				g.moved[w.ID] = g.tick
				g.ranked[r] = append(g.ranked[r], w)
				touched += len(w.In) + len(w.Out)
			}
		}
	}
	lo := max(g.changed[0][0][0].Rank-1, 0)
	hi := min(g.changed[len(g.changed)-1][0][0].Rank+1, len(graph.ByRank)-1)
	around := 0
	for r := lo; r < hi; r++ {
		around += g.pairEdges[r]
	}

	var better bool
	if touched*deltaRatio < around {
		better = g.delta() < 0
	} else {
		after := graph.rangeCrossings(lo, hi)
		set(0)
		before := graph.rangeCrossings(lo, hi)
		set(1)
		better = after < before
	}

	for _, c := range g.changed {
		r := c[1][0].Rank
		g.ranked[r] = g.ranked[r][:0]
		if !better {
			continue
		}
		for _, w := range c[1] {
			g.from[w.ID] = int32(w.Pos)
		}
		g.tables[r].valid = false
		if r > 0 {
			g.tables[r-1].valid = false
		}
	}
	if !better {
		set(0)
	}
	return better
}

// deltaRatio is how many times as many edges as a move moves the ranks
// around it have before its change is counted alone, see try: counting
// an edge alone takes a search in a table, and the moved edges count
// against each other too. Tests set it to count either way.
var deltaRatio = 16

// delta returns how many more crossings the ranks have after a move than
// before, see try
func (g *groupMover) delta() float32 {
	graph := g.graph
	delta := float32(0)
	last := -1
	for _, c := range g.changed {
		r := c[0][0].Rank
		for _, upper := range [2]int{r - 1, r} {
			if upper > last && upper >= 0 && upper+1 < len(graph.ByRank) {
				last = upper
				delta += g.pairDelta(upper)
			}
		}
	}

	// a flat edge passes over the nodes between its ends
	for _, edge := range graph.Flat {
		a, b := edge[0], edge[1]
		if g.moved[a.ID] == g.tick || g.moved[b.ID] == g.tick {
			was := max(g.from[a.ID], g.from[b.ID]) - min(g.from[a.ID], g.from[b.ID]) - 1
			delta += flatPassOver(edge) - float32(was)
		}
	}
	return delta
}

// groupEdge is an edge at a moved node: where it ends before a move and
// after, and its weight
type groupEdge struct {
	a0, b0, a1, b1 float32
	weight         float32
}

// pairDelta returns how many more crossings the edges between rank r and
// the next have after a move than before. Only the edges at the moved
// nodes change: they cross the edges as they were, see table, less
// themselves as they were, and each other as they are.
func (g *groupMover) pairDelta(r int) float32 {
	graph := g.graph
	edges := g.edges[:0]
	add := func(u, v *Node) {
		from, to := graph.ports(u, v)
		edges = append(edges, groupEdge{
			a0: float32(g.from[u.ID]) + from, b0: float32(g.from[v.ID]) + to,
			a1: float32(u.Pos) + from, b1: float32(v.Pos) + to,
			weight: graph.Weight(u, v),
		})
	}
	for _, u := range g.ranked[r] {
		for _, v := range u.Out {
			add(u, v)
		}
	}
	for _, v := range g.ranked[r+1] {
		for _, u := range v.In {
			if g.moved[u.ID] != g.tick {
				add(u, v)
			}
		}
	}
	g.edges = edges
	if len(edges) == 0 {
		return 0
	}

	t := g.table(r)
	cross := func(a, b, c, d float32) bool { return a < c && b > d || a > c && b < d }
	delta := float32(0)
	for i, e := range edges {
		delta += e.weight * (t.crossing(e.a1, e.b1) - t.crossing(e.a0, e.b0))
		for k, f := range edges {
			w := e.weight * f.weight
			if cross(e.a1, e.b1, f.a0, f.b0) {
				delta -= w
			}
			if cross(e.a0, e.b0, f.a0, f.b0) {
				delta += w
			}
			if k > i {
				if cross(e.a1, e.b1, f.a1, f.b1) {
					delta += w
				}
				if cross(e.a0, e.b0, f.a0, f.b0) {
					delta -= w
				}
			}
		}
	}
	return delta
}

// pairTable indexes the edges between a rank and the next by where they
// end, to count those that cross an edge. The edges go in the order of
// their upper ends; a Fenwick tree over them finds any first i of them as
// a few aligned blocks, and each block keeps its lower ends sorted, as
// the levels of a bottom-up merge sort do.
type pairTable struct {
	valid bool
	upper []float32 // the upper ends, sorted
	// levels[l] holds the lower ends in blocks of 1<<l edges, each block
	// sorted and summing the weights from its start
	levels [][]pairEnd
	order  []int // scratch: the edges by upper end
}

// pairEnd is the lower end of an edge, its weight, and the weight of the
// edges up to it in its block
type pairEnd struct{ at, weight, sum float32 }

// below returns the weight of the ends of a sorted block before b, or
// with or up to b
func below(block []pairEnd, b float32, or bool) float32 {
	i, _ := slices.BinarySearchFunc(block, b, func(e pairEnd, b float32) int {
		if e.at < b || or && e.at == b {
			return -1
		}
		return 1
	})
	if i == 0 {
		return 0
	}
	return block[i-1].sum
}

// prefix returns the weight of the first i edges by upper end that end
// before b below, or with or up to b, and of all of them
func (t *pairTable) prefix(i int, b float32, or bool) (part, all float32) {
	for k := i; k > 0; k -= k & -k {
		size := k & -k
		block := t.levels[bits.TrailingZeros(uint(size))][k-size : k]
		part += below(block, b, or)
		all += block[size-1].sum
	}
	return part, all
}

// crossing returns the weight of the edges that cross an edge from a to
// b: left of it above and right below, or right above and left below
func (t *pairTable) crossing(a, b float32) float32 {
	lt, _ := slices.BinarySearch(t.upper, a)
	le := lt
	for le < len(t.upper) && t.upper[le] == a {
		le++
	}
	upTo, left := t.prefix(lt, b, true)
	before, _ := t.prefix(le, b, false)
	all, _ := t.prefix(len(t.upper), b, false)
	return left - upTo + all - before
}

// table returns the index of the edges between rank r and the next, as
// they are before a move
func (g *groupMover) table(r int) *pairTable {
	t := &g.tables[r]
	if t.valid {
		return t
	}
	graph := g.graph
	if len(t.levels) == 0 {
		t.levels = make([][]pairEnd, 1)
	}

	// level 0: the lower ends by upper end, each edge a block
	t.upper = t.upper[:0]
	ends := t.levels[0][:0]
	for _, u := range graph.ByRank[r] {
		for _, v := range u.Out {
			from, to := graph.ports(u, v)
			t.upper = append(t.upper, float32(g.from[u.ID])+from)
			w := graph.Weight(u, v)
			ends = append(ends, pairEnd{float32(g.from[v.ID]) + to, w, w})
		}
	}
	n := len(ends)
	t.order = resize(t.order, n)
	for i := range t.order {
		t.order[i] = i
	}
	slices.SortStableFunc(t.order, func(x, y int) int { return cmp.Compare(t.upper[x], t.upper[y]) })
	// the edges leave the nodes of the rank in order, sorted already
	// unless ports put an edge before one of the node before, or the
	// rank holds a move being tried, as it does when a try builds this
	if !slices.IsSorted(t.order) {
		sorted := make([]pairEnd, n)
		for i, k := range t.order {
			sorted[i] = ends[k]
		}
		copy(ends, sorted)
	}
	t.levels[0] = ends
	slices.Sort(t.upper)

	// each level above merges the blocks of the one below in pairs
	l := 1
	for size := 2; size <= n; size, l = size*2, l+1 {
		if l == len(t.levels) {
			t.levels = append(t.levels, nil)
		}
		lower, out := t.levels[l-1], resize(t.levels[l], n/size*size)
		for start := 0; start+size <= n; start += size {
			x, y := lower[start:start+size/2], lower[start+size/2:start+size]
			i, j := 0, 0
			sum := float32(0)
			for k := start; k < start+size; k++ {
				var e pairEnd
				if j == len(y) || i < len(x) && x[i].at <= y[j].at {
					e, i = x[i], i+1
				} else {
					e, j = y[j], j+1
				}
				sum += e.weight
				e.sum = sum
				out[k] = e
			}
		}
		t.levels[l] = out
	}
	t.levels = t.levels[:l]
	t.valid = true
	return t
}
