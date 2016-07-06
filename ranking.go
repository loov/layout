package layout

import (
	"math/rand"
	"time"
)

func Rank(graph *Graph) {
	rand.Seed(time.Now().UnixNano())

	Rank_Frontload(graph)

	for i := 0; i < 7; i++ {
		Rank_Improve_MinimizeEdges(graph, i%2 == 0)
	}

	graph.ByRank = nil
	for _, node := range graph.Nodes {
		if node.Rank >= len(graph.ByRank) {
			byRank := make([]NodeIDs, node.Rank+1)
			copy(byRank, graph.ByRank)
			graph.ByRank = byRank
		}
		graph.ByRank[node.Rank].Add(node.ID)
	}
}

// Rank_Frontload assigns node.Rank := max(node.In[i].Rank) + 1
func Rank_Frontload(graph *Graph) {
	roots := NodeIDs{}
	incount := make([]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		incount[node.ID] = len(node.In)
		if len(node.In) == 0 {
			roots.Add(node.ID)
		}
	}

	rank := 0
	for len(roots) > 0 {
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

// Rank_Backload assigns node.Rank := min(node.Out[i].Rank) - 1
func Rank_Backload(graph *Graph) {
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

// Rank_Improve_MinimizeEdges moves nodes up/down to more equally distribute
func Rank_Improve_MinimizeEdges(graph *Graph, down bool) (changed bool) {
	if down {
		// try to move nodes down
		for _, node := range graph.Nodes {
			if len(node.In) <= len(node.Out) {
				// there are more edges below, try to move node downwards
				minrank := len(graph.Nodes)
				for _, did := range node.Out {
					minrank = min(graph.Nodes[did].Rank, minrank)
				}
				if node.Rank <= minrank-1 {
					if len(node.In) == len(node.Out) {
						// node.Rank = node.Rank
						node.Rank = (node.Rank + (minrank - 1) + 1) / 2
						// node.Rank = randbetween(node.Rank, minrank-1)
					} else {
						node.Rank = minrank - 1
					}
					changed = true
				}
			}
		}
	} else {
		for _, node := range graph.Nodes {
			if len(node.In) >= len(node.Out) {
				// there are more edges above, try to move node upwards
				maxrank := 0
				for _, sid := range node.In {
					maxrank = max(graph.Nodes[sid].Rank, maxrank)
				}
				if node.Rank >= maxrank+1 {
					if len(node.In) == len(node.Out) {
						// node.Rank = node.Rank
						node.Rank = (node.Rank + (maxrank + 1)) / 2
						// node.Rank = randbetween(node.Rank, maxrank+1)
					} else {
						node.Rank = maxrank + 1
					}
					changed = true
				}
			}
		}
	}
	return
}
