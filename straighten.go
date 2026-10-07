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
	// merged edges share a run, see Graph.MergeEdges
	merged := func(a, b *ledge) bool {
		ma, mb := graphdef.merged[a], graphdef.merged[b]
		return ma[0] != 0 && ma[0] == mb[0] || ma[1] != 0 && ma[1] == mb[1]
	}
	// the helpers below fill buffers kept across the tries, as each try
	// runs them over every node and edge, and their results are used up
	// before they run again
	// a run of an edge at a coordinate across it, from lo to hi along it
	type segment struct {
		at, lo, hi Length
		edge       *ledge
	}
	var horizontal, vertical []segment
	var xs, wants []float32
	var adjacent, kept []*hier.Node
	// plain comparisons, as there are no NaNs, sort faster than cmp.Compare
	byStart := func(a, b segment) int {
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
	measure := func() (bends, crossings, overlaps int) {
		horizontal, vertical = horizontal[:0], vertical[:0]
		for _, edge := range graphdef.Edges {
			for i := 1; i < len(edge.Path); i++ {
				a, b := edge.Path[i-1], edge.Path[i]
				switch {
				case a.Y == b.Y && a.X != b.X:
					horizontal = append(horizontal, segment{a.Y, min(a.X, b.X), max(a.X, b.X), edge})
				case a.X == b.X && a.Y != b.Y:
					vertical = append(vertical, segment{a.X, min(a.Y, b.Y), max(a.Y, b.Y), edge})
				}
				if i+1 < len(edge.Path) {
					c := edge.Path[i+1]
					if (a.X == b.X) != (b.X == c.X) {
						bends++
					}
				}
			}
		}
		// sorted, a run meets only the runs that start within its span
		slices.SortFunc(horizontal, byStart)
		slices.SortFunc(vertical, byStart)
		for _, h := range horizontal {
			i, _ := slices.BinarySearchFunc(vertical, h.lo, func(v segment, x Length) int {
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
		for _, runs := range [][]segment{horizontal, vertical} {
			for i, r := range runs {
				for _, q := range runs[i+1:] {
					if q.at != r.at || q.lo >= r.hi {
						break
					}
					// merged looks up maps, so it goes last
					if r.edge != q.edge && !merged(r.edge, q.edge) {
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
				if (absLength(h.at-tl.Y) <= near.Y || absLength(h.at-br.Y) <= near.Y) && min(h.hi, br.X)-max(h.lo, tl.X) > near.X {
					overlaps++
				}
			}
			for _, v := range vertical {
				if (absLength(v.at-tl.X) <= near.X || absLength(v.at-br.X) <= near.X) && min(v.hi, br.Y)-max(v.lo, tl.Y) > near.Y {
					overlaps++
				}
			}
		}
		return bends, crossings, overlaps
	}
	gap := func(a, b *hier.Node) float32 { return a.Radius.X + b.Radius.X + graph.Apart(a, b) }
	at := func() []float32 {
		xs = slices.Grow(xs[:0], len(graph.Nodes))[:len(graph.Nodes)]
		for i, node := range graph.Nodes {
			xs[i] = node.Center.X
		}
		return xs
	}
	// the nodes a node has edges to before or after it, past ranks of
	// labels only, along edges that merge into a fork, as positioning
	// centers fans; not long edges past other nodes, nor edges with labels
	real := make([]bool, len(graph.ByRank))
	for r, layer := range graph.ByRank {
		real[r] = slices.ContainsFunc(layer, func(node *hier.Node) bool { return !node.Virtual })
	}
	neighbors := func(node *hier.Node, before bool) []*hier.Node {
		out := adjacent[:0]
		next := node.Out
		if before {
			next = node.In
		}
		for _, n := range next {
			for n.Virtual && !n.Carrier && !real[n.Rank] {
				ends := n.Out
				if before {
					ends = n.In
				}
				if len(ends) != 1 {
					break
				}
				n = ends[0]
			}
			if !n.Virtual || n.Carrier && !graphdef.MergeEdges {
				out = append(out, n)
			}
		}
		adjacent = out
		return out
	}
	// a node centered over its children, or under its parents, stays: a
	// balanced fork is worth its bends
	balanced := func(node *hier.Node) bool {
		for _, before := range []bool{true, false} {
			ends := neighbors(node, before)
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
	// forks and joins stay where positioning put them, among their edges
	fork := func(node *hier.Node) bool {
		return !node.Virtual && (len(neighbors(node, true)) >= 2 || len(neighbors(node, false)) >= 2)
	}
	// ends slide along a node in text, so an edge between nodes that
	// overlap across the rank, one end alone on its side, is straight
	// already
	straight := func(node, other *hier.Node) bool {
		if node.Virtual || other.Virtual {
			return false
		}
		alone := len(node.Out) == 1 || len(other.In) == 1
		if !slices.Contains(node.Out, other) {
			alone = len(node.In) == 1 || len(other.Out) == 1
		}
		cell := float32(graphdef.cellWidth())
		lo, hi := max(node.Center.X-node.Radius.X, other.Center.X-other.Radius.X), min(node.Center.X+node.Radius.X, other.Center.X+other.Radius.X)
		return alone && hi-lo > 2*cell
	}
	move := func(node *hier.Node, x float32) {
		layer := graph.ByRank[node.Rank]
		i := slices.Index(layer, node)
		node.Center.X = x
		for j := i + 1; j < len(layer); j++ {
			layer[j].Center.X = max(layer[j].Center.X, layer[j-1].Center.X+gap(layer[j-1], layer[j]))
		}
		for j := i - 1; j >= 0; j-- {
			layer[j].Center.X = min(layer[j].Center.X, layer[j+1].Center.X-gap(layer[j], layer[j+1]))
		}
	}
	// the drawing doesn't widen for it
	width := func() float32 {
		lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, node := range graph.Nodes {
			lo, hi = min(lo, node.Center.X-node.Radius.X), max(hi, node.Center.X+node.Radius.X)
		}
		return hi - lo
	}
	wide := width()
	// a cell along the ranks: a column, or a row sideways
	cell := float32(graphdef.cell().X)
	bends, crossings, overlaps := measure()
	for range 4 {
		improved := false
		for _, node := range graph.Nodes {
			if node.Virtual || fork(node) {
				continue
			}
			// in line with a node it has an edge to, or a cell or two over,
			// where ends that take slots along the node line up
			wants = wants[:0]
			for _, others := range [2][]*hier.Node{node.In, node.Out} {
				for _, other := range others {
					if !straight(node, other) {
						wants = append(wants, other.Center.X+other.Anchor-node.Anchor)
					}
				}
			}
			for _, k := range []float32{1, -1, 2, -2} {
				wants = append(wants, node.Center.X+k*cell)
			}
			for _, want := range wants {
				if math.Abs(float64(want-node.Center.X)) < 0.5 {
					continue
				}
				before := at()
				kept = kept[:0]
				for _, n := range graph.Nodes {
					if !n.Virtual && balanced(n) {
						kept = append(kept, n)
					}
				}
				move(node, want)
				if width() > wide+0.5 || slices.ContainsFunc(kept, func(n *hier.Node) bool { return !balanced(n) }) ||
					slices.ContainsFunc(graph.Nodes, func(n *hier.Node) bool { return fork(n) && n.Center.X != before[n.ID] }) {
					for i, x := range before {
						graph.Nodes[i].Center.X = x
					}
					continue
				}
				route()
				if b, c, o := measure(); b < bends && c <= crossings && o <= overlaps {
					bends, crossings, overlaps, improved = b, c, o, true
					break
				}
				for i, x := range before {
					graph.Nodes[i].Center.X = x
				}
			}
		}
		if !improved {
			break
		}
	}
}
