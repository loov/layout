package layout

import (
	"math"
	"slices"

	"github.com/loov/layout/internal/hier"
)

// straightenNodes moves a node along its rank in line with a node it has
// an edge to, and pushes the ones beside it as far as they have to go,
// where the edges that route draws then bend less and cross no more;
// positioning can't tell which edges end up straight. The measure
// doesn't look at labels, so the caller places them after.
func straightenNodes(graph *hier.Graph, graphdef *lgraph, route func()) {
	st := &straightener{graph: graph, graphdef: graphdef, cell: float32(graphdef.cell().X)}
	// the nodes a node has edges to before or after it, see neighbors
	st.real = make([]bool, len(graph.ByRank))
	for r, layer := range graph.ByRank {
		st.real[r] = slices.ContainsFunc(layer, func(node *hier.Node) bool { return !node.Virtual })
	}
	wide := st.width()
	bends := st.countBends()
	crossings, overlaps := st.measure()
	// the node of the last move kept; the nodes after it were tried
	// where they are now, so after a pass they would only fail again
	last := len(graph.Nodes)
	for pass := range 4 {
		improved := false
		for idx, node := range graph.Nodes {
			if pass > 0 && !improved && idx > last {
				break
			}
			if node.Virtual || st.fork(node) {
				continue
			}
			for _, want := range st.wants(node) {
				if math.Abs(float64(want-node.Center.X)) < 0.5 {
					continue
				}
				before := st.at()
				if !st.moves(node, want, wide, before) {
					continue
				}
				route()
				if b := st.countBends(); b < bends {
					if c, o := st.measure(); c <= crossings && o <= overlaps {
						bends, crossings, overlaps, improved = b, c, o, true
						last = idx
						break
					}
				}
				st.restore(before)
			}
		}
		if !improved {
			break
		}
	}
}

// straightener tries moves of nodes along their ranks, see
// straightenNodes. Its methods fill buffers kept across the tries, as
// each try runs them over every node and edge, and their results are used
// up before they run again.
type straightener struct {
	graph    *hier.Graph
	graphdef *lgraph
	// cell is along the ranks, a column, or a row sideways
	cell float32
	// whether a rank has nodes that aren't virtual, see neighbors
	real []bool

	horizontal, vertical []axisRun
	xs, wanted           []float32
	adjacent, kept       []*hier.Node
}

// axisRun is a run of an edge at a coordinate across it, from lo to hi
// along it
type axisRun struct {
	at, lo, hi Length
	edge       *ledge
}

// byStart orders runs by where they are, then where they start, with
// plain comparisons, as there are no NaNs, which sort faster than
// cmp.Compare
func byStart(a, b axisRun) int {
	switch {
	case a.at < b.at:
		return -1
	case a.at > b.at:
		return 1
	case a.lo < b.lo:
		return -1
	case a.lo > b.lo:
		return 1
	}
	return 0
}

// merged reports whether a and b share a run, see Graph.MergeEdges
func (st *straightener) merged(a, b *ledge) bool {
	ma, mb := st.graphdef.merged[a], st.graphdef.merged[b]
	return ma[0] != 0 && ma[0] == mb[0] || ma[1] != 0 && ma[1] == mb[1]
}

// countBends counts the bends of the edges: a move must take out bends,
// which are quick to count, before the crossings and overlaps, which
// aren't, are looked at
func (st *straightener) countBends() (bends int) {
	for _, edge := range st.graphdef.Edges {
		for i := 2; i < len(edge.Path); i++ {
			a, b, c := edge.Path[i-2], edge.Path[i-1], edge.Path[i]
			if (a.X == b.X) != (b.X == c.X) {
				bends++
			}
		}
	}
	return bends
}

