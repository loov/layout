package hier

import (
	"cmp"
	"slices"
)

// DefaultOrderIterations is the number of ordering sweeps OrderRanks runs
const DefaultOrderIterations = 24

// OrderRanks tries to minimize crossing edges
func OrderRanks(graph *Graph) { OrderRanksN(graph, DefaultOrderIterations) }

// OrderRanksN tries to minimize crossing edges with the given number of
// median sweeps; more sweeps can find better orders on large graphs.
func OrderRanksN(graph *Graph, iterations int) {
	// like Graphviz, sweep from more than one starting order: a sweep
	// moves nodes one at a time, and can't move a whole column of them
	// out of the way of the edges crossing it
	starts := []func(){
		func() { OrderRanksDepthFirst(graph) },
		func() { orderRanksBreadthFirst(graph, true) },
		func() { orderRanksBreadthFirst(graph, false) },
	}
	var best []Nodes
	var bestCrossings, bestLength float32
	improves := func(first bool) bool {
		crossings, length := graph.TotalCrossings(), graph.TotalEdgeLength()
		if first || crossings < bestCrossings || (crossings == bestCrossings && length < bestLength) {
			best, bestCrossings, bestLength = saveOrder(graph), crossings, length
			return true
		}
		return false
	}
	for k, start := range starts {
		start()
		orderFlatEdges(graph)
		orderClusters(graph)
		improves(k == 0)

		stale := 0
		for i := 0; i < iterations && stale < 4; i++ {
			OrderRanksByMedian(graph, i%2 == 0)
			OrderRanksSift(graph)
			OrderRanksTranspose(graph)
			orderFlatEdges(graph)
			orderClusters(graph)

			if improves(false) {
				stale = 0
			} else {
				stale++
			}
		}
	}
	// the best order can be one that no transpose has seen, such as a
	// starting one, when every sweep from it ends up worse
	graph.ByRank = best
	best = saveOrder(graph)
	OrderRanksTranspose(graph)
	orderFlatEdges(graph)
	orderClusters(graph)
	if graph.TotalCrossings() >= bestCrossings {
		graph.ByRank = best
	}
	OrderRanksChains(graph)
	orderTies(graph)
	graph.assignPos()
}

// orderTies puts neighbors in a rank that are as good either way, in
// crossings and edge length, in input order, which is the order of their
// ids; it keeps clusters together and flat edges left to right
func orderTies(graph *Graph) {
	neighbors := flatNeighbors(graph)
	flat := map[[2]ID]bool{}
	for _, edge := range graph.Flat {
		flat[[2]ID{edge[0].ID, edge[1].ID}] = true
		flat[[2]ID{edge[1].ID, edge[0].ID}] = true
	}
	graph.assignPos()
	// every swap puts a pair in order, so this ends
	for changed := true; changed; {
		changed = false
		for _, layer := range graph.ByRank {
			for i := 0; i+1 < len(layer); i++ {
				left, right := layer[i], layer[i+1]
				if right.ID > left.ID || left.Cluster != right.Cluster ||
					left.BorderLeft || left.BorderRight || right.BorderLeft || right.BorderRight ||
					flat[[2]ID{left.ID, right.ID}] {
					continue
				}
				if before, after := graph.crossingsBothWays(left, right); before != after {
					continue
				}
				if graph.edgeLength(left, i)+graph.edgeLength(right, i+1) != graph.edgeLength(left, i+1)+graph.edgeLength(right, i) {
					continue
				}
				if flatShorter(neighbors, left, right) != 0 {
					continue
				}
				layer[i], layer[i+1] = right, left
				left.Pos, right.Pos = i+1, i
				changed = true
			}
		}
	}
}

// saveOrder returns a copy of the current rank ordering
func saveOrder(graph *Graph) []Nodes {
	saved := make([]Nodes, len(graph.ByRank))
	for i, layer := range graph.ByRank {
		saved[i] = slices.Clone(layer)
	}
	return saved
}

// OrderRanksDepthFirst reorders based on depth first traverse
func OrderRanksDepthFirst(graph *Graph) {
	if len(graph.ByRank) == 0 {
		return
	}

	seen := NewNodeSet(graph.NodeCount())
	ranking := make([]Nodes, len(graph.ByRank))

	var process func(node *Node)
	process = func(src *Node) {
		if !seen.Include(src) {
			return
		}

		ranking[src.Rank].Append(src)
		for _, dst := range src.Out {
			process(dst)
		}
	}

	roots := graph.Roots()
	roots.SortDescending()

	for _, id := range roots {
		process(id)
	}

	graph.ByRank = ranking
}

