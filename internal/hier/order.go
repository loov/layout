package hier

import (
	"sort"
)

// DefaultOrderRanks does recommended rank ordering
func DefaultOrderRanks(graph *Graph) *Graph {
	OrderRanks(graph)
	return graph
}

// OrderRanks tries to minimize crossign edges
func OrderRanks(graph *Graph) {
	OrderRanksDepthFirst(graph)
	for i := 0; i < 100; i++ {
		OrderRanksByCoef(graph, i%2 == 0)
		if OrderRanksTranspose(graph) == 0 {
			break
		}
	}
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

// OrderRanksByCoef reorders based on target grid and coef
func OrderRanksByCoef(graph *Graph, down bool) {
	OrderRanksAssignMetrics(graph, down)

	for _, nodes := range graph.ByRank {
		sort.Slice(nodes, func(i, k int) bool {
			a, b := nodes[i], nodes[k]

			if a.Coef == -1.0 {
				if b.Coef == -1.0 {
					return a.GridX < b.GridX
				} else {
					return a.GridX < b.Coef
				}
			} else {
				if b.Coef == -1.0 {
					return a.Coef < b.GridX
				} else {
					return a.Coef < b.Coef
				}
			}
		})
	}
}

// OrderRanksAssignMetrics recalculates metrics for ordering
func OrderRanksAssignMetrics(graph *Graph, down bool) {
	for _, nodes := range graph.ByRank {
		for i, node := range nodes {
			node.GridX = float32(i)
		}
	}

	for _, node := range graph.Nodes {
		var adj Nodes
		if down {
			adj = node.Out
		} else {
			adj = node.In
		}

		if len(adj) == 0 {
			node.Coef = -1
		} else if len(adj)&1 == 1 {
			node.Coef = adj[len(adj)>>1].GridX
		} else if len(adj) == 2 {
			node.Coef = (adj[0].GridX + adj[1].GridX) / 2.0
		} else {
			leftx := adj[len(adj)>>1-1].GridX
			rightx := adj[len(adj)>>1].GridX

			left := leftx - adj[0].GridX
			right := adj[len(adj)-1].GridX - rightx
			node.Coef = (leftx*right + rightx*left) / (left + right)
		}
	}
}

// OrderRanksTranspose swaps nodes which are side by side and will use less crossings
func OrderRanksTranspose(graph *Graph) (swaps int) {
	for limit := 0; limit < 20; limit++ {
		improved := false

		for _, nodes := range graph.ByRank[1:] {
			left := nodes[0]
			for i, right := range nodes[1:] {
				if graph.CrossingsUp(left, right) > graph.CrossingsUp(right, left) {
					nodes[i], nodes[i+1] = right, left
					right, left = left, right
					swaps++
				}
				left = right
			}
		}

		if !improved {
			return
		}
	}

	return 0
}
