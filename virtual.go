package layout

// AddVirtualVertices creates nodes for edges spanning multiple ranks
//
//     Rank  input    output
//      0      A        A
//            /|       / \
//      1    B |  =>  B   V
//            \|       \ /
//      2      C        C
func AddVirtualVertices(graph *Graph) {
	if len(graph.ByRank) == 0 {
		return
	}

	for _, src := range graph.Nodes {
		for di, dst := range src.Out {
			if dst.Rank-src.Rank <= 1 {
				continue
			}

			src.Out[di] = nil
			dst.In.Remove(src)

			for rank := dst.Rank - 1; rank > src.Rank; rank-- {
				node := graph.AddNode()
				node.Rank = rank
				node.Virtual = true
				graph.ByRank[node.Rank].Append(node)
				graph.AddEdge(node, dst)
				dst = node
			}

			src.Out[di] = dst
			dst.In.Append(src)
		}
	}
}
