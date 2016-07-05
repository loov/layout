package layout

// CreateVirtualVertices creates nodes for edges spanning multiple ranks
//
//     Rank  input    output
//      0      A        A
//            /|       / \
//      1    B |  =>  B   V
//            \|       \ /
//      2      C        C
func CreateVirtualVertices(graph *Graph) {
	if len(graph.ByRank) == 0 {
		return
	}

	for _, src := range graph.Nodes {
		for _, did := range src.Out {
			dst := graph.Nodes[did]
			if dst.Rank-src.Rank <= 1 {
				continue
			}

			src.Out.Remove(dst.ID)
			dst.In.Remove(src.ID)

			for rank := dst.Rank - 1; rank > src.Rank; rank-- {
				_, node := graph.Node()
				node.Rank = rank
				node.Virtual = true
				graph.ByRank[node.Rank].Add(node.ID)
				graph.Edge(node.ID, dst.ID)
				dst = node
			}

			src.Out.Add(dst.ID)
			dst.In.Add(src.ID)
		}
	}
}
