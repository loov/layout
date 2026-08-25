package hier

import (
	"cmp"
	"slices"
)

// DefaultOrderRanks does recommended rank ordering
func DefaultOrderRanks(graph *Graph) *Graph {
	OrderRanks(graph)
	return graph
}

// DefaultOrderIterations is the number of ordering sweeps OrderRanks runs
const DefaultOrderIterations = 24

// OrderRanks tries to minimize crossing edges
func OrderRanks(graph *Graph) { OrderRanksN(graph, DefaultOrderIterations) }

// OrderRanksN tries to minimize crossing edges with the given number of
// median sweeps; more sweeps can find better orders on large graphs.
func OrderRanksN(graph *Graph, iterations int) {
	OrderRanksDepthFirst(graph)
	orderClusters(graph)

	best := saveOrder(graph)
	bestCrossings, bestLength := graph.TotalCrossings(), graph.TotalEdgeLength()
	stale := 0
	for i := 0; i < iterations && stale < 4; i++ {
		OrderRanksByMedian(graph, i%2 == 0)
		OrderRanksTranspose(graph)
		orderFlatEdges(graph)
		orderClusters(graph)

		crossings, length := graph.TotalCrossings(), graph.TotalEdgeLength()
		if crossings < bestCrossings || (crossings == bestCrossings && length < bestLength) {
			best, bestCrossings, bestLength = saveOrder(graph), crossings, length
			stale = 0
		} else {
			stale++
		}
	}
	graph.ByRank = best
	graph.assignPos()
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
			node.Coef = medianGridX(adj, node.GridX)
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

// medianGridX returns the weighted median of adj positions, or fallback when
// there are no neighbors.
func medianGridX(adj Nodes, fallback float32) float32 {
	if len(adj) == 0 {
		return fallback
	}
	xs := make([]float32, len(adj))
	for i, n := range adj {
		xs[i] = n.GridX
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

// orderFlatEdges ensures the source of every flat edge is left of its target
// by moving the target right after the source.
func orderFlatEdges(graph *Graph) {
	graph.assignPos()
	for _, edge := range graph.Flat {
		src, dst := edge[0], edge[1]
		if src.Pos < dst.Pos {
			continue
		}
		layer := graph.ByRank[src.Rank]
		layer.moveNode(dst.Pos, src.Pos)
		layer.assignPos()
	}
}

// moveNode moves the node at index from to index to, shifting the others
func (nodes Nodes) moveNode(from, to int) {
	node := nodes[from]
	if from < to {
		copy(nodes[from:to], nodes[from+1:to+1])
	} else {
		copy(nodes[to+1:from+1], nodes[to:from])
	}
	nodes[to] = node
}

// OrderRanksTranspose swaps adjacent nodes while it reduces crossings, or
// shortens edges without adding crossings.
func OrderRanksTranspose(graph *Graph) (swaps int) {
	graph.assignPos()
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
			for i := 0; i+1 < len(nodes); i++ {
				left, right := nodes[i], nodes[i+1]
				before, after := graph.crossingsBothWays(left, right)
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
