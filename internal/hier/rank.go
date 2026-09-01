package hier

import "slices"

// Rank assigns ranks with the network simplex method, evens out rank
// widths and fills in ByRank.
func Rank(graph *Graph) { RankWith(graph, true) }

// RankWith assigns ranks with the network simplex method and fills in
// ByRank; with balance set, nodes that can move freely are spread over
// the least populated ranks.
func RankWith(graph *Graph, balance bool) {
	RankNetworkSimplex(graph)
	flipBackwardEdges(graph)
	RankCompact(graph)
	if balance {
		RankBalance(graph)
		RankCompact(graph)
	}
	extractFlatEdges(graph)

	graph.ByRank = nil
	for _, node := range graph.Nodes {
		if node.Rank >= len(graph.ByRank) {
			byRank := make([]Nodes, node.Rank+1)
			copy(byRank, graph.ByRank)
			graph.ByRank = byRank
		}
		graph.ByRank[node.Rank].Append(node)
	}
}

// flipBackwardEdges reverses edges whose target ended up above their source
// because of rank constraints, keeping every edge pointing down or flat
func flipBackwardEdges(graph *Graph) {
	var backward [][2]*Node
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if dst.Rank < src.Rank {
				backward = append(backward, [2]*Node{src, dst})
			}
		}
	}
	for _, edge := range backward {
		src, dst := edge[0], edge[1]
		weight, minlen := graph.Weight(src, dst), graph.MinLen(src, dst)
		src.Out.Remove(dst)
		dst.In.Remove(src)
		delete(graph.weights, [2]ID{src.ID, dst.ID})
		delete(graph.minlens, [2]ID{src.ID, dst.ID})
		graph.AddWeightedEdge(dst, src, weight)
		graph.SetMinLen(dst, src, minlen)
	}
}

// extractFlatEdges moves edges between nodes on the same rank to graph.Flat
func extractFlatEdges(graph *Graph) {
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if src.Rank == dst.Rank {
				graph.Flat = append(graph.Flat, [2]*Node{src, dst})
			}
		}
	}
	for _, edge := range graph.Flat {
		src, dst := edge[0], edge[1]
		src.Out.Remove(dst)
		dst.In.Remove(src)
	}
}

// DoubleRanks spreads nodes over every other rank so that every edge spans
// at least two ranks and gets a virtual node in between.
func DoubleRanks(graph *Graph) {
	for _, node := range graph.Nodes {
		node.Rank *= 2
	}
	byRank := make([]Nodes, 2*len(graph.ByRank)-1)
	for i, layer := range graph.ByRank {
		byRank[2*i] = layer
	}
	graph.ByRank = byRank
}

// RankCompact renumbers ranks so that there are no empty ranks. Ranks
// crossed by an edge that must span several ranks are kept, so that
// compacting doesn't undo its minimum length.
func RankCompact(graph *Graph) {
	used := map[int]bool{}
	for _, node := range graph.Nodes {
		used[node.Rank] = true
	}
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			if graph.MinLen(src, dst) <= 1 {
				continue
			}
			for rank := src.Rank + 1; rank < dst.Rank; rank++ {
				used[rank] = true
			}
		}
	}
	ranks := make([]int, 0, len(used))
	for r := range used {
		ranks = append(ranks, r)
	}
	slices.Sort(ranks)
	remap := make(map[int]int, len(ranks))
	for i, r := range ranks {
		remap[r] = i
	}
	for _, node := range graph.Nodes {
		node.Rank = remap[node.Rank]
	}
}

// RankBalance moves nodes with equal in/out degree to the least populated
// rank they can legally occupy, evening out rank widths.
func RankBalance(graph *Graph) {
	width := map[int]int{}
	for _, node := range graph.Nodes {
		width[node.Rank]++
	}

	pinned := graph.pinnedNodes()
	for _, node := range graph.Nodes {
		if graph.InWeight(node) != graph.OutWeight(node) || len(node.In) == 0 || pinned.Contains(node) {
			continue
		}
		lo, hi := 0, len(graph.Nodes)
		for _, src := range node.In {
			lo = max(lo, src.Rank+int(graph.MinLen(src, node)))
		}
		for _, dst := range node.Out {
			hi = min(hi, dst.Rank-int(graph.MinLen(node, dst)))
		}
		best := node.Rank
		for r := lo; r <= hi; r++ {
			if width[r] < width[best] {
				best = r
			}
		}
		width[node.Rank]--
		width[best]++
		node.Rank = best
	}
}

// pinnedNodes returns the nodes whose rank is fixed by a SameRank group
func (graph *Graph) pinnedNodes() NodeSet {
	pinned := NewNodeSet(graph.NodeCount())
	for _, group := range graph.SameRank {
		for _, node := range group {
			pinned.Add(node)
		}
	}
	return pinned
}
