package hier

import "slices"

// OrderRanksBlocks moves columns of nodes as a whole. Sift and transpose
// move one node at a time, and a node in a column, such as one of a fan
// into a hub, costs crossings with the rest of its column wherever it
// moves alone. A column is a block of nodes, each aligned with the median
// of the nodes above it as in Brandes–Köpf; every block in turn goes to
// the places in its ranks, found by dynamic programming over the ranks,
// that cross least with everything else where it is, and stays there if
// that crosses less in total.
func OrderRanksBlocks(graph *Graph) (moves int) {
	if len(graph.Clusters) > 0 {
		// a block would have to stay inside its clusters
		return 0
	}
	graph.assignPos()
	flat := flatNeighbors(graph)
	mover := &blockMover{graph: graph}
	for _, block := range alignBlocks(graph, flat) {
		if mover.move(block) {
			moves++
		}
	}
	return moves
}

// alignBlocks aligns every node with a median of the nodes above it, left
// to right, without alignments crossing each other, and returns the
// blocks of more than one node, top to bottom
func alignBlocks(graph *Graph, flat map[*Node]Nodes) []Nodes {
	free := func(n *Node) bool { return !n.IsBorder() && len(flat[n]) == 0 }
	below := make([]*Node, len(graph.Nodes))
	above := make([]bool, len(graph.Nodes))
	var ins Nodes
	for _, layer := range graph.ByRank {
		last := -1
		for _, v := range layer {
			if !free(v) || len(v.In) == 0 {
				continue
			}
			ins = append(ins[:0], v.In...)
			slices.SortFunc(ins, func(a, b *Node) int { return a.Pos - b.Pos })
			lo, hi := medians(len(ins))
			for _, m := range [2]int{lo, hi} {
				u := ins[m]
				if below[u.ID] == nil && u.Pos > last && free(u) && u.Rank+1 == v.Rank {
					below[u.ID], above[v.ID] = v, true
					last = u.Pos
					break
				}
			}
		}
	}

	var blocks []Nodes
	for _, layer := range graph.ByRank {
		for _, v := range layer {
			if above[v.ID] || below[v.ID] == nil {
				continue
			}
			block := Nodes{v}
			for n := below[v.ID]; n != nil; n = below[n.ID] {
				block.Append(n)
			}
			blocks = append(blocks, block)
		}
	}

	return blocks
}

// blockMover moves blocks, see OrderRanksBlocks, reusing its buffers.
// Slot p of a node b of a block is the place after the first p nodes of
// its rank but b.
type blockMover struct {
	graph *Graph
	block Nodes

	unary  [][]float32 // the crossings of block[i] at every slot, see slotCosts
	cands  [][]int     // the slots block[i] can gain at
	best   [][]float32 // the least cost of block[:i+1] with block[i] at cands[i][k]
	from   [][]int     // the candidate of block[i-1] that best[i][k] takes
	slot   []int
	before []Nodes // the ranks before the move

	left, right []float32
	edges       []blockEdge
	starts      []int
	column      []float32
}

// blockEdge is an edge between the ranks of the edge b->c of a block,
// with the slots of its ends, see blockEdges
type blockEdge struct {
	x, y   int
	weight float32 // times the weight of b->c
}

// resize returns buf with length n, reusing its room
func resize[T any](buf []T, n int) []T {
	return slices.Grow(buf[:0], n)[:n]
}

// move moves the nodes of block, one per rank from the top, to the places
// that cross least, and reports whether it moved them
func (m *blockMover) move(block Nodes) bool {
	graph := m.graph
	m.block = block
	n := len(block)
	m.unary, m.cands = resize(m.unary, n), resize(m.cands, n)
	m.best, m.from = resize(m.best, n), resize(m.from, n)

	// the crossings the block has where it is; wherever it goes, every
	// node of it has at least the least crossings of its own edges
	cost, least := float32(0), float32(0)
	for i, b := range block {
		m.unary[i] = m.slotCosts(b, m.unary[i])
		cost += m.unary[i][b.Pos]
		least += slices.Min(m.unary[i])
		if i > 0 {
			cost += graph.blockEdgeCrossings(block[i-1], b)
		}
	}

	if least >= cost {
		return false
	}

	// a node can only go where its own edges cross less than the block
	// gains in all, so only those slots count
	budget := cost - least
	for i, costs := range m.unary {
		low := slices.Min(costs)
		m.cands[i] = m.cands[i][:0]
		for p, c := range costs {
			if c-low < budget {
				m.cands[i] = append(m.cands[i], p)
			}
		}
	}

	m.best[0], m.from[0] = m.best[0][:0], m.from[0][:0]
	for _, p := range m.cands[0] {
		m.best[0] = append(m.best[0], m.unary[0][p])
		m.from[0] = append(m.from[0], -1)
	}
	for i := 1; i < n; i++ {
		m.blockEdges(block[i-1], block[i])
		m.best[i], m.from[i] = m.best[i][:0], m.from[i][:0]
		for _, q := range m.cands[i] {
			// the crossings of the edge into block[i] at q, for every slot
			// of block[i-1]
			pair := m.blockEdgeCosts(q)
			c, back := m.best[i-1][0]+pair[m.cands[i-1][0]], 0
			for k, p := range m.cands[i-1] {
				if d := m.best[i-1][k] + pair[p]; d < c {
					c, back = d, k
				}
			}
			m.best[i] = append(m.best[i], c+m.unary[i][q])
			m.from[i] = append(m.from[i], back)
		}
	}

	last := n - 1
	k := 0
	for j, c := range m.best[last] {
		if c < m.best[last][k] {
			k = j
		}
	}
	if m.best[last][k] >= cost {
		return false
	}
	m.slot = resize(m.slot, n)
	for i := last; i >= 0; i-- {
		m.slot[i] = m.cands[i][k]
		k = m.from[i][k]
	}

	lo, hi := max(block[0].Rank-1, 0), min(block[last].Rank+1, len(graph.ByRank)-1)
	before := graph.rangeCrossings(lo, hi)
	m.before = resize(m.before, n)
	for i, b := range block {
		layer := graph.ByRank[b.Rank]
		m.before[i] = append(m.before[i][:0], layer...)
		// slide the nodes between b and its slot over by one
		at, to := b.Pos, m.slot[i]
		if to < at {
			copy(layer[to+1:at+1], layer[to:at])
		} else {
			copy(layer[at:to], layer[at+1:to+1])
		}
		layer[to] = b
		layer.assignPos()
	}

	if graph.rangeCrossings(lo, hi) < before {
		return true
	}
	for i, b := range block {
		layer := graph.ByRank[b.Rank]
		copy(layer, m.before[i])
		layer.assignPos()
	}
	return false
}

