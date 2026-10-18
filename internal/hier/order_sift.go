package hier

// maxSiftLayer bounds the O(n²) pairwise crossing matrix per layer;
// larger layers are left to median+transpose.
const maxSiftLayer = 400

// OrderRanksSift moves every node to the position within its layer that
// minimizes crossings with both neighbor layers, using a pairwise crossing
// matrix (c[u][v] = crossings with u left of v); it repeats while a layer
// improves.
func OrderRanksSift(graph *Graph) (moves int) {
	graph.assignPos()
	var buf struct {
		d, left []float32
		order   []int
	}
	for _, layer := range graph.ByRank {
		n := len(layer)
		if n < 3 || n > maxSiftLayer {
			continue
		}
		layer.cachePos()
		// d[v*n+u] is how many fewer crossings v has right of u than left
		// of it, by the crossings c either way: rows, which the scan
		// below reads, stay in the cache where columns would not
		d := resize(buf.d, n*n)
		left := resize(buf.left, n) // the crossings of v left of all others
		buf.d, buf.left = d, left
		clear(left)
		for i := range n {
			d[i*n+i] = 0
			for k := i + 1; k < n; k++ {
				ik, ki := graph.crossingsBothWays(layer[i], layer[k])
				d[i*n+k], d[k*n+i] = ki-ik, ik-ki
				left[i] += ik
				left[k] += ki
			}
		}

		// order holds indices into layer; the costs do not depend on
		// positions inside the layer, so they hold for the whole pass
		order := resize(buf.order, n)
		buf.order = order
		for i := range order {
			order[i] = i
		}
		for range 4 {
			improved := false
			for i := range n {
				v := order[i]
				row := d[v*n : v*n+n]
				// cost of v at the leftmost slot, then sliding right past
				// each of the others; ties keep the current slot
				cost := left[v]
				best, bestCost := 0, cost
				slot := 0
				for j, u := range order {
					if j == i {
						continue
					}
					slot++
					cost += row[u]
					if cost < bestCost || (cost == bestCost && slot == i) {
						best, bestCost = slot, cost
					}
				}
				if best != i {
					move(order, i, best)
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

// move moves s[from] to index to, shifting the elements between them over
// by one
func move[T any](s []T, from, to int) {
	v := s[from]
	if from < to {
		copy(s[from:to], s[from+1:to+1])
	} else {
		copy(s[to+1:from+1], s[to:from])
	}
	s[to] = v
}
