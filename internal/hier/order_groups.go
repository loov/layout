package hier

import "slices"

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

	lo := max(g.changed[0][0][0].Rank-1, 0)
	hi := min(g.changed[len(g.changed)-1][0][0].Rank+1, len(graph.ByRank)-1)
	set(0)
	before := graph.rangeCrossings(lo, hi)
	set(1)
	if graph.rangeCrossings(lo, hi) < before {
		return true
	}
	set(0)
	return false
}
