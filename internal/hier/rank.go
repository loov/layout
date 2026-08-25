package hier

import "slices"

// DefaultRank does recommended ranking algorithm
func DefaultRank(graph *Graph) *Graph {
	Rank(graph)
	return graph
}

// Rank assigns ranks with the network simplex method, evens out rank
// widths and fills in ByRank.
func Rank(graph *Graph) {
	RankNetworkSimplex(graph)
	RankCompact(graph)
	RankBalance(graph)
	RankCompact(graph)

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

// RankCompact renumbers ranks so that there are no empty ranks
func RankCompact(graph *Graph) {
	used := map[int]bool{}
	for _, node := range graph.Nodes {
		used[node.Rank] = true
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
			lo = max(lo, src.Rank+1)
		}
		for _, dst := range node.Out {
			hi = min(hi, dst.Rank-1)
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
