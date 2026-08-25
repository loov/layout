package hier

// maxSiftLayer bounds the O(n²) pairwise crossing matrix per layer.
// ponytail: larger layers are left to median+transpose
const maxSiftLayer = 400

// OrderRanksSift moves every node to the position within its layer that
// minimizes crossings with both neighbor layers, using a pairwise crossing
// matrix (c[u][v] = crossings with u left of v); it repeats while a layer
// improves.
func OrderRanksSift(graph *Graph) (moves int) {
	graph.assignPos()
	for _, layer := range graph.ByRank {
		n := len(layer)
		if n < 3 || n > maxSiftLayer {
			continue
		}
		c := make([]float32, n*n)
		for i := 0; i < n; i++ {
			for k := i + 1; k < n; k++ {
				c[i*n+k], c[k*n+i] = graph.crossingsBothWays(layer[i], layer[k])
			}
		}
		// order holds indices into layer; the matrix does not depend on
		// positions inside the layer, so it is valid for the whole pass
		order := make([]int, n)
		for i := range order {
			order[i] = i
		}
		others := make([]int, 0, n)
		for range 4 {
			improved := false
			for i := 0; i < n; i++ {
				v := order[i]
				others = append(others[:0], order[:i]...)
				others = append(others, order[i+1:]...)
				// cost of v at the leftmost slot, then sliding right past
				// each of the others; ties keep the current slot
				cost := float32(0)
				for _, u := range others {
					cost += c[v*n+u]
				}
				best, bestCost := 0, cost
				for k, u := range others {
					cost += c[u*n+v] - c[v*n+u]
					if cost < bestCost || (cost == bestCost && k+1 == i) {
						best, bestCost = k+1, cost
					}
				}
				if best != i {
					Nodes(nil).moveInt(order, i, best)
					moves++
					improved = true
				}
			}
			if !improved {
				break
			}
		}
		reordered := make(Nodes, n)
		for i, k := range order {
			reordered[i] = layer[k]
		}
		copy(layer, reordered)
		layer.assignPos()
	}
	return moves
}

// moveInt moves order[from] to index to, shifting the others
func (Nodes) moveInt(order []int, from, to int) {
	v := order[from]
	if from < to {
		copy(order[from:to], order[from+1:to+1])
	} else {
		copy(order[to+1:from+1], order[to:from])
	}
	order[to] = v
}
