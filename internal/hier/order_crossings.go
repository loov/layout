package hier

import "slices"

// Crossing counts require up-to-date Node.Pos, see assignPos.
// Each crossing counts the product of the two edge weights.

// CrossingsUp counts crossings between edges into u and v from the rank above,
// assuming u is placed left of v.
func (graph *Graph) CrossingsUp(u, v *Node) float32 {
	total := float32(0)
	for _, w := range u.In {
		for _, z := range v.In {
			if z.Pos < w.Pos {
				total += graph.Weight(w, u) * graph.Weight(z, v)
			}
		}
	}
	return total
}

// CrossingsDown counts crossings between edges out of u and v to the rank below,
// assuming u is placed left of v.
func (graph *Graph) CrossingsDown(u, v *Node) float32 {
	total := float32(0)
	for _, w := range u.Out {
		for _, z := range v.Out {
			if z.Pos < w.Pos {
				total += graph.Weight(u, w) * graph.Weight(v, z)
			}
		}
	}
	return total
}

// Crossings counts crossings on both sides assuming u is left of v
func (graph *Graph) Crossings(u, v *Node) float32 {
	uv, _ := graph.crossingsBothWays(u, v)
	return uv
}

// crossingsBothWays returns the crossings of u and v with u left of v, and
// with v left of u, visiting each neighbor pair once
func (graph *Graph) crossingsBothWays(u, v *Node) (uv, vu float32) {
	if len(graph.weights) == 0 {
		// unweighted fast path
		count := func(uadj, vadj Nodes) {
			for _, w := range uadj {
				for _, z := range vadj {
					if z.Pos < w.Pos {
						uv++
					} else if z.Pos > w.Pos {
						vu++
					}
				}
			}
		}
		count(u.In, v.In)
		count(u.Out, v.Out)
		return uv, vu
	}
	for _, w := range u.In {
		for _, z := range v.In {
			if z.Pos < w.Pos {
				uv += graph.Weight(w, u) * graph.Weight(z, v)
			} else if z.Pos > w.Pos {
				vu += graph.Weight(w, u) * graph.Weight(z, v)
			}
		}
	}
	for _, w := range u.Out {
		for _, z := range v.Out {
			if z.Pos < w.Pos {
				uv += graph.Weight(u, w) * graph.Weight(v, z)
			} else if z.Pos > w.Pos {
				vu += graph.Weight(u, w) * graph.Weight(v, z)
			}
		}
	}
	return uv, vu
}

// assignPos records each node's index within its rank
func (graph *Graph) assignPos() {
	for _, layer := range graph.ByRank {
		layer.assignPos()
	}
}

// assignPos records each node's index within the layer
func (layer Nodes) assignPos() {
	for i, node := range layer {
		node.Pos = i
	}
}

// TotalCrossings counts weighted edge crossings between all adjacent ranks
func (graph *Graph) TotalCrossings() float32 {
	graph.assignPos()
	total := float32(0)
	for r := 1; r < len(graph.ByRank); r++ {
		total += graph.bilayerCrossings(graph.ByRank[r-1], graph.ByRank[r])
	}
	return total
}

// bilayerCrossings counts weighted crossings between two adjacent layers in
// O(E log V) with an accumulator tree over the lower layer positions.
func (graph *Graph) bilayerCrossings(upper, lower Nodes) float32 {
	if len(lower) == 0 {
		return 0
	}
	// edges sorted by (upper pos, lower pos)
	type edge struct {
		pos    int
		weight float32
	}
	var edges []edge
	for _, u := range upper {
		start := len(edges)
		for _, v := range u.Out {
			edges = append(edges, edge{v.Pos, graph.Weight(u, v)})
		}
		slices.SortFunc(edges[start:], func(a, b edge) int { return a.pos - b.pos })
	}

	// tree leaves are lower positions; inner nodes hold summed weights
	first := 1
	for first < len(lower) {
		first *= 2
	}
	tree := make([]float32, 2*first)
	total := float32(0)
	for _, e := range edges {
		i := first + e.pos
		tree[i] += e.weight
		for i > 1 {
			if i%2 == 0 {
				total += e.weight * tree[i+1] // edges already seen to the right
			}
			i /= 2
			tree[i] += e.weight
		}
	}
	return total
}

// TotalEdgeLength sums the weighted horizontal distance, in positions,
// between the ends of every edge; a secondary ordering objective.
func (graph *Graph) TotalEdgeLength() float32 {
	graph.assignPos()
	total := float32(0)
	for _, src := range graph.Nodes {
		for _, dst := range src.Out {
			d := float32(src.Pos - dst.Pos)
			if d < 0 {
				d = -d
			}
			total += graph.Weight(src, dst) * d
		}
	}
	return total
}

// edgeLength sums the horizontal distance of node's edges given it sits at pos
func (graph *Graph) edgeLength(node *Node, pos int) float32 {
	total := float32(0)
	for _, src := range node.In {
		d := float32(src.Pos - pos)
		if d < 0 {
			d = -d
		}
		total += graph.Weight(src, node) * d
	}
	for _, dst := range node.Out {
		d := float32(dst.Pos - pos)
		if d < 0 {
			d = -d
		}
		total += graph.Weight(node, dst) * d
	}
	return total
}
