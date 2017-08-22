package layout

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

	seen := make([]bool, len(graph.Nodes))
	ranking := make([]NodeIDs, len(graph.ByRank))

	var recurse func(id NodeID)
	recurse = func(id NodeID) {
		src := graph.Nodes[id]
		seen[id] = true
		ranking[src.Rank].Add(id)
		for _, did := range src.Out {
			if !seen[did] {
				recurse(did)
			}
		}
	}

	first := NodeIDs{}
	for _, node := range graph.Nodes {
		if len(node.In) == 0 {
			first.Add(node.ID)
		}
	}
	sortNodesByOutdegree(graph, first)

	for _, id := range first {
		recurse(id)
	}

	graph.ByRank = ranking
}

func OrderRanks_Improve_WeightedMedian(graph *Graph, down bool) {
	OrderRanks_Improve_WeightedMedian_AssignCoef(graph, down)
	for _, nodes := range graph.ByRank {
		sort.Slice(nodes, func(i, k int) bool {
			aid, bid := nodes[i], nodes[k]
			a, b := graph.Nodes[aid], graph.Nodes[bid]

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
	gridx := func(id NodeID) float32 {
		return graph.Nodes[id].GridX
	}

	for _, nodes := range graph.ByRank {
		for gridx, nid := range nodes {
			graph.Nodes[nid].GridX = float32(gridx)
		}
	}

	for _, n := range graph.Nodes {
		var adj NodeIDs
		if down {
			adj = n.Out
		} else {
			adj = n.In
		}

		if len(adj) == 0 {
			n.Coef = -1
		} else if len(adj)&1 == 1 {
			n.Coef = gridx(adj[len(adj)>>1])
		} else if len(adj) == 2 {
			n.Coef = (gridx(adj[0]) + gridx(adj[1])) / 2.0
		} else {
			leftx := gridx(adj[len(adj)>>1-1])
			rightx := gridx(adj[len(adj)>>1])

			left := leftx - gridx(adj[0])
			right := gridx(adj[len(adj)-1]) - rightx
			n.Coef = (leftx*right + rightx*left) / (left + right)
		}
	}
}

func OrderRanks_Improve_Transpose(graph *Graph) (swaps int) {
	for {
		improved := false

		for _, nodes := range graph.ByRank[1:] {
			for i := range nodes[:len(nodes)-1] {
				v := nodes[i]
				w := nodes[i+1]
				if graph.CrossingsUp(v, w) > graph.CrossingsUp(w, v) {
					nodes[i], nodes[i+1] = nodes[i+1], nodes[i]
					swaps++
				}
			}
		}

		if !improved {
			return
		}
	}
}
