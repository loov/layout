package hier

func (graph *Graph) CrossingsUp(u, v *Node) int {
	if u.Rank == 0 {
		return 0
	}

	count := 0
	prev := graph.ByRank[u.Rank-1]
	for _, w := range u.In {
		for _, z := range v.In {
			if prev.IndexOf(z) < prev.IndexOf(w) {
				count++
			}
		}
	}
	return count
}

func (graph *Graph) CrossingsDown(u, v *Node) int {
	if u.Rank == len(graph.ByRank)-1 {
		return 0
	}

	count := 0
	next := graph.ByRank[u.Rank+1]
	for _, w := range u.In {
		for _, z := range v.In {
			if next.IndexOf(z) < next.IndexOf(w) {
				count++
			}
		}
	}
	return count
}

func (graph *Graph) Crossings(u, v *Node) int {
	return graph.CrossingsDown(u, v) + graph.CrossingsUp(u, v)
}