// OrderRanksByMedian sweeps layers, sorting each by the median position of
// its neighbors in the previously processed layer. down sweeps top-to-bottom
// using incoming edges, otherwise bottom-to-top using outgoing edges.
func OrderRanksByMedian(graph *Graph, down bool) {
	assignGridX(graph)

	order := make([]int, len(graph.ByRank))
	for i := range order {
		order[i] = i
	}
	if !down {
		slices.Reverse(order)
	}

	for _, r := range order {
		layer := graph.ByRank[r]
		for _, node := range layer {
			adj := node.In
			if !down {
				adj = node.Out
			}
			node.Coef = medianGridX(graph.portGridX(node, adj, down), node.GridX)
		}
		clusterCoef(layer)
		slices.SortStableFunc(layer, func(a, b *Node) int {
			if a.Coef != b.Coef {
				return cmp.Compare(a.Coef, b.Coef)
			}
			return cmp.Compare(borderSide(a), borderSide(b))
		})
		for i, node := range layer {
			node.GridX = float32(i)
		}
	}
}

// assignGridX records each node's index within its rank as GridX
func assignGridX(graph *Graph) {
	for _, nodes := range graph.ByRank {
		for i, node := range nodes {
			node.GridX = float32(i)
		}
	}
}

// portGridX returns where the edges of node to the nodes adj, before it
// when down is set and after it otherwise, end on them: their GridX, off
// by where on the node the edge ends, see Graph.Ports
func (graph *Graph) portGridX(node *Node, adj Nodes, down bool) []float32 {
	xs := make([]float32, len(adj))
	for i, n := range adj {
		xs[i] = n.GridX
		if down {
			xs[i] += graph.Ports[[2]ID{n.ID, node.ID}][0]
		} else {
			xs[i] += graph.Ports[[2]ID{node.ID, n.ID}][1]
		}
	}
	return xs
}

// medianGridX returns the weighted median of the positions xs, or
// fallback when there are none.
func medianGridX(xs []float32, fallback float32) float32 {
	if len(xs) == 0 {
		return fallback
	}
	slices.Sort(xs)

	m := len(xs) / 2
	switch {
	case len(xs)&1 == 1:
		return xs[m]
	case len(xs) == 2:
		return (xs[0] + xs[1]) / 2
	default:
		left := xs[m-1] - xs[0]
		right := xs[len(xs)-1] - xs[m]
		if left+right == 0 {
			return (xs[m-1] + xs[m]) / 2
		}
		return (xs[m-1]*right + xs[m]*left) / (left + right)
	}
}

// crossingsByPos counts unweighted crossings of u left of v and v left of u
// from the cached neighbor position arrays
func crossingsByPos(u, v *Node) (uv, vu float32) {
	var a, b int32
	for _, w := range u.inPos {
		for _, z := range v.inPos {
			if z < w {
				a++
			} else if z > w {
				b++
			}
		}
	}
	for _, w := range u.outPos {
		for _, z := range v.outPos {
			if z < w {
				a++
			} else if z > w {
				b++
			}
		}
	}
	return float32(a), float32(b)
}

// flatNeighbors returns the other ends of the flat edges of every node.
func flatNeighbors(graph *Graph) map[*Node]Nodes {
	flat := map[*Node]Nodes{}
	for _, edge := range graph.Flat {
		flat[edge[0]] = append(flat[edge[0]], edge[1])
		flat[edge[1]] = append(flat[edge[1]], edge[0])
	}
	return flat
}

// flatShorter returns how many fewer nodes the flat edges of neighbors left
// and right pass over when they swap places; a flat edge costs a crossing
// for every node it passes over, see TotalCrossings.
func flatShorter(flat map[*Node]Nodes, left, right *Node) (shorter float32) {
	for _, x := range flat[left] {
		if x.Pos > right.Pos {
			shorter++
		} else if x.Pos < left.Pos {
			shorter--
		}
	}
	for _, x := range flat[right] {
		if x.Pos < left.Pos {
			shorter++
		} else if x.Pos > right.Pos {
			shorter--
		}
	}
	return shorter
}

