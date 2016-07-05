package layout

// FrontloadRanks assigns node.Rank := max(node.In[i].Rank) + 1
func FrontloadRanks(graph *Graph) {
	roots := NodeIDs{}
	incount := make([]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		incount[node.ID] = len(node.In)
		if len(node.In) == 0 {
			roots.Add(node.ID)
		}
	}

	rank := 0
	graph.ByRank = nil
	for len(roots) > 0 {
		graph.ByRank = append(graph.ByRank, roots)
		next := NodeIDs{}
		for _, sid := range roots {
			src := graph.Nodes[sid]
			src.Rank = rank
			for _, did := range src.Out {
				incount[did]--
				if incount[did] == 0 {
					next.Add(did)
				}
			}
		}
		roots = next
		rank++
	}
}

// BackloadRanks assigns node.Rank := min(node.Out[i].Rank) - 1
func BackloadRanks(graph *Graph) {
	roots := NodeIDs{}
	outcount := make([]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		outcount[node.ID] = len(node.Out)
		if len(node.Out) == 0 {
			roots.Add(node.ID)
		}
	}

	rank := 0
	graph.ByRank = nil
	for len(roots) > 0 {
		graph.ByRank = append(graph.ByRank, roots)
		next := NodeIDs{}
		for _, did := range roots {
			dst := graph.Nodes[did]
			dst.Rank = rank
			for _, sid := range dst.In {
				outcount[sid]--
				if outcount[sid] == 0 {
					next.Add(sid)
				}
			}
		}
		roots = next
		rank++
	}

	for i := range graph.ByRank[:len(graph.ByRank)/2] {
		k := len(graph.ByRank) - i - 1
		graph.ByRank[i], graph.ByRank[k] = graph.ByRank[k], graph.ByRank[i]
	}

	for rank, nodes := range graph.ByRank {
		for _, id := range nodes {
			graph.Nodes[id].Rank = rank
		}
	}
}