// measure counts the crossings of the edges, and the runs along each
// other or along the frame of a cluster
func (st *straightener) measure() (crossings, overlaps int) {
	graphdef := st.graphdef
	horizontal, vertical := st.horizontal[:0], st.vertical[:0]
	for _, edge := range graphdef.Edges {
		for i := 1; i < len(edge.Path); i++ {
			a, b := edge.Path[i-1], edge.Path[i]
			switch {
			case a.Y == b.Y && a.X != b.X:
				horizontal = append(horizontal, axisRun{a.Y, min(a.X, b.X), max(a.X, b.X), edge})
			case a.X == b.X && a.Y != b.Y:
				vertical = append(vertical, axisRun{a.X, min(a.Y, b.Y), max(a.Y, b.Y), edge})
			}
		}
	}
	st.horizontal, st.vertical = horizontal, vertical
	// sorted, a run meets only the runs that start within its span
	slices.SortFunc(horizontal, byStart)
	slices.SortFunc(vertical, byStart)
	for _, h := range horizontal {
		i, _ := slices.BinarySearchFunc(vertical, h.lo, func(v axisRun, x Length) int {
			if v.at <= x {
				return -1
			}
			return 1
		})
		for _, v := range vertical[i:] {
			if v.at >= h.hi {
				break
			}
			if h.edge != v.edge && v.lo < h.at && h.at < v.hi {
				crossings++
			}
		}
	}
	// runs of different edges along each other
	for _, runs := range [][]axisRun{horizontal, vertical} {
		for i, r := range runs {
			for _, q := range runs[i+1:] {
				if q.at != r.at || q.lo >= r.hi {
					break
				}
				// merged looks up maps, so it goes last
				if r.edge != q.edge && !st.merged(r.edge, q.edge) {
					overlaps++
				}
			}
		}
	}
	// runs along the frame of a cluster, within half a cell of it, which
	// text draws on it
	cell := graphdef.cell()
	near := Vector{X: cell.X / 2, Y: cell.Y / 2}
	for _, cluster := range graphdef.Clusters {
		tl, br := cluster.TopLeft, cluster.BottomRight
		for _, h := range horizontal {
			if ((h.at-tl.Y).Abs() <= near.Y || (h.at-br.Y).Abs() <= near.Y) && min(h.hi, br.X)-max(h.lo, tl.X) > near.X {
				overlaps++
			}
		}
		for _, v := range vertical {
			if ((v.at-tl.X).Abs() <= near.X || (v.at-br.X).Abs() <= near.X) && min(v.hi, br.Y)-max(v.lo, tl.Y) > near.Y {
				overlaps++
			}
		}
	}
	return crossings, overlaps
}

func (st *straightener) gap(a, b *hier.Node) float32 {
	return a.Radius.X + b.Radius.X + st.graph.Apart(a, b)
}

// at returns where the nodes are along their ranks
func (st *straightener) at() []float32 {
	nodes := st.graph.Nodes
	st.xs = slices.Grow(st.xs[:0], len(nodes))[:len(nodes)]
	for i, node := range nodes {
		st.xs[i] = node.Center.X
	}
	return st.xs
}

// restore puts the nodes back where at had them
func (st *straightener) restore(xs []float32) {
	for i, x := range xs {
		st.graph.Nodes[i].Center.X = x
	}
}

// neighbors returns the nodes node has edges to before or after it, past
// ranks of labels only, along edges that merge into a fork, as
// positioning centers fans; not long edges past other nodes, nor edges
// with labels
func (st *straightener) neighbors(node *hier.Node, before bool) []*hier.Node {
	out := st.adjacent[:0]
	next := node.Out
	if before {
		next = node.In
	}
	for _, n := range next {
		for n.Virtual && !n.Carrier && !st.real[n.Rank] {
			ends := n.Out
			if before {
				ends = n.In
			}
			if len(ends) != 1 {
				break
			}
			n = ends[0]
		}
		if !n.Virtual || n.Carrier && !st.graphdef.MergeEdges {
			out = append(out, n)
		}
	}
	st.adjacent = out
	return out
}

