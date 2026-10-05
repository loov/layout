package hier

// OrderRanksChains shifts the virtual chain of every long edge as a whole
// one position left or right while that reduces crossings, or shortens
// edges without adding crossings. Transpose and sift move one node at a
// time, and a chain beside a node that has an edge crossing it often
// costs the same on either side of the node in any single rank.
func OrderRanksChains(graph *Graph) (moves int) {
	graph.assignPos()
	var chains []Nodes
	for _, src := range graph.Nodes {
		if src.Virtual {
			continue
		}
		for _, next := range src.Out {
			var chain Nodes
			for next.Virtual && !next.BorderLeft && !next.BorderRight {
				chain.Append(next)
				next = next.Out[0]
			}
			if len(chain) > 1 {
				chains = append(chains, chain)
			}
		}
	}

	// crossings between the ranks around the chain
	crossings := func(chain Nodes) float32 {
		lo, hi := max(chain[0].Rank-1, 0), min(chain[len(chain)-1].Rank+1, len(graph.ByRank)-1)
		total := float32(0)
		for r := lo + 1; r <= hi; r++ {
			total += graph.bilayerCrossings(graph.ByRank[r-1], graph.ByRank[r])
		}
		return total
	}
	// swap exchanges every chain node with its neighbor dir away
	swap := func(chain Nodes, dir int) {
		for _, v := range chain {
			layer := graph.ByRank[v.Rank]
			u := layer[v.Pos+dir]
			layer[v.Pos], layer[u.Pos] = u, v
			v.Pos, u.Pos = u.Pos, v.Pos
		}
	}
	// movable reports whether every chain node has a neighbor dir away
	// in its cluster to swap with
	movable := func(chain Nodes, dir int) bool {
		for _, v := range chain {
			layer, k := graph.ByRank[v.Rank], v.Pos+dir
			if k < 0 || k >= len(layer) {
				return false
			}
			u := layer[k]
			if u.BorderLeft || u.BorderRight || u.Cluster != v.Cluster {
				return false
			}
		}
		return true
	}

	for range 8 {
		improved := false
		for _, chain := range chains {
			for _, dir := range [2]int{-1, 1} {
				for movable(chain, dir) {
					before := crossings(chain)
					swap(chain, dir)
					after := crossings(chain)
					if after < before {
						moves++
						improved = true
						continue
					}
					swap(chain, -dir)
					break
				}
			}
		}
		if !improved {
			break
		}
	}
	return moves
}
