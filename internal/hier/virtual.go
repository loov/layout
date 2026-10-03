package hier

import "slices"

// AddVirtuals creates nodes for edges spanning multiple ranks
//
//	Rank  input    output
//	 0      A        A
//	       /|       / \
//	 1    B |  =>  B   V
//	       \|       \ /
//	 2      C        C
func AddVirtuals(graph *Graph) {
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
			weight := graph.Weight(src, dst)
			// parallel edges share the weight, keep it for the next one
			if !slices.Contains(src.Out[di+1:], dst) {
				delete(graph.weights, [2]ID{src.ID, dst.ID})
				delete(graph.minlens, [2]ID{src.ID, dst.ID})
			}

			for rank := dst.Rank - 1; rank > src.Rank; rank-- {
				node := graph.AddNode()
				node.Rank = rank
				node.Virtual = true
				graph.ByRank[node.Rank].Append(node)
				graph.AddWeightedEdge(node, dst, weight)
				dst = node
			}

			src.Out[di] = dst
			dst.In.Append(src)
			graph.SetWeight(src, dst, weight)
		}
	}
}