// balanced reports whether node is centered over its children, or under
// its parents, which stays: a balanced fork is worth its bends
func (st *straightener) balanced(node *hier.Node) bool {
	for _, before := range []bool{true, false} {
		ends := st.neighbors(node, before)
		lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, other := range ends {
			lo, hi = min(lo, other.Center.X+other.Anchor), max(hi, other.Center.X+other.Anchor)
		}
		if len(ends) >= 2 && math.Abs(float64(node.Center.X+node.Anchor-(lo+hi)/2)) < 1 {
			return true
		}
	}
	return false
}

// fork reports whether node is a fork or a join, which stays where
// positioning put it, among its edges
func (st *straightener) fork(node *hier.Node) bool {
	return !node.Virtual && (len(st.neighbors(node, true)) >= 2 || len(st.neighbors(node, false)) >= 2)
}

// straight reports whether the edge between node and other is straight
// already: ends slide along a node in text, so an edge between nodes that
// overlap across the rank, one end alone on its side, is
func (st *straightener) straight(node, other *hier.Node) bool {
	if node.Virtual || other.Virtual {
		return false
	}
	alone := len(node.Out) == 1 || len(other.In) == 1
	if !slices.Contains(node.Out, other) {
		alone = len(node.In) == 1 || len(other.Out) == 1
	}
	lo, hi := max(node.Center.X-node.Radius.X, other.Center.X-other.Radius.X), min(node.Center.X+node.Radius.X, other.Center.X+other.Radius.X)
	return alone && hi-lo > 2*st.cell
}

// wants returns where node could go: in line with a node it has an edge
// to, or a cell or two over, where ends that take slots along the node
// line up
func (st *straightener) wants(node *hier.Node) []float32 {
	wants := st.wanted[:0]
	for _, others := range [2][]*hier.Node{node.In, node.Out} {
		for _, other := range others {
			if !st.straight(node, other) {
				wants = append(wants, other.Center.X+other.Anchor-node.Anchor)
			}
		}
	}
	for _, k := range []float32{1, -1, 2, -2} {
		wants = append(wants, node.Center.X+k*st.cell)
	}
	st.wanted = wants
	return wants
}

// move puts node at x and pushes the nodes beside it as far as they
// have to go
func (st *straightener) move(node *hier.Node, x float32) {
	layer := st.graph.ByRank[node.Rank]
	i := slices.Index(layer, node)
	node.Center.X = x
	for j := i + 1; j < len(layer); j++ {
		layer[j].Center.X = max(layer[j].Center.X, layer[j-1].Center.X+st.gap(layer[j-1], layer[j]))
	}
	for j := i - 1; j >= 0; j-- {
		layer[j].Center.X = min(layer[j].Center.X, layer[j+1].Center.X-st.gap(layer[j], layer[j+1]))
	}
}

// moves moves node to x, see move, and reports whether it may stay
// there: the drawing doesn't widen past wide for it, balanced nodes stay
// balanced and forks stay put; otherwise the nodes go back to before
func (st *straightener) moves(node *hier.Node, x, wide float32, before []float32) bool {
	kept := st.kept[:0]
	for _, n := range st.graph.Nodes {
		if !n.Virtual && st.balanced(n) {
			kept = append(kept, n)
		}
	}
	st.kept = kept
	st.move(node, x)
	if st.width() > wide+0.5 || slices.ContainsFunc(kept, func(n *hier.Node) bool { return !st.balanced(n) }) ||
		slices.ContainsFunc(st.graph.Nodes, func(n *hier.Node) bool { return st.fork(n) && n.Center.X != before[n.ID] }) {
		st.restore(before)
		return false
	}
	return true
}

// width returns how wide the drawing is
func (st *straightener) width() float32 {
	lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
	for _, node := range st.graph.Nodes {
		lo, hi = min(lo, node.Center.X-node.Radius.X), max(hi, node.Center.X+node.Radius.X)
	}
	return hi - lo
}
