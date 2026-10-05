package layout

import (
	"math"
	"slices"

	"github.com/loov/layout/internal/hier"
)

// straightenNodes moves a node along its rank in line with a node it has
// an edge to, and pushes the ones beside it as far as they have to go,
// where the edges that finish draws then bend less and cross no more;
// positioning can't tell which edges end up straight
func straightenNodes(graph *hier.Graph, graphdef *lgraph, finish func()) {
	// merged edges share a run, see Graph.MergeEdges
	merged := func(a, b *ledge) bool {
		ma, mb := graphdef.merged[a], graphdef.merged[b]
		return ma[0] != 0 && ma[0] == mb[0] || ma[1] != 0 && ma[1] == mb[1]
	}
	measure := func() (bends, crossings, overlaps int) {
		type segment struct {
			a, b Vector
			edge *ledge
		}
		var horizontal, vertical []segment
		for _, edge := range graphdef.Edges {
			for i := 1; i < len(edge.Path); i++ {
				a, b := edge.Path[i-1], edge.Path[i]
				switch {
				case a.Y == b.Y && a.X != b.X:
					horizontal = append(horizontal, segment{a, b, edge})
				case a.X == b.X && a.Y != b.Y:
					vertical = append(vertical, segment{a, b, edge})
				}
				if i+1 < len(edge.Path) {
					c := edge.Path[i+1]
					if (a.X == b.X) != (b.X == c.X) {
						bends++
					}
				}
			}
		}
		for _, h := range horizontal {
			for _, v := range vertical {
				if h.edge != v.edge &&
					min(h.a.X, h.b.X) < v.a.X && v.a.X < max(h.a.X, h.b.X) &&
					min(v.a.Y, v.b.Y) < h.a.Y && h.a.Y < max(v.a.Y, v.b.Y) {
					crossings++
				}
			}
		}
		// runs of different edges along each other
		for _, runs := range [][]segment{horizontal, vertical} {
			for i, r := range runs {
				for _, q := range runs[i+1:] {
					if r.edge == q.edge || merged(r.edge, q.edge) {
						continue
					}
					same := r.a.Y == q.a.Y && min(max(r.a.X, r.b.X), max(q.a.X, q.b.X)) > max(min(r.a.X, r.b.X), min(q.a.X, q.b.X))
					if r.a.X == r.b.X {
						same = r.a.X == q.a.X && min(max(r.a.Y, r.b.Y), max(q.a.Y, q.b.Y)) > max(min(r.a.Y, r.b.Y), min(q.a.Y, q.b.Y))
					}
					if same {
						overlaps++
					}
				}
			}
		}
		// runs along the frame of a cluster, within half a cell of it, which
		// text draws on it
		near := Vector{X: graphdef.cellWidth() / 2, Y: graphdef.LineHeight / 2}
		if sideways(graphdef.RankDir) {
			near.X, near.Y = near.Y, near.X
		}
		for _, cluster := range graphdef.Clusters {
			tl, br := cluster.TopLeft, cluster.BottomRight
			for _, h := range horizontal {
				lo, hi := min(h.a.X, h.b.X), max(h.a.X, h.b.X)
				if (absLength(h.a.Y-tl.Y) <= near.Y || absLength(h.a.Y-br.Y) <= near.Y) && min(hi, br.X)-max(lo, tl.X) > near.X {
					overlaps++
				}
			}
			for _, v := range vertical {
				lo, hi := min(v.a.Y, v.b.Y), max(v.a.Y, v.b.Y)
				if (absLength(v.a.X-tl.X) <= near.X || absLength(v.a.X-br.X) <= near.X) && min(hi, br.Y)-max(lo, tl.Y) > near.Y {
					overlaps++
				}
			}
		}
		return bends, crossings, overlaps
	}
	fan := func(node *hier.Node) bool { return !node.Virtual && len(node.In) == 1 && len(node.In[0].Out) > 1 }
	gap := func(a, b *hier.Node) float32 {
		gap := a.Radius.X + b.Radius.X
		if fan(a) && fan(b) && a.In[0] != b.In[0] {
			gap += graph.FamilyGap
		}
		return gap
	}
	at := func() []float32 {
		xs := make([]float32, len(graph.Nodes))
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
	label := func(node *hier.Node) bool { return node.Virtual && node.Anchor != 0 }
	neighbors := func(node *hier.Node, before bool) []*hier.Node {
		var out []*hier.Node
		next := node.Out
		if before {
			next = node.In
		}
		for _, n := range next {
			for n.Virtual && !label(n) && !real[n.Rank] {
				ends := n.Out
				if before {
					ends = n.In
				}
				if len(ends) != 1 {
					break
				}
				n = ends[0]
			}
			if !n.Virtual || label(n) && !graphdef.MergeEdges {
				out = append(out, n)
			}
		}
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
	cell := float32(graphdef.cellWidth())
	if sideways(graphdef.RankDir) {
		cell = float32(graphdef.LineHeight)
	}
	bends, crossings, overlaps := measure()
	for range 4 {
		improved := false
		for _, node := range graph.Nodes {
			if node.Virtual || fork(node) {
				continue
			}
			// in line with a node it has an edge to, or a cell or two over,
			// where ends that take slots along the node line up
			var wants []float32
			for _, other := range slices.Concat(node.In, node.Out) {
				if !straight(node, other) {
					wants = append(wants, other.Center.X+other.Anchor-node.Anchor)
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
				var kept []*hier.Node
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
				finish()
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
	finish()
}
