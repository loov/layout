package hier

import (
	"sort"
)

func OrderRanks(graph *Graph) {
	OrderRanks_Initial_DepthFirst(graph)
	for i := 0; i < 100; i++ {
		OrderRanks_Improve_WeightedMedian(graph, i%2 == 0)
		if OrderRanks_Improve_Transpose(graph) == 0 {
			break
		}
	}
}

func OrderRanks_Initial_DepthFirst(graph *Graph) {
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

func OrderRanks_Improve_WeightedMedian(graph *Graph, down bool) {
	OrderRanks_Improve_WeightedMedian_AssignCoef(graph, down)

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

func OrderRanks_Improve_WeightedMedian_AssignCoef(graph *Graph, down bool) {
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

func OrderRanks_Improve_Transpose(graph *Graph) (swaps int) {
	for {
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
}
