package hier

import "slices"

// DefaultRank does recommended ranking algorithm
func DefaultRank(graph *Graph) *Graph {
	Rank(graph)
	return graph
}

// Rank implements basic ranking algorithm
func Rank(graph *Graph) {
	RankFrontload(graph)
	RankSameRank(graph)

	// greedy tightening; network simplex would give optimal edge spans
	for i := 0; i < 100 && RankMinimizeEdgeStep(graph, i%2 == 0); i++ {
	}
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

// RankFrontload assigns node.Rank := max(node.In[i].Rank) + 1
func RankFrontload(graph *Graph) {
	roots := graph.Roots()

	incount := make([]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		incount[node.ID] = len(node.In)
	}

	rank := 0
	for len(roots) > 0 {
		next := Nodes{}
		for _, src := range roots {
			src.Rank = rank
			for _, dst := range src.Out {
				incount[dst.ID]--
				if incount[dst.ID] == 0 {
					next.Append(dst)
				}
			}
		}
		roots = next
		rank++
	}
}

// RankMinimizeEdgeStep moves nodes up/down to more equally distribute
func RankMinimizeEdgeStep(graph *Graph, down bool) (changed bool) {
	pinned := graph.pinnedNodes()
	if down {
		// try to move nodes down
		for _, node := range graph.Nodes {
			if len(node.Out) == 0 || pinned.Contains(node) {
				continue
			}
			if graph.InWeight(node) <= graph.OutWeight(node) {
				// there are more edges below, try to move node downwards
				minrank := len(graph.Nodes)
				for _, dst := range node.Out {
					minrank = min(dst.Rank, minrank)
				}
				if graph.InWeight(node) < graph.OutWeight(node) && node.Rank < minrank-1 {
					node.Rank = minrank - 1
					changed = true
				}
			}
		}
	} else {
		for _, node := range graph.Nodes {
			if len(node.In) == 0 || pinned.Contains(node) {
				continue
			}
			if graph.InWeight(node) >= graph.OutWeight(node) {
				// there are more edges above, try to move node upwards
				maxrank := 0
				for _, src := range node.In {
					maxrank = max(src.Rank, maxrank)
				}
				if graph.InWeight(node) > graph.OutWeight(node) && node.Rank > maxrank+1 {
					node.Rank = maxrank + 1
					changed = true
				}
			}
		}
	}
	return
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

// RankSameRank raises each SameRank group to its highest member rank and
// re-propagates the rank constraints along edges until stable.
func RankSameRank(graph *Graph) {
	if len(graph.SameRank) == 0 {
		return
	}
	for changed := true; changed; {
		changed = false
		for _, group := range graph.SameRank {
			top := 0
			for _, node := range group {
				top = max(top, node.Rank)
			}
			for _, node := range group {
				if node.Rank != top {
					node.Rank = top
					changed = true
				}
			}
		}
		for _, src := range graph.Nodes {
			for _, dst := range src.Out {
				if dst.Rank <= src.Rank {
					dst.Rank = src.Rank + 1
					changed = true
				}
			}
		}
	}
}
