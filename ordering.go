package glay

func OrderRanks(graph *Graph) {
	OrderRanksDepthFirst(graph)
	for i := 0; i < 24; i++ {
		OrderRanksMedianImprove(graph)
	}
}

func OrderRanksDepthFirst(graph *Graph) {
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

func OrderRanksMedianImprove(graph *Graph) {

}
