package hier

import (
	"cmp"
	"slices"
)

// Crossing counts require up-to-date Node.Pos, see assignPos.
// Each crossing counts the product of the two edge weights.

// crossingsBothWays returns the crossings of u and v with u left of v, and
// with v left of u, visiting each neighbor pair once. Without weights and
// ports it counts from the neighbor positions cached by cachePos.
func (graph *Graph) crossingsBothWays(u, v *Node) (uv, vu float32) {
	if len(graph.weights) == 0 && len(graph.Ports) == 0 {
		return crossingsByPos(u, v)
	}

	// where the edges end on the neighbors, see Ports
	in := func(w, u *Node) float32 { return float32(w.Pos) + graph.Ports[[2]ID{w.ID, u.ID}][0] }
	out := func(u, w *Node) float32 { return float32(w.Pos) + graph.Ports[[2]ID{u.ID, w.ID}][1] }
	for _, w := range u.In {
		for _, z := range v.In {
			if zx, wx := in(z, v), in(w, u); zx < wx {
				uv += graph.Weight(w, u) * graph.Weight(z, v)
			} else if zx > wx {
				vu += graph.Weight(w, u) * graph.Weight(z, v)
			}
		}
	}

	for _, w := range u.Out {
		for _, z := range v.Out {
			if zx, wx := out(v, z), out(u, w); zx < wx {
				uv += graph.Weight(u, w) * graph.Weight(v, z)
			} else if zx > wx {
				vu += graph.Weight(u, w) * graph.Weight(v, z)
			}
		}
	}

	return uv, vu
}

// crossingsByPos counts unweighted crossings of u left of v and v left of u
// from the cached neighbor position arrays
func crossingsByPos(u, v *Node) (uv, vu float32) {
	var a, b int32
	for _, w := range u.inPos {
		for _, z := range v.inPos {
			if z < w {
				a++
			} else if z > w {
				b++
			}
		}
	}

	for _, w := range u.outPos {
		for _, z := range v.outPos {
			if z < w {
				a++
			} else if z > w {
				b++
			}
		}
	}

	return float32(a), float32(b)
}

// cachePos caches the positions of the neighbors of the nodes for
// crossingsByPos, so that its pair loops stream; they hold while the
// neighbor ranks keep their order
func (nodes Nodes) cachePos() {
	for _, node := range nodes {
		node.inPos = node.inPos[:0]
		for _, src := range node.In {
			node.inPos = append(node.inPos, int32(src.Pos))
		}
		node.outPos = node.outPos[:0]
		for _, dst := range node.Out {
			node.outPos = append(node.outPos, int32(dst.Pos))
		}
	}
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

// TotalCrossings counts weighted edge crossings between all adjacent ranks,
// and the nodes flat edges pass over
func (graph *Graph) TotalCrossings() float32 {
	graph.assignPos()
	total := float32(0)
	for r := 1; r < len(graph.ByRank); r++ {
		total += graph.bilayerCrossings(graph.ByRank[r-1], graph.ByRank[r])
	}

	// a flat edge crosses every node it passes over; unweighted, so with
	// heavy edges elsewhere it hardly counts
	for _, edge := range graph.Flat {
		total += float32(max(edge[0].Pos, edge[1].Pos) - min(edge[0].Pos, edge[1].Pos) - 1)
	}
	return total
}

// bilayerCrossings counts weighted crossings between two adjacent layers in
// O(E log V) with an accumulator tree over where the edges reach the
// lower layer: in the order they leave upper, the crossings are the pairs
// that reach lower the other way round. Edges end where Ports puts them
// on their nodes.
func (graph *Graph) bilayerCrossings(upper, lower Nodes) float32 {
	if len(lower) == 0 {
		return 0
	}

	// the leaf of the tree an edge reaches, in the order the edges leave
	type edge struct {
		at     int
		weight float32
	}
	var edges []edge
	leaves := len(lower)
	if len(graph.Ports) == 0 {
		// the leaves are the positions in lower
		for _, u := range upper {
			start := len(edges)
			for _, v := range u.Out {
				edges = append(edges, edge{v.Pos, graph.Weight(u, v)})
			}
			slices.SortFunc(edges[start:], func(a, b edge) int { return a.at - b.at })
		}
	} else {
		// ends of different nodes can meet, so sort them all, and the
		// leaves are the places edges reach, in order
		type end struct{ from, to, weight float32 }
		var ends []end
		for _, u := range upper {
			for _, v := range u.Out {
				p := graph.Ports[[2]ID{u.ID, v.ID}]
				ends = append(ends, end{float32(u.Pos) + p[0], float32(v.Pos) + p[1], graph.Weight(u, v)})
			}
		}

		slices.SortFunc(ends, func(a, b end) int {
			if a.from != b.from {
				return cmp.Compare(a.from, b.from)
			}
			return cmp.Compare(a.to, b.to)
		})
		tos := make([]float32, len(ends))
		for i, e := range ends {
			tos[i] = e.to
		}
		slices.Sort(tos)
		tos = slices.Compact(tos)
		for _, e := range ends {
			at, _ := slices.BinarySearch(tos, e.to)
			edges = append(edges, edge{at, e.weight})
		}
		leaves = len(tos)
	}

	// inner nodes of the tree hold summed weights
	first := 1
	for first < leaves {
		first *= 2
	}
	tree := make([]float32, 2*first)
	total := float32(0)
	for _, e := range edges {
		i := first + e.at
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
