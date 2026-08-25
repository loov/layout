package hier

import (
	"slices"
	"sort"
)

// DefaultOrderRanks does recommended rank ordering
func DefaultOrderRanks(graph *Graph) *Graph {
	OrderRanks(graph)
	return graph
}

// OrderRanks tries to minimize crossing edges
func OrderRanks(graph *Graph) {
	OrderRanksDepthFirst(graph)

	best := saveOrder(graph)
	bestCrossings, bestLength := graph.TotalCrossings(), graph.TotalEdgeLength()
	for i := range 24 {
		OrderRanksByMedian(graph, i%2 == 0)
		OrderRanksTranspose(graph)

		crossings, length := graph.TotalCrossings(), graph.TotalEdgeLength()
		if crossings < bestCrossings || (crossings == bestCrossings && length < bestLength) {
			best, bestCrossings, bestLength = saveOrder(graph), crossings, length
		}
	}
	graph.ByRank = best
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
		sort.SliceStable(layer, func(i, k int) bool {
			return layer[i].Coef < layer[k].Coef
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

// OrderRanksTranspose swaps adjacent nodes while it reduces crossings, or
// shortens edges without adding crossings.
func OrderRanksTranspose(graph *Graph) (swaps int) {
	graph.assignPos()
	for range 20 {
		improved := false
		for _, nodes := range graph.ByRank {
			for i := 0; i+1 < len(nodes); i++ {
				left, right := nodes[i], nodes[i+1]
				before, after := graph.Crossings(left, right), graph.Crossings(right, left)
				if before == after {
					before = graph.edgeLength(left, i) + graph.edgeLength(right, i+1)
					after = graph.edgeLength(left, i+1) + graph.edgeLength(right, i)
				}
				if before > after {
					nodes[i], nodes[i+1] = right, left
					nodes.assignPos()
					swaps++
					improved = true
				}
			}
		}
		if !improved {
			return swaps
		}
	}
	return swaps
}