// orderFlatEdges ensures the source of every flat edge is left of its target
// by sorting the ranks with misordered flat edges topologically: the
// leftmost node whose flat sources are all placed goes next, which leaves
// the other nodes in their order.
func orderFlatEdges(graph *Graph) {
	graph.assignPos()
	preds := map[*Node]int{}
	succs := map[*Node]Nodes{}
	misordered := map[int]bool{}
	for _, edge := range graph.Flat {
		src, dst := edge[0], edge[1]
		if src.Pos > dst.Pos {
			misordered[src.Rank] = true
		}
		preds[dst]++
		succs[src] = append(succs[src], dst)
	}
	for rank := range misordered {
		layer := graph.ByRank[rank]
		placed := make([]bool, len(layer))
		sorted := make(Nodes, 0, len(layer))
		// quadratic in the rank size; a ready queue by position if wide
		// ranks with flat edges get slow
		for len(sorted) < len(layer) {
			// on a cycle of flat edges no node is ready; take the leftmost
			pick, fallback := -1, -1
			for i, node := range layer {
				if placed[i] {
					continue
				}
				if fallback < 0 {
					fallback = i
				}
				if preds[node] == 0 {
					pick = i
					break
				}
			}
			if pick < 0 {
				pick = fallback
			}
			placed[pick] = true
			sorted = append(sorted, layer[pick])
			for _, dst := range succs[layer[pick]] {
				preds[dst]--
			}
		}
		copy(layer, sorted)
		layer.assignPos()
	}
}

// OrderRanksTranspose swaps adjacent nodes while it reduces crossings, or
// shortens edges without adding crossings.
func OrderRanksTranspose(graph *Graph) (swaps int) {
	graph.assignPos()
	// the cached positions leave out weights and ports
	weighted := len(graph.weights) > 0 || len(graph.Ports) > 0
	flat := flatNeighbors(graph)
	// a layer only needs another look when it or a neighbor changed
	dirty := make([]bool, len(graph.ByRank))
	for i := range dirty {
		dirty[i] = true
	}
	for range 20 {
		improved := false
		changed := make([]bool, len(graph.ByRank))
		for r, nodes := range graph.ByRank {
			if !dirty[r] {
				continue
			}
			// neighbor positions don't change while this layer is processed;
			// cache them as flat arrays so the pair loop below streams
			if !weighted {
				for _, node := range nodes {
					node.inPos = node.inPos[:0]
					for _, src := range node.In {
						node.inPos = append(node.inPos, int32(src.Pos))
					}
					node.outPos = node.outPos[:0]
					for _, dst := range node.Out {
						node.outPos = append(node.outPos, int32(dst.Pos))
					}
				}
			}
			for i := 0; i+1 < len(nodes); i++ {
				left, right := nodes[i], nodes[i+1]
				var before, after float32
				if weighted {
					before, after = graph.crossingsBothWays(left, right)
				} else {
					before, after = crossingsByPos(left, right)
				}
				after -= flatShorter(flat, left, right)
				if before == after {
					before = graph.edgeLength(left, i) + graph.edgeLength(right, i+1)
					after = graph.edgeLength(left, i+1) + graph.edgeLength(right, i)
				}
				if before > after {
					nodes[i], nodes[i+1] = right, left
					left.Pos, right.Pos = i+1, i
					swaps++
					improved = true
					changed[r] = true
				}
			}
		}
		for r := range dirty {
			dirty[r] = changed[r] || (r > 0 && changed[r-1]) || (r+1 < len(changed) && changed[r+1])
		}
		if !improved {
			return swaps
		}
	}
	return swaps
}

// orderRanksBreadthFirst orders the ranks by a breadth first walk in input
// order, from the sources when down, otherwise from the sinks.
func orderRanksBreadthFirst(graph *Graph, down bool) {
	seen := NewNodeSet(graph.NodeCount())
	ranking := make([]Nodes, len(graph.ByRank))
	for _, root := range graph.Nodes {
		if (down && root.InDegree() > 0) || (!down && root.OutDegree() > 0) || !seen.Include(root) {
			continue
		}
		queue := Nodes{root}
		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			ranking[node.Rank].Append(node)
			next := node.Out
			if !down {
				next = node.In
			}
			for _, dst := range next {
				if seen.Include(dst) {
					queue = append(queue, dst)
				}
			}
		}
	}
	graph.ByRank = ranking
}