// slotCosts returns in costs the crossings of the edges of b that don't
// lead to its block with the edges of the other nodes of its rank, for b
// at every slot among them
func (m *blockMover) slotCosts(b *Node, costs []float32) []float32 {
	graph, layer := m.graph, m.graph.ByRank[b.Rank]
	others := len(layer) - 1
	costs = resize(costs, others+1)
	m.left, m.right = resize(m.left, others), resize(m.right, others)
	clear(m.left)
	clear(m.right)
	for _, above := range [2]bool{true, false} {
		for _, w := range b.Adjacent(above) {
			if slices.Contains(m.block, w) {
				continue
			}
			for k := range others {
				u := layer[k]
				if k >= b.Pos {
					u = layer[k+1]
				}
				for _, z := range u.Adjacent(above) {
					// left[k]: crossings with u left of b; right[k]: with
					// b left of u
					weight := graph.edgeWeight(b, w) * graph.edgeWeight(u, z)
					if z.Pos > w.Pos {
						m.left[k] += weight
					} else if z.Pos < w.Pos {
						m.right[k] += weight
					}
				}
			}
		}
	}

	costs[0] = 0
	for k := range others {
		costs[0] += m.right[k]
	}
	for p := range others {
		costs[p+1] = costs[p] - m.right[p] + m.left[p]
	}
	return costs
}

// edgeWeight returns the weight of the edge between a and b, either way
func (graph *Graph) edgeWeight(a, b *Node) float32 {
	if a.Rank < b.Rank {
		return graph.Weight(a, b)
	}
	return graph.Weight(b, a)
}

// blockEdges collects the edges between the other nodes of the ranks of
// the edge b->c of a block, by the slots of their ends, in the order of
// their upper ends
func (m *blockMover) blockEdges(b, c *Node) {
	graph, upper := m.graph, m.graph.ByRank[b.Rank]
	m.edges = m.edges[:0]
	for x, u := range upper {
		if u == b {
			continue
		}
		if x > b.Pos {
			x--
		}
		for _, v := range u.Out {
			if v == c || v.Rank != c.Rank {
				continue
			}
			y := v.Pos
			if v.Pos > c.Pos {
				y--
			}
			m.edges = append(m.edges, blockEdge{x, y, graph.Weight(b, c) * graph.Weight(u, v)})
		}
	}

	m.column = resize(m.column, len(upper))
}

// blockEdgeCosts returns the crossings of the edge b->c with the edges of
// blockEdges, with c at slot q, for b at every slot
func (m *blockMover) blockEdgeCosts(q int) []float32 {
	costs := m.column
	clear(costs)

	// with b leftmost every edge is right of it at the top, and crosses
	// b->c when it ends left of c; past slot p, the edges from it are
	// left of b and cross b->c when they end right of c
	for _, e := range m.edges {
		if e.y < q {
			costs[0] += e.weight
			costs[e.x+1] -= e.weight
		} else {
			costs[e.x+1] += e.weight
		}
	}

	for p := 1; p < len(costs); p++ {
		costs[p] += costs[p-1]
	}
	return costs
}

// blockEdgeCrossings counts the crossings of the edge b->c with the edges
// between the other nodes of their ranks
func (graph *Graph) blockEdgeCrossings(b, c *Node) float32 {
	total := float32(0)
	for _, u := range graph.ByRank[b.Rank] {
		if u == b {
			continue
		}
		for _, v := range u.Out {
			if v != c && v.Rank == c.Rank && (u.Pos < b.Pos) != (v.Pos < c.Pos) {
				total += graph.Weight(b, c) * graph.Weight(u, v)
			}
		}
	}

	return total
}

// rangeCrossings counts the crossings between the ranks lo to hi, and the
// nodes the flat edges in them pass over
func (graph *Graph) rangeCrossings(lo, hi int) float32 {
	total := float32(0)
	for r := lo + 1; r <= hi; r++ {
		total += graph.bilayerCrossings(graph.ByRank[r-1], graph.ByRank[r])
	}
	for _, edge := range graph.Flat {
		if edge[0].Rank >= lo && edge[0].Rank <= hi {
			total += float32(max(edge[0].Pos, edge[1].Pos) - min(edge[0].Pos, edge[1].Pos) - 1)
		}
	}
	return total
}
