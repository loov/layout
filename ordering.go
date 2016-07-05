package layout

func OrderRanks(graph *Graph) {
	OrderRanks_Initial_DepthFirst(graph)
	for i := 0; i < 100; i++ {
		OrderRanks_Improve_Median(graph)
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

	graph.Sort(first, func(a, b *Node) bool { return len(a.Out) > len(b.Out) })
	for _, id := range first {
		recurse(id)
	}

	graph.ByRank = ranking
}

func OrderRanks_Improve_Median(graph *Graph) {

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
